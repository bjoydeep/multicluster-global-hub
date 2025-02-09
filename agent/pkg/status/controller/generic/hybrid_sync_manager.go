package generic

import (
	"errors"
	"sync"

	"github.com/go-logr/logr"

	statusbundle "github.com/stolostron/multicluster-global-hub/agent/pkg/status/bundle"
	"github.com/stolostron/multicluster-global-hub/pkg/bundle"
	"github.com/stolostron/multicluster-global-hub/pkg/transport/producer"
)

var errExpectingDeltaStateBundle = errors.New("expecting a BundleCollectionEntry that wraps a DeltaStateBundle bundle")

// hybridSyncManager manages two BundleCollectionEntry instances in application of hybrid-sync mode.
// won't get collected by the GC since callbacks are used.
type HybridSyncManager struct {
	log                        logr.Logger
	activeSyncMode             bundle.BundleSyncMode
	bundleCollectionEntryMap   map[bundle.BundleSyncMode]*BundleCollectionEntry
	deltaStateBundle           statusbundle.DeltaStateBundle
	sentDeltaCountSwitchFactor int
	sentDeltaCount             int
	lock                       sync.Mutex
}

// NewHybridSyncManager creates a manager that manages two BundleCollectionEntry instances that wrap a
// complete-state bundle and a delta-state bundle.
func NewHybridSyncManager(log logr.Logger, completeStateBundleCollectionEntry *BundleCollectionEntry,
	deltaStateBundleCollectionEntry *BundleCollectionEntry,
) (*HybridSyncManager, error) {
	// check that the delta state collection does indeed wrap a delta bundle
	deltaStateBundle, ok := deltaStateBundleCollectionEntry.bundle.(statusbundle.DeltaStateBundle)
	if !ok {
		return nil, errExpectingDeltaStateBundle
	}

	hybridSyncManager := &HybridSyncManager{
		log:            log,
		activeSyncMode: bundle.CompleteStateMode,
		bundleCollectionEntryMap: map[bundle.BundleSyncMode]*BundleCollectionEntry{
			bundle.CompleteStateMode: completeStateBundleCollectionEntry,
			bundle.DeltaStateMode:    deltaStateBundleCollectionEntry,
		},
		deltaStateBundle: deltaStateBundle,
		sentDeltaCount:   0,
		lock:             sync.Mutex{},
	}

	hybridSyncManager.appendPredicates()

	return hybridSyncManager, nil
}

func (manager *HybridSyncManager) GetBundleCollectionEntry(syncMode bundle.BundleSyncMode) *BundleCollectionEntry {
	return manager.bundleCollectionEntryMap[syncMode]
}

func (manager *HybridSyncManager) SetHybridModeCallBack(deltaCountSwitchFactor int, transportObj producer.Producer) {
	manager.sentDeltaCountSwitchFactor = deltaCountSwitchFactor
	// hybrid mode may be disabled in some different scenarios.
	if manager.sentDeltaCountSwitchFactor <= 0 || !transportObj.SupportsDeltaBundles() {
		return
	}
	for _, bundleCollectionEntry := range manager.bundleCollectionEntryMap {
		transportObj.Subscribe(bundleCollectionEntry.transportBundleKey,
			map[producer.EventType]producer.EventCallback{
				producer.DeliveryAttempt: manager.handleTransportationAttempt,
				producer.DeliverySuccess: manager.handleTransportationSuccess,
				producer.DeliveryFailure: manager.handleTransportationFailure,
			})
	}
}

func (manager *HybridSyncManager) appendPredicates() {
	// append predicates for mode-management
	for syncMode, bundleCollectionEntry := range manager.bundleCollectionEntryMap {
		entry := bundleCollectionEntry       // to use in func
		mode := syncMode                     // to use in func
		originalPredicate := entry.predicate // avoid recursion
		entry.predicate = func() bool {
			manager.lock.Lock()
			defer manager.lock.Unlock()

			return manager.activeSyncMode == mode && originalPredicate()
		}
	}
}

func (manager *HybridSyncManager) handleTransportationAttempt() {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	if manager.activeSyncMode == bundle.CompleteStateMode {
		manager.switchToDeltaStateMode()
		return
	}

	// else we're in delta
	manager.sentDeltaCount++

	if manager.sentDeltaCount == manager.sentDeltaCountSwitchFactor {
		manager.switchToCompleteStateMode()
		return
	}

	// reset delta bundle objects
	manager.deltaStateBundle.Reset()
}

func (manager *HybridSyncManager) handleTransportationSuccess() {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	if manager.activeSyncMode == bundle.DeltaStateMode {
		return
	}

	manager.switchToDeltaStateMode()
}

func (manager *HybridSyncManager) handleTransportationFailure() {
	manager.lock.Lock()
	defer manager.lock.Unlock()

	if manager.activeSyncMode == bundle.CompleteStateMode {
		return
	}

	manager.log.Info("transportation failure callback invoked")
	manager.switchToCompleteStateMode()
}

func (manager *HybridSyncManager) switchToCompleteStateMode() {
	manager.log.Info("switched to complete-state mode")
	manager.activeSyncMode = bundle.CompleteStateMode
}

func (manager *HybridSyncManager) switchToDeltaStateMode() {
	manager.log.Info("switched to delta-state mode")

	manager.activeSyncMode = bundle.DeltaStateMode
	manager.sentDeltaCount = 0

	manager.deltaStateBundle.Reset()
	manager.deltaStateBundle.SyncState()
}
