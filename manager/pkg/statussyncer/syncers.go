package statussyncer

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/stolostron/multicluster-global-hub/manager/pkg/config"
	statusbundle "github.com/stolostron/multicluster-global-hub/manager/pkg/statussyncer/bundle"
	"github.com/stolostron/multicluster-global-hub/manager/pkg/statussyncer/dispatcher"
	dbsyncer "github.com/stolostron/multicluster-global-hub/manager/pkg/statussyncer/syncers"
	"github.com/stolostron/multicluster-global-hub/pkg/bundle/helpers"
	"github.com/stolostron/multicluster-global-hub/pkg/bundle/status"
	"github.com/stolostron/multicluster-global-hub/pkg/conflator"
	"github.com/stolostron/multicluster-global-hub/pkg/conflator/db/workerpool"
	"github.com/stolostron/multicluster-global-hub/pkg/statistics"
	"github.com/stolostron/multicluster-global-hub/pkg/transport"
	"github.com/stolostron/multicluster-global-hub/pkg/transport/consumer"
)

// AddStatusSyncers performs the initial setup required before starting the runtime manager.
// adds controllers and/or runnables to the manager, registers handler functions within the dispatcher
//
//	and create bundle functions within the bundle.
func AddStatusSyncers(mgr ctrl.Manager, managerConfig *config.ManagerConfig) (
	dbsyncer.BundleRegisterable, error,
) {
	// register statistics within the runtime manager
	stats, err := addStatisticController(mgr, managerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to add statistics to manager - %w", err)
	}

	// conflationReadyQueue is shared between conflation manager and dispatcher
	conflationReadyQueue := conflator.NewConflationReadyQueue(stats)
	// manage all Conflation Units
	conflationManager := conflator.NewConflationManager(conflationReadyQueue, stats)

	// database layer initialization - worker pool + connection pool
	dbWorkerPool, err := workerpool.NewDBWorkerPool(managerConfig.DatabaseConfig, stats)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DBWorkerPool: %w", err)
	}
	if err := mgr.Add(dbWorkerPool); err != nil {
		return nil, fmt.Errorf("failed to add DB worker pool: %w", err)
	}

	transportDispatcher, err := getTransportDispatcher(mgr, conflationManager, managerConfig, stats)
	if err != nil {
		return nil, fmt.Errorf("failed to get transport dispatcher: %w", err)
	}

	// add ConflationDispatcher to the runtime manager
	if err := mgr.Add(dispatcher.NewConflationDispatcher(
		ctrl.Log.WithName("conflation-dispatcher"),
		conflationReadyQueue, dbWorkerPool)); err != nil {
		return nil, fmt.Errorf("failed to add conflation dispatcher to runtime manager: %w", err)
	}

	// register db syncers create bundle functions within transport and handler functions within dispatcher
	dbSyncers := []dbsyncer.DBSyncer{
		dbsyncer.NewHubClusterHeartbeatDBSyncer(ctrl.Log.WithName("hub-heartbeat-syncer")),
		dbsyncer.NewHubClusterInfoDBSyncer(ctrl.Log.WithName("hub-info-syncer")),
		dbsyncer.NewManagedClustersDBSyncer(ctrl.Log.WithName("managed-cluster-syncer")),
		dbsyncer.NewCompliancesDBSyncer(ctrl.Log.WithName("compliances-syncer")),
		dbsyncer.NewLocalPolicySpecSyncer(ctrl.Log.WithName("local-policy-spec-syncer")),
		dbsyncer.NewLocalPolicyEventSyncer(ctrl.Log.WithName("local-policy-event-syncer")),
	}

	if managerConfig.EnableGlobalResource {
		dbSyncers = append(dbSyncers,
			dbsyncer.NewPlacementRulesDBSyncer(ctrl.Log.WithName("placement-rules-db-syncer")),
			dbsyncer.NewPlacementsDBSyncer(ctrl.Log.WithName("placements-db-syncer")),
			dbsyncer.NewPlacementDecisionsDBSyncer(
				ctrl.Log.WithName("placement-decisions-db-syncer")),
			dbsyncer.NewSubscriptionStatusesDBSyncer(
				ctrl.Log.WithName("subscription-statuses-db-syncer")),
			dbsyncer.NewSubscriptionReportsDBSyncer(
				ctrl.Log.WithName("subscription-reports-db-syncer")),
			dbsyncer.NewLocalSpecPlacementruleSyncer(
				ctrl.Log.WithName("local-spec-placementrule-syncer")),
		)
	}

	for _, dbsyncerObj := range dbSyncers {
		dbsyncerObj.RegisterCreateBundleFunctions(transportDispatcher)
		dbsyncerObj.RegisterBundleHandlerFunctions(conflationManager)
	}

	return transportDispatcher, nil
}

