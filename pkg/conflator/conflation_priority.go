package conflator

// ConflationPriority sets processing priorities of bundles.
type ConflationPriority uint8

// priority list of conflation unit.
const (
	ManagedClustersPriority               ConflationPriority = iota
	ClustersPerPolicyPriority             ConflationPriority = iota
	CompleteComplianceStatusPriority      ConflationPriority = iota
	DeltaComplianceStatusPriority         ConflationPriority = iota
	MinimalComplianceStatusPriority       ConflationPriority = iota
	HubClusterInfoStatusPriority          ConflationPriority = iota
	ControlInfoPriority                   ConflationPriority = iota
	LocalPolicySpecPriority               ConflationPriority = iota
	LocalClustersPerPolicyPriority        ConflationPriority = iota
	LocalCompleteComplianceStatusPriority ConflationPriority = iota
	LocalPolicyStatusEventPriority        ConflationPriority = iota

	// enable global resource
	PlacementRulePriority           ConflationPriority = iota
	PlacementPriority               ConflationPriority = iota
	PlacementDecisionPriority       ConflationPriority = iota
	SubscriptionStatusPriority      ConflationPriority = iota
	SubscriptionReportPriority      ConflationPriority = iota
	LocalPlacementRulesSpecPriority ConflationPriority = iota
)
