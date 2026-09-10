package v1alpha1

// Condition types set on every CRD's status.conditions. A given controller
// only ever sets the subset relevant to its own CRD (e.g. TradeBotConfig has
// no workload, so it never sets WorkloadReady).
const (
	// ConditionReady is the top-level summary: true only when every other
	// condition the object sets is satisfied. This is what other objects
	// referencing this one (and humans) should check first.
	ConditionReady = "Ready"
	// ConditionConfigResolved reports whether every reference the object
	// depends on - a Config/Strategy name, a Secret a spec field points at -
	// was found and was valid.
	ConditionConfigResolved = "ConfigResolved"
	// ConditionWorkloadReady reports whether the workload the object owns
	// (a StatefulSet, Job, or Deployment) exists and is healthy.
	ConditionWorkloadReady = "WorkloadReady"
	// ConditionWorkloadImmutable is true when a spec change could not be
	// applied to an already-created immutable workload (P0-4:
	// batchv1.Job.spec.template) and was silently not applied as a result.
	ConditionWorkloadImmutable = "WorkloadImmutable"

	// ConditionTradeBotRefsResolved reports whether every name in
	// FreqUI.Spec.TradeBotRefs resolved to an actual TradeBot (P2-5): a typo
	// there used to silently yield no CORS entry for that bot, with nothing
	// visible on the FreqUI object to say why.
	ConditionTradeBotRefsResolved = "TradeBotRefsResolved"

	// ConditionConfigDrift is reserved for P2-4: a rendered config that
	// hasn't been rolled out to the running pod under UpdateStrategy: Manual.
	// No controller sets it yet.
	ConditionConfigDrift = "ConfigDrift"
	// ConditionBotReachable reports whether the operator's poller (P4-3)
	// could reach this bot's freqtrade REST API on its most recent poll
	// attempt. Independent of WorkloadReady: a StatefulSet can be perfectly
	// healthy while its api_server is disabled, misconfigured, or still
	// starting up - this condition is specifically about whether
	// status.bot can currently be trusted.
	ConditionBotReachable = "BotReachable"
	// ConditionStateReconciled reports whether spec.state (v1beta1-only,
	// P4-4) matches the bot's last-observed actual state
	// (status.bot.state, from the same poller). Continuously reconciled,
	// not a one-shot action: a bot that crashes and restarts comes back
	// Running and must be re-stopped without anyone asking again.
	ConditionStateReconciled = "StateReconciled"
)

