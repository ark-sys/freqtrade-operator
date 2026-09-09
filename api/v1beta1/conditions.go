package v1beta1

// Condition types set on Backtest.status.conditions. Deliberately a fresh,
// self-contained vocabulary rather than importing api/v1alpha1's - v1beta1
// is meant to outlive v1alpha1 (P6-5: v1alpha1 served but deprecated once
// v1beta1 lands), so it shouldn't depend on the package that's headed for
// removal, even where a string value happens to read the same.
const (
	// ConditionReady is the top-level summary: true only when every other
	// condition is satisfied.
	ConditionReady = "Ready"
	// ConditionConfigResolved reports whether spec.configRef and
	// spec.strategyRef both resolved to an existing, valid object.
	ConditionConfigResolved = "ConfigResolved"
	// ConditionWorkloadReady reports whether the Job this Backtest owns
	// exists and reflects Pending/Running/Succeeded/Failed correctly into
	// status.phase.
	ConditionWorkloadReady = "WorkloadReady"
	// ConditionResultsAvailable reports whether status.results was
	// successfully populated from the run's own sidecar-written ConfigMap
	// (P6-2). Not set by anything in P6-1 - the Job runs and status.phase
	// reflects it, but nothing extracts results yet.
	ConditionResultsAvailable = "ResultsAvailable"
)

// Condition reasons. Shared across every condition type that can produce
// them, so the same failure always reads the same way.
const (
	// ReasonReferenceNotFound: spec.configRef or spec.strategyRef does not
	// resolve to an existing object.
	ReasonReferenceNotFound = "ReferenceNotFound"
	// ReasonConfigInvalid: a reference resolved, but rendering its content
	// (via configbuilder) failed.
	ReasonConfigInvalid = "ConfigInvalid"
	// ReasonReconcileError: a generic, otherwise-unclassified error occurred
	// while reconciling (a List/Get/Create/Patch call failed).
	ReasonReconcileError = "ReconcileError"
	// ReasonWorkloadProgressing: the Job exists but hasn't finished yet.
	ReasonWorkloadProgressing = "WorkloadProgressing"
	// ReasonWorkloadFailed: the Job ran and failed.
	ReasonWorkloadFailed = "WorkloadFailed"
	// ReasonWorkloadSucceeded: the Job ran to completion successfully.
	ReasonWorkloadSucceeded = "WorkloadSucceeded"
	// ReasonResultsUnavailable is the ResultsAvailable=False reason (P6-2):
	// the run succeeded, but its result file was missing or malformed, so
	// only the summary is missing - the raw file, if any, is still on the
	// results PVC, and this never fails the Backtest itself.
	ReasonResultsUnavailable = "ResultsUnavailable"

	// ReasonAsExpected is the positive-case reason for a condition type when
	// nothing more specific applies. Kubernetes Conditions require a
	// non-empty Reason even when Status is True.
	ReasonAsExpected = "AsExpected"
)

// SetObservedGeneration implements shared.ConditionedObject, letting
// shared.PatchStatus set it generically regardless of which CRD it's
// holding - the same interface v1alpha1's four types implement, satisfied
// independently here since v1beta1 doesn't import v1alpha1 for its own
// types (see this file's own top comment; TradeBot is the one declared
// exception - see tradebot_types.go's own doc comment on TBAppConfig).
func (b *Backtest) SetObservedGeneration(generation int64) { b.Status.ObservedGeneration = generation }

func (t *TradeBot) SetObservedGeneration(generation int64) { t.Status.ObservedGeneration = generation }