// the transport dispatcher implement the BundleRegister() method, which can dispatch message to syncers
// both kafkaConsumer and Cloudevents transport dispatcher will forward message to conflation manager
func getTransportDispatcher(mgr ctrl.Manager, conflationManager *conflator.ConflationManager,
	managerConfig *config.ManagerConfig, stats *statistics.Statistics,
) (dbsyncer.BundleRegisterable, error) {
	if managerConfig.TransportConfig.TransportFormat == string(transport.KafkaMessageFormat) {
		kafkaConsumer, err := consumer.NewKafkaConsumer(
			managerConfig.TransportConfig.KafkaConfig,
			ctrl.Log.WithName("message-consumer"))
		if err != nil {
			return nil, fmt.Errorf("failed to create kafka-consumer: %w", err)
		}
		kafkaConsumer.SetConflationManager(conflationManager)
		kafkaConsumer.SetCommitter(consumer.NewCommitter(
			managerConfig.TransportConfig.CommitterInterval,
			managerConfig.TransportConfig.KafkaConfig.ConsumerConfig.ConsumerTopic, kafkaConsumer.Consumer(),
			conflationManager.GetBundlesMetadata, ctrl.Log.WithName("message-consumer")),
		)
		kafkaConsumer.SetStatistics(stats)
		if err := mgr.Add(kafkaConsumer); err != nil {
			return nil, fmt.Errorf("failed to add status transport bridge: %w", err)
		}
		return kafkaConsumer, nil
	} else {
		consumer, err := consumer.NewGenericConsumer(managerConfig.TransportConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize transport consumer: %w", err)
		}
		if err := mgr.Add(consumer); err != nil {
			return nil, fmt.Errorf("failed to add transport consumer to manager: %w", err)
		}
		// consume message from consumer and dispatcher it to conflation manager
		transportDispatcher := dispatcher.NewTransportDispatcher(
			ctrl.Log.WithName("transport-dispatcher"), consumer,
			conflationManager, stats)
		if err := mgr.Add(transportDispatcher); err != nil {
			return nil, fmt.Errorf("failed to add transport dispatcher to runtime manager: %w", err)
		}
		return transportDispatcher, nil
	}
}

// only statistic the local policy and managed clusters
func addStatisticController(mgr ctrl.Manager, managerConfig *config.ManagerConfig) (*statistics.Statistics, error) {
	bundleTypes := []string{
		helpers.GetBundleType(&status.HubClusterInfoBundle{}),
		helpers.GetBundleType(&statusbundle.HubClusterHeartbeatBundle{}),
		helpers.GetBundleType(&statusbundle.ManagedClustersStatusBundle{}),
		helpers.GetBundleType(&statusbundle.LocalPolicySpecBundle{}),
		helpers.GetBundleType(&status.ClusterPolicyEventBundle{}),
		helpers.GetBundleType(&statusbundle.LocalClustersPerPolicyBundle{}),
		helpers.GetBundleType(&statusbundle.LocalCompleteComplianceStatusBundle{}),
	}

	if managerConfig.EnableGlobalResource {
		bundleTypes = append(bundleTypes,
			helpers.GetBundleType(&statusbundle.LocalPlacementRulesBundle{}),
			helpers.GetBundleType(&statusbundle.PlacementsBundle{}),
			helpers.GetBundleType(&statusbundle.PlacementDecisionsBundle{}),
			helpers.GetBundleType(&statusbundle.ClustersPerPolicyBundle{}),
			helpers.GetBundleType(&statusbundle.CompleteComplianceStatusBundle{}),
		)
	}
	// create statistics
	stats := statistics.NewStatistics(managerConfig.StatisticsConfig, bundleTypes)

	if err := mgr.Add(stats); err != nil {
		return nil, fmt.Errorf("failed to add statistics to manager - %w", err)
	}
	return stats, nil
}