// Condition reasons. Shared across every condition type and controller that
// can produce them, so the same failure always reads the same way regardless
// of which CRD reports it.
const (
	// ReasonReferenceNotFound: a referenced object (Strategy,
	// TradeBotConfig, TradeBot) does not exist.
	ReasonReferenceNotFound = "ReferenceNotFound"
	// ReasonConfigInvalid: a reference resolved, but rendering or validating
	// its content failed (e.g. TradeBotConfig with no Exchange section).
	ReasonConfigInvalid = "ConfigInvalid"
	// ReasonSecretMissing: a Secret a spec field references was not found.
	ReasonSecretMissing = "SecretMissing"
	// ReasonWorkloadProgressing: the owned workload exists but isn't ready
	// yet (e.g. StatefulSet with ReadyReplicas < 1).
	ReasonWorkloadProgressing = "WorkloadProgressing"
	// ReasonWorkloadFailed: the owned workload exists and has failed (e.g. a
	// Job with Status.Failed > 0).
	ReasonWorkloadFailed = "WorkloadFailed"
	// ReasonWorkloadSucceeded: a one-shot workload (a Job) ran to
	// completion successfully. Distinct from ReasonWorkloadHealthy (the
	// positive case for a long-running workload like a StatefulSet) because
	// Phase is derived from this reason, and "done" and "actively serving"
	// are different phases even though both are WorkloadReady=True.
	ReasonWorkloadSucceeded = "WorkloadSucceeded"
	// ReasonWorkloadImmutable: a general-purpose reason available for any
	// condition (e.g. Ready) when a WorkloadImmutable=True situation is the
	// root cause. The WorkloadImmutable condition's own reason is the more
	// specific ReasonSpecChangeIgnored/ReasonSpecMatchesWorkload pair below.
	ReasonWorkloadImmutable = "WorkloadImmutable"
	// ReasonSpecChangeIgnored is the WorkloadImmutable=True reason: a spec
	// change could not be applied to the already-created immutable workload
	// and was silently not applied as a result (P0-4).
	ReasonSpecChangeIgnored = "SpecChangeIgnored"
	// ReasonReconcileError: a generic, otherwise-unclassified error occurred
	// while reconciling (a List/Get/Create/Update call failed).
	ReasonReconcileError = "ReconcileError"
	// ReasonJobModeRemoved: spec.freqtrade_command is set to a one-shot
	// command (P6-4) - TradeBot is trade-only now that v1beta1 has no
	// field for it at all; Backtest (D1, P6-1) is the replacement. Normal
	// create/update traffic can never actually produce this - the
	// conversion webhook already rejects it, since v1beta1 is the storage
	// version - this only ever fires for a TradeBot stored as v1alpha1
	// bytes from before that migration.
	ReasonJobModeRemoved = "JobModeRemoved"
	// ReasonPendingRestart is the ConfigDrift=True reason (P2-4): the
	// rendered config changed but spec.updateStrategy is Manual, so the
	// StatefulSet pod template was deliberately left untouched. The
	// condition message carries the exact kubectl command to apply it.
	ReasonPendingRestart = "PendingRestart"
	// ReasonUnresolvableTradeBotRefs is the TradeBotRefsResolved=False
	// reason: one or more names in FreqUI.Spec.TradeBotRefs don't match any
	// TradeBot in the namespace (P2-5). Non-blocking - FreqUI still deploys
	// and serves the UI; the affected bot(s) just get no CORS/API route.
	ReasonUnresolvableTradeBotRefs = "UnresolvableTradeBotRefs"

	// ReasonConnectionRefused is a BotReachable=False reason (P4-3): the TCP
	// connection itself failed (nothing listening, NetworkPolicy denying
	// it, DNS failure, ...).
	ReasonConnectionRefused = "ConnectionRefused"
	// ReasonAuthFailed is a BotReachable=False reason: the connection
	// succeeded but freqtrade rejected the Basic Auth credentials (401) -
	// almost always apiServer.secretRef drifting from what's actually in
	// the running bot's rendered config.json.
	ReasonAuthFailed = "AuthFailed"
	// ReasonTimeout is a BotReachable=False reason: no response within the
	// poller's http.Client timeout.
	ReasonTimeout = "Timeout"
	// ReasonIntrospectionDisabled is the BotReachable reason when
	// spec.introspection.enabled is false - not a failure, just "the
	// operator was told not to poll this bot," so status.bot is never
	// populated and this stays neither True nor False (a distinct reason,
	// not one of Reachable's own success/failure pair).
	ReasonIntrospectionDisabled = "IntrospectionDisabled"

	// ReasonBotUnreachable is the StateReconciled=False reason (P4-4): the
	// bot is unreachable (BotReachable=False), so desired state cannot be
	// verified or enforced right now. Deliberately does not retry the
	// start/stop call itself here - it relies entirely on the poller's own
	// backoff to eventually flip BotReachable, rather than probing the bot
	// a second way on top of it.
	ReasonBotUnreachable = "BotUnreachable"
	// ReasonStateChangePending is the StateReconciled=False reason: a
	// start/stop call was just issued to align the bot with spec.state,
	// but hasn't been confirmed yet - that only happens once the poller's
	// next poll refreshes status.bot.state.
	ReasonStateChangePending = "StateChangePending"
	// ReasonStateChangeFailed is the StateReconciled=False reason: the bot
	// was reachable per the last poll, but the start/stop call itself
	// failed (e.g. it went unreachable in the moments since).
	ReasonStateChangeFailed = "StateChangeFailed"

	// ReasonAsExpected is the positive-case reason for a condition type when
	// nothing more specific applies - e.g. ConfigResolved=True. Kubernetes
	// Conditions require a non-empty Reason even when Status is True.
	ReasonAsExpected = "AsExpected"
	// ReasonWorkloadHealthy is the positive-case reason for WorkloadReady.
	ReasonWorkloadHealthy = "WorkloadHealthy"
	// ReasonSpecMatchesWorkload is the positive-case reason for
	// WorkloadImmutable=False: the running workload's template already
	// matches the current spec, so nothing was silently dropped.
	ReasonSpecMatchesWorkload = "SpecMatchesWorkload"
)

// SetObservedGeneration implements shared.ConditionedObject, letting
// shared.PatchStatus set it generically regardless of which of the four
// CRDs it's holding.
func (t *TradeBot) SetObservedGeneration(generation int64) { t.Status.ObservedGeneration = generation }

func (f *FreqUI) SetObservedGeneration(generation int64) { f.Status.ObservedGeneration = generation }

func (s *Strategy) SetObservedGeneration(generation int64) { s.Status.ObservedGeneration = generation }

func (c *TradeBotConfig) SetObservedGeneration(generation int64) {
	c.Status.ObservedGeneration = generation
}
