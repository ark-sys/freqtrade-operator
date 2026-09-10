package tradebot

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot/botclient"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebotconfig/configbuilder"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// BotFinalizer is the finalizer name used for TradeBot resources
const BotFinalizer = "freqtrade.io/finalizer"

// Reconciler TradeBotReconciler reconciles a TradeBot object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// FinalizerGracePeriod bounds how long finalizeTradeBot waits for a
	// TradeBot's StatefulSet to scale down before removing the finalizer
	// anyway. Zero means use defaultFinalizerGracePeriod.
	FinalizerGracePeriod time.Duration

	// MaxConcurrentReconciles bounds how many TradeBots this controller
	// reconciles in parallel. Zero means use defaultMaxConcurrentReconciles.
	MaxConcurrentReconciles int

	// Recorder emits the P2-4 ConfigRestart Event under spec.updateStrategy:
	// Auto. Nil is fine - a scoped addition for that one signal, not the
	// full per-controller EventRecorder rollout P4-1 covers; recordConfigRestartEvent
	// skips emitting rather than dereferencing a nil interface.
	Recorder record.EventRecorder

	// DefaultImage is the freqtrade image reference used when
	// spec.app.pod.image doesn't override it. Empty means use
	// defaultFreqtradeImage. Digest-pinned by default (P3-3): a floating
	// tag would let a routine pod restart silently pick up a new freqtrade
	// version mid-trading.
	DefaultImage string

	// OperatorNamespace is the namespace the operator's own pod runs in
	// (read from the POD_NAMESPACE downward-API env var - see cmd/main.go),
	// used as the allow-from-operator peer in each trade-mode TradeBot's
	// NetworkPolicy (P3-4, built ahead of D4/P4-3's bot-polling per the
	// plan's own instruction to land both rules together). Empty disables
	// that peer entirely rather than guessing, since a wrong namespace
	// would silently lock the operator itself out later.
	OperatorNamespace string
}

// collectCORSHostsForTradeBot returns a deduplicated, normalized list of CORS hosts.
// collectCORSHostsForTradeBot only ever allows origins this operator can
// itself account for: each referencing FreqUI's own configured (or derived
// default) origin. It deliberately does NOT add a speculative
// "<botname>.<frequi-host>" subdomain entry (P3-4) - nothing in this
// operator provisions per-bot subdomains (FreqUI is a single shared
// dashboard across all of TradeBotRefs, not one deployment per bot), so
// that entry never corresponded to anything actually served and only
// widened a trading API's CORS allowlist for no reason. Anyone who does
// have real per-bot origin routing can still list it explicitly via
// TradeBotConfig.Spec.APIServer.CORSOrigins, which is additive with this.
func collectCORSHostsForTradeBot(
	tradeBot *freqtradev1alpha1.TradeBot, frequiList *freqtradev1beta1.FreqUIList,
) []string {
	var corsHosts []string
	corsSet := make(map[string]struct{})

	for _, frequi := range frequiList.Items {
		for _, ref := range frequi.Spec.TradeBotRefs {
			if ref.Name == tradeBot.Name {
				if frequi.Spec.Host != "" {
					host := frequi.Spec.Host
					scheme := "https"
					hostname := host

					if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
						u, err := url.Parse(host)
						if err == nil && u.Host != "" {
							scheme = u.Scheme
							hostname = u.Host
						}
					} else if strings.HasPrefix(host, "localhost") {
						scheme = "http"
					}

					baseURL := fmt.Sprintf("%s://%s", scheme, hostname)
					if _, exists := corsSet[baseURL]; !exists {
						corsHosts = append(corsHosts, baseURL)
						corsSet[baseURL] = struct{}{}
					}
				} else {
					candidates := []string{
						fmt.Sprintf("http://%s.%s.svc.cluster.local", frequi.Name, frequi.Namespace),
						"http://localhost:8080",
					}
					for _, h := range candidates {
						if _, exists := corsSet[h]; !exists {
							corsHosts = append(corsHosts, h)
							corsSet[h] = struct{}{}
						}
					}
				}
				break
			}
		}
	}

	sort.Strings(corsHosts)
	return corsHosts
}

// referencingFreqUINames returns the (deduplicated, sorted) names of every
// FreqUI in frequiList that references tradeBot, for
// resources.BuildNetworkPolicy's allow-from-FreqUI peers (P3-4) - each
// FreqUI's Deployment pod carries app: <frequi.Name> (see
// controllers/frequi/resources/deployment.go), so the name is exactly what
// a NetworkPolicy peer needs to select it.
func referencingFreqUINames(tradeBot *freqtradev1alpha1.TradeBot, frequiList *freqtradev1beta1.FreqUIList) []string {
	var names []string
	for _, frequi := range frequiList.Items {
		for _, ref := range frequi.Spec.TradeBotRefs {
			if ref.Name == tradeBot.Name {
				names = append(names, frequi.Name)
				break
			}
		}
	}
	sort.Strings(names)
	return names
}

func validateCORSHosts(hosts []string) error {
	for _, h := range hosts {
		if !strings.HasPrefix(h, "http://") && !strings.HasPrefix(h, "https://") {
			return fmt.Errorf("CORS host %q must start with http:// or https://", h)
		}
	}
	return nil
}

// Helper function to log configData keys
func keys(m map[string]string) []string {
	k := make([]string, 0, len(m))
	for key := range m {
		k = append(k, key)
	}
	return k
}

// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebots,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebots/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebots/finalizers,verbs=update
// +kubebuilder:rbac:groups=freqtrade.io,resources=strategies,verbs=get;list;watch
// +kubebuilder:rbac:groups=freqtrade.io,resources=tradebotconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=freqtrade.io,resources=frequis,verbs=list;watch
// patch (not update) is what server-side apply issues (shared.Apply,
// P2-1) - it's a distinct RBAC verb, so replacing the old Update-based
// ApplyX functions meant replacing this too. update survives only where
// finalizers.go still calls the plain client Update directly (scaling the
// StatefulSet to zero, stripping owner refs off a preserved PVC).
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile handles the reconciliation loop for TradeBot resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. Fetch TradeBot
	var tradeBot freqtradev1alpha1.TradeBot
	if err := r.Get(ctx, req.NamespacedName, &tradeBot); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("TradeBot resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.V(1).Error(err, "Failed to get TradeBot resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info(
		"Reconciling TradeBot", "name", tradeBot.Name, "generation", tradeBot.Generation, "phase", tradeBot.Status.Phase,
	)

	// 2. Handle deletion if needed
	if tradeBot.GetDeletionTimestamp() != nil {
		logger.V(1).Info("TradeBot is being deleted", "name", tradeBot.Name)
		if controllerutil.ContainsFinalizer(&tradeBot, BotFinalizer) {
			logger.V(1).Info("Running finalization logic for TradeBot", "name", tradeBot.Name)
			done, err := r.finalizeTradeBot(ctx, &tradeBot)
			if err != nil {
				logger.V(1).Error(err, "Failed to finalize TradeBot")
				logger.V(2).Info("Requeue requested", "reason", "finalization error", "error", err)
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}
			if !done {
				// Cleanup is still in progress (StatefulSet scale-down hasn't
				// landed yet, or we're inside the grace period). Requeue
				// instead of blocking this worker - MaxConcurrentReconciles
				// is 1, so a blocking wait here would stall every other
				// TradeBot's reconciliation too.
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			// Fetched and updated as v1beta1, not tradeBot's own v1alpha1
			// copy (P4-4): v1alpha1 has no Spec.State field at all, so a
			// v1alpha1 Update round-trips the whole object through a
			// conversion that silently resets any v1beta1-only field back
			// to its CRD default - verified directly, this reset a Stopped
			// bot back to Running. Every write this reconciler makes to a
			// TradeBot's own spec/metadata goes through v1beta1 for
			// exactly this reason; reads of fields v1alpha1 still carries
			// (Config, Strategy, App, ...) keep using tradeBot above.
			var latestTradeBotBeta freqtradev1beta1.TradeBot
			latestKey := client.ObjectKey{Namespace: tradeBot.Namespace, Name: tradeBot.Name}
			if getErr := r.Get(ctx, latestKey, &latestTradeBotBeta); getErr != nil {
				if errors.IsNotFound(getErr) {
					logger.V(2).Info("TradeBot resource not found during finalizer removal, ignoring")
					return ctrl.Result{}, nil
				}
				logger.V(1).Error(getErr, "Failed to get latest TradeBot before finalizer removal")
				logger.V(2).Info("Requeue requested", "reason", "finalizer removal get error", "error", getErr)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, getErr
			}
			controllerutil.RemoveFinalizer(&latestTradeBotBeta, BotFinalizer)
			if err := r.Update(ctx, &latestTradeBotBeta); err != nil {
				logger.V(1).Error(err, "Failed to remove finalizer from TradeBot")
				logger.V(2).Info("Requeue requested", "reason", "finalizer update error", "error", err)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
			logger.V(1).Info("Finalizer removed from TradeBot", "name", latestTradeBotBeta.Name)
		}
		logger.V(1).Info("TradeBot deletion handling complete", "name", tradeBot.Name)
		return ctrl.Result{}, nil
	}

	// 3. Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&tradeBot, BotFinalizer) {
		logger.V(1).Info("Adding finalizer to TradeBot", "name", tradeBot.Name)
		// v1beta1, not tradeBot's own v1alpha1 copy - see the identical
		// note on the finalizer-removal Get below.
		var tradeBotBeta freqtradev1beta1.TradeBot
		if err := r.Get(ctx, req.NamespacedName, &tradeBotBeta); err != nil {
			logger.V(1).Error(err, "Failed to get TradeBot as v1beta1 to add finalizer")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, client.IgnoreNotFound(err)
		}
		controllerutil.AddFinalizer(&tradeBotBeta, BotFinalizer)
		if err := r.Update(ctx, &tradeBotBeta); err != nil {
			logger.V(1).Error(err, "Failed to add finalizer to TradeBot")
			logger.V(2).Info("Requeue requested", "reason", "add finalizer error", "error", err)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		logger.V(1).Info(
			"Finalizer added, returning to avoid further processing until update is processed", "name", tradeBot.Name,
		)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 3.5. Reject Job mode outright (P6-4): TradeBot is trade-only now that
	// v1beta1 has no FreqtradeCommand/FreqtradeArguments/Data fields at
	// all - Backtest is their replacement (D1, P6-1). Normal create/update
	// traffic can never actually reach this point with a non-trade command
	// set: v1beta1 is the storage version, so the conversion webhook
	// (api/v1alpha1/tradebot_conversion.go) already rejects it at the API
	// layer, on every write. This is a second, independent guard for the
	// one path that check can't cover - a TradeBot stored as v1alpha1
	// bytes from before this migration, which a plain Get (this reconciler
	// requests v1alpha1, a version-matched read needs no conversion at
	// all) can still read back successfully. Silently treating it as
	// trade-mode instead would be actively wrong, not just unsupported -
	// see the ground rules on financial risk.
	if cmd := strings.TrimSpace(tradeBot.Spec.FreqtradeCommand); cmd != "" && cmd != "trade" {
		return r.failReconcile(
			ctx, &tradeBot, freqtradev1alpha1.ConditionConfigResolved, freqtradev1alpha1.ReasonJobModeRemoved,
			fmt.Sprintf("spec.freqtrade_command=%q is no longer supported - TradeBot is trade-only; "+
				"recreate this as a Backtest instead", cmd), nil,
		)
	}

	// 4. Fetch all referenced CRDs
	logger.V(2).Info(
		"Fetching referenced resources for TradeBot", "strategy", tradeBot.Spec.Strategy, "config", tradeBot.Spec.Config,
	)
	resources, err := r.fetchReferencedResources(ctx, &tradeBot, req.Namespace)
	if err != nil {
		reason := freqtradev1alpha1.ReasonReconcileError
		if errors.IsNotFound(err) {
			reason = freqtradev1alpha1.ReasonReferenceNotFound
		}
		logger.Error(err, "Failed to fetch referenced resources")
		return r.failReconcile(ctx, &tradeBot, freqtradev1alpha1.ConditionConfigResolved, reason,
			fmt.Sprintf("Failed to fetch referenced resources: %v", err), err)
	}
	logger.V(2).Info("Fetched referenced resources", "strategy", tradeBot.Spec.Strategy, "config", tradeBot.Spec.Config)

	// 5. Collect CORS hosts from all FreqUI referencing this TradeBot
	var frequiList freqtradev1beta1.FreqUIList
	if err := r.List(ctx, &frequiList, client.InNamespace(tradeBot.Namespace)); err != nil {
		logger.Error(err, "Failed to list FreqUI resources")
		return ctrl.Result{}, fmt.Errorf("failed to list FreqUI resources: %w", err)
	}
	corsHosts := collectCORSHostsForTradeBot(&tradeBot, &frequiList)
	logger.V(2).Info("Collected CORS hosts", "corsHosts", corsHosts)
	freqUINames := referencingFreqUINames(&tradeBot, &frequiList)
	var corsWarning string
	if len(corsHosts) == 0 {
		corsWarning = "Warning: No CORS hosts configured. API may not be accessible from UIs. " +
			"Setting default CORS hosts (localhost and BOTNAME.SERVICE.svc.cluster.local)."
	} else if err := validateCORSHosts(corsHosts); err != nil {
		corsWarning = fmt.Sprintf("Warning: Invalid CORS host: %v", err)
	}

	// 6. Build config.json from TradeBot and referenced TradeBotConfig using configbuilder
	logger.V(2).Info("Building config.json for TradeBot", "name", tradeBot.Name)
	renderStart := time.Now()
	configData, err := configbuilder.BuildConfig(
		ctx, r.Client,
		tradeBot.Name, tradeBot.Namespace,
		resources.tradebotconfig,
		corsHosts,
	)
	shared.ConfigRenderDuration.Observe(time.Since(renderStart).Seconds())
	if err != nil {
		logger.Error(err, "Failed to build config")
		reason := freqtradev1alpha1.ReasonConfigInvalid
		if errors.IsNotFound(err) {
			reason = freqtradev1alpha1.ReasonSecretMissing
		}
		return r.failReconcile(ctx, &tradeBot, freqtradev1alpha1.ConditionConfigResolved, reason,
			fmt.Sprintf("Failed to build configuration: %v", err), err)
	}
	logger.V(2).Info("Config built successfully", "configDataKeys", keys(configData))

	// 7. Create or update required resources
	logger.V(2).Info("Reconciling resources", "configDataKeys", keys(configData))
	outcome, err := r.reconcileResources(ctx, &tradeBot, resources.strategy, configData, freqUINames)
	if err != nil {
		logger.Error(err, "Failed to reconcile resources")
		return r.failReconcile(
			ctx, &tradeBot, freqtradev1alpha1.ConditionWorkloadReady, freqtradev1alpha1.ReasonReconcileError,
			fmt.Sprintf("Failed to reconcile resources: %v", err), err,
		)
	}
	logger.V(2).Info("Resources reconciled successfully", "name", tradeBot.Name)

	// 8. Success path: reflect the underlying workload's state into status.
	// This is what actually clears a stale ConfigResolved/WorkloadReady=False
	// condition left over from a previous failed reconcile.
	workload, err := r.computeWorkloadStatus(ctx, &tradeBot)
	if err != nil {
		logger.Error(err, "Failed to read workload status")
		return r.failReconcile(
			ctx, &tradeBot, freqtradev1alpha1.ConditionWorkloadReady, freqtradev1alpha1.ReasonReconcileError,
			fmt.Sprintf("Failed to read workload status: %v", err), err,
		)
	}

	// A CORS misconfiguration doesn't stop the bot from running, but it is
	// actionable operator feedback, so it takes priority over a routine
	// workload status message.
	message := workload.message
	if corsWarning != "" {
		message = corsWarning
	}

	// Captured before PatchStatus overwrites tradeBot.Status below - this is
	// still exactly what r.Get returned at the top of Reconcile, since none
	// of the failReconcile branches above were taken if execution reached
	// here. P4-1's Events are about notable transitions, not routine
	// confirmations: comparing old vs new is what tells "became ready" apart
	// from "already was," and "config changed" apart from "reconciled a
	// no-op."
	wasWorkloadReady := meta.IsStatusConditionTrue(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionWorkloadReady)
	wasConfigDrift := meta.IsStatusConditionTrue(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionConfigDrift)
	hadWorkloadBefore := tradeBot.Status.ResolvedImage != ""
	oldConfigHash := tradeBot.Status.AppliedConfigHash

	if err := patchTradeBotStatus(ctx, r.Client, &tradeBot, func() {
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type:    freqtradev1alpha1.ConditionConfigResolved,
			Status:  metav1.ConditionTrue,
			Reason:  freqtradev1alpha1.ReasonAsExpected,
			Message: "",
		})
		status, reason := metav1.ConditionFalse, freqtradev1alpha1.ReasonWorkloadProgressing
		if workload.ready {
			status, reason = metav1.ConditionTrue, freqtradev1alpha1.ReasonWorkloadHealthy
		}
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type: freqtradev1alpha1.ConditionWorkloadReady, Status: status, Reason: reason, Message: workload.message,
		})
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type: freqtradev1alpha1.ConditionReady, Status: status, Reason: reason, Message: message,
		})
		meta.SetStatusCondition(&tradeBot.Status.Conditions,
			configDriftCondition(tradeBot.Name, tradeBot.Namespace, tradeBot.Generation, outcome.configDrift))
		tradeBot.Status.AppliedConfigHash = outcome.appliedConfigHash
		tradeBot.Status.ResolvedImage = outcome.resolvedImage
		tradeBot.Status.Phase = deriveTradeBotPhase(tradeBot.Status.Conditions)
		tradeBot.Status.Message = message
	}); err != nil {
		logger.Error(err, "Failed to update TradeBot status")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	r.recordLifecycleEvents(&tradeBot, lifecycleEventInputs{
		wasWorkloadReady:  wasWorkloadReady,
		wasConfigDrift:    wasConfigDrift,
		hadWorkloadBefore: hadWorkloadBefore,
		oldConfigHash:     oldConfigHash,
		newConfigHash:     outcome.appliedConfigHash,
	})

	// 9. Reconcile spec.state (P4-4). v1beta1-only (see reconcileState's
	// own doc comment on why this needs its own fetch), compared
	// continuously against the poller's own observation, never applied
	// once - a bot that crashes and restarts comes back Running and must
	// be re-stopped without anyone asking again.
	requeueAfter, err := r.reconcileStateAndComputeRequeue(ctx, req.NamespacedName, &tradeBot, workload.requeueAfter)
	if err != nil {
		logger.Error(err, "Failed to reconcile bot state")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	logger.V(1).Info("TradeBot reconciliation completed successfully",
		"name", tradeBot.Name, "phase", tradeBot.Status.Phase, "message", message)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// reconcileStateAndComputeRequeue fetches the TradeBot fresh as v1beta1,
// reconciles spec.state against it, and folds the result into
// defaultRequeueAfter - the shorter of the two wins, so a just-issued
// state change gets re-verified sooner than the routine interval.
// Extracted from Reconcile purely to keep its own cyclomatic complexity
// down; there's no other reason to split it here.
func (r *Reconciler) reconcileStateAndComputeRequeue(
	ctx context.Context, key client.ObjectKey, tradeBot *freqtradev1alpha1.TradeBot, defaultRequeueAfter time.Duration,
) (time.Duration, error) {
	var tradeBotBeta freqtradev1beta1.TradeBot
	if err := r.Get(ctx, key, &tradeBotBeta); err != nil {
		return defaultRequeueAfter, client.IgnoreNotFound(err)
	}
	stateChangeRequeue, err := r.reconcileState(ctx, tradeBot, &tradeBotBeta)
	if err != nil {
		return 0, err
	}
	if stateChangeRequeue > 0 && (defaultRequeueAfter == 0 || stateChangeRequeue < defaultRequeueAfter) {
		return stateChangeRequeue, nil
	}
	return defaultRequeueAfter, nil
}

// reconcileState compares tradeBotBeta.Spec.State (Running|Stopped,
// v1beta1-only, P4-4) against tradeBot.Status.Bot.State (the poller's most
// recent observation, P4-3) and, on a mismatch with a reachable bot,
// delegates to reconcileStateChange to actually issue the call. Split out
// specifically so tests can exercise reconcileStateChange directly against
// an httptest.Server - the same reason poll/pollWithClient are split
// (P4-3): a real bot's base URL is in-cluster DNS, which nothing in a unit
// test can make resolve to a local test server.
func (r *Reconciler) reconcileState(
	ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot, tradeBotBeta *freqtradev1beta1.TradeBot,
) (time.Duration, error) {
	desired := tradeBotBeta.Spec.State
	if desired == "" {
		// +kubebuilder:default=Running: a plain Get doesn't backfill CRD
		// defaults onto an in-memory struct for an object stored before
		// this field existed, so an explicit fallback is still needed here.
		desired = freqtradev1beta1.TradeBotStateRunning
	}

	if !meta.IsStatusConditionTrue(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionBotReachable) {
		// Never retry-storm an unreachable bot (P4-4): rely entirely on
		// the poller's own backoff to eventually flip BotReachable, rather
		// than probing the bot a second way on top of it.
		return 0, patchTradeBotStatus(ctx, r.Client, tradeBot, func() {
			meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
				Type: freqtradev1alpha1.ConditionStateReconciled, Status: metav1.ConditionFalse,
				Reason:  freqtradev1alpha1.ReasonBotUnreachable,
				Message: "Bot is unreachable; desired state cannot be verified or enforced",
			})
		})
	}

	observed := "unknown"
	if tradeBot.Status.Bot != nil {
		observed = tradeBot.Status.Bot.State
	}
	if (desired == freqtradev1beta1.TradeBotStateRunning && observed == "running") ||
		(desired == freqtradev1beta1.TradeBotStateStopped && observed == "stopped") {
		return 0, patchTradeBotStatus(ctx, r.Client, tradeBot, func() {
			meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
				Type: freqtradev1alpha1.ConditionStateReconciled, Status: metav1.ConditionTrue,
				Reason: freqtradev1alpha1.ReasonAsExpected,
			})
		})
	}

	username, password, _, err := resolveCredentials(ctx, r.Client, tradeBot)
	if err != nil {
		log.FromContext(ctx).Error(err, "Failed to resolve credentials for state reconciliation")
		return 0, patchTradeBotStatus(ctx, r.Client, tradeBot, func() {
			meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
				Type: freqtradev1alpha1.ConditionStateReconciled, Status: metav1.ConditionFalse,
				Reason: freqtradev1alpha1.ReasonStateChangeFailed, Message: err.Error(),
			})
		})
	}

	return r.reconcileStateChange(ctx, tradeBot, botclient.New(botBaseURL(tradeBot), username, password), desired)
}

// reconcileStateChange issues exactly one start or stop call to align a
// reachable bot's actual state with desired, then records the outcome -
// success (an Event, plus StateChangePending, since only the next poll
// actually confirms it landed) or failure (a Warning Event, plus
// StateChangeFailed) - into StateReconciled. "Requeue to re-verify, do not
// fire-and-forget," per the plan's own instruction: the returned duration
// is non-zero exactly when a call was just issued, so the caller re-checks
// once the poller's next poll has had a chance to confirm it.
//
// Hard scope boundary: start and stop only, via botClient's own Start/Stop
// - never anything that opens or closes a position on the human's behalf.
func (r *Reconciler) reconcileStateChange(
	ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot, botClient *botclient.Client, desired string,
) (time.Duration, error) {
	logger := log.FromContext(ctx)

	action, eventReason := "start", "BotStarted"
	var callErr error
	if desired == freqtradev1beta1.TradeBotStateStopped {
		action, eventReason = "stop", "BotStopped"
		callErr = botClient.Stop(ctx)
	} else {
		callErr = botClient.Start(ctx)
	}

	if callErr != nil {
		logger.Error(callErr, "Failed to change bot state", "action", action)
		if r.Recorder != nil {
			r.Recorder.Event(tradeBot, corev1.EventTypeWarning, "BotStateChangeFailed",
				fmt.Sprintf("Failed to %s bot: %v", action, callErr))
		}
		return 0, patchTradeBotStatus(ctx, r.Client, tradeBot, func() {
			meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
				Type: freqtradev1alpha1.ConditionStateReconciled, Status: metav1.ConditionFalse,
				Reason: freqtradev1alpha1.ReasonStateChangeFailed, Message: callErr.Error(),
			})
		})
	}

	if r.Recorder != nil {
		r.Recorder.Event(tradeBot, corev1.EventTypeNormal, eventReason,
			fmt.Sprintf("Issued %s to align with spec.state=%s", action, desired))
	}
	patchErr := patchTradeBotStatus(ctx, r.Client, tradeBot, func() {
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type: freqtradev1alpha1.ConditionStateReconciled, Status: metav1.ConditionFalse,
			Reason:  freqtradev1alpha1.ReasonStateChangePending,
			Message: fmt.Sprintf("Issued %s; waiting for the next poll to confirm", action),
		})
	})
	return pollInterval(tradeBot) + 5*time.Second, patchErr
}

// lifecycleEventInputs is the "before" half of the before/after comparison
// recordLifecycleEvents needs - captured pre-PatchStatus, since tradeBot's
// own Status has already been overwritten with the "after" values by the
// time recordLifecycleEvents runs.
type lifecycleEventInputs struct {
	wasWorkloadReady  bool
	wasConfigDrift    bool
	hadWorkloadBefore bool
	oldConfigHash     string
	newConfigHash     string
}

// recordLifecycleEvents emits the P4-1 Normal/Warning Events for the
// notable state transitions a successful Reconcile can produce - "config
// rendered," "workload created," "bot became ready," and "config drift
// detected" from the plan's own list ("restart triggered" is
// recordConfigRestartEvent, P2-4; "validation failed"/"secret missing" are
// failReconcile's, below). Reads tradeBot.Status post-PatchStatus, i.e. the
// "after" half.
func (r *Reconciler) recordLifecycleEvents(tradeBot *freqtradev1alpha1.TradeBot, before lifecycleEventInputs) {
	if r.Recorder == nil {
		return
	}

	if !before.hadWorkloadBefore && tradeBot.Status.ResolvedImage != "" {
		r.Recorder.Event(tradeBot, corev1.EventTypeNormal, "WorkloadCreated",
			"Workload created for the first time")
	}
	if before.newConfigHash != "" && before.newConfigHash != before.oldConfigHash {
		r.Recorder.Event(tradeBot, corev1.EventTypeNormal, "ConfigRendered",
			"Rendered config.json changed")
	}
	nowReady := meta.IsStatusConditionTrue(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionWorkloadReady)
	if !before.wasWorkloadReady && nowReady {
		r.Recorder.Event(tradeBot, corev1.EventTypeNormal, "BotReady", "Bot workload is ready")
	}
	nowDrifting := meta.IsStatusConditionTrue(tradeBot.Status.Conditions, freqtradev1alpha1.ConditionConfigDrift)
	if !before.wasConfigDrift && nowDrifting {
		r.Recorder.Event(tradeBot, corev1.EventTypeWarning, "ConfigDrift",
			"Rendered config has changed but is not yet reflected in the running workload - "+
				"see the ConfigDrift condition for the remedy")
	}
}

// workloadStatus is the raw outcome of inspecting the workload TradeBot
// owns, before it's translated into conditions. Exactly one of ready,
// succeeded, or failed is true unless the workload is still progressing (an
// unstarted/not-yet-ready StatefulSet or Job), in which case none are.
type workloadStatus struct {
	ready        bool
	message      string
	requeueAfter time.Duration
}

// computeWorkloadStatus inspects the StatefulSet TradeBot owns. Trade-only
// (P6-4) - by the time this runs, Reconcile has already rejected anything
// else (see its own step 3.5), so there's no Job branch here anymore.
func (r *Reconciler) computeWorkloadStatus(
	ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot,
) (workloadStatus, error) {
	key := client.ObjectKeyFromObject(tradeBot)

	var sts appsv1.StatefulSet
	if err := r.Get(ctx, key, &sts); err != nil {
		return workloadStatus{}, fmt.Errorf("failed to get StatefulSet: %w", err)
	}
	if sts.Status.ReadyReplicas < 1 {
		return workloadStatus{
			message:      fmt.Sprintf("Waiting for StatefulSet to become ready (%d ready)", sts.Status.ReadyReplicas),
			requeueAfter: 15 * time.Second,
		}, nil
	}
	return workloadStatus{ready: true}, nil
}

// deriveTradeBotPhase computes the human-facing Phase from Conditions - it
// is never itself the source of truth.
func deriveTradeBotPhase(conditions []metav1.Condition) string {
	configResolved := meta.FindStatusCondition(conditions, freqtradev1alpha1.ConditionConfigResolved)
	if configResolved != nil && configResolved.Status == metav1.ConditionFalse {
		if configResolved.Reason == freqtradev1alpha1.ReasonReferenceNotFound {
			return "Error"
		}
		return "ConfigError"
	}
	c := meta.FindStatusCondition(conditions, freqtradev1alpha1.ConditionWorkloadReady)
	if c == nil {
		return ""
	}
	switch c.Reason {
	case freqtradev1alpha1.ReasonWorkloadHealthy:
		return "Running"
	case freqtradev1alpha1.ReasonReconcileError:
		return "ResourceError"
	default:
		return "Pending"
	}
}

// failReconcileRequeueAfter is how soon a failed reconcile re-checks, when
// the failure itself isn't returned as an error (see failReconcile).
const failReconcileRequeueAfter = 30 * time.Second

// failReconcile sets conditionType and Ready to False with the given
// reason/message, derives Phase from the result, and returns a ctrl.Result:
// failReconcileRequeueAfter is honored when err is nil, otherwise
// controller-runtime's own exponential backoff takes over (its Result is
// ignored whenever a non-nil error is also returned).
func (r *Reconciler) failReconcile(
	ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot,
	conditionType, reason, message string, err error,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if patchErr := patchTradeBotStatus(ctx, r.Client, tradeBot, func() {
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type: conditionType, Status: metav1.ConditionFalse, Reason: reason, Message: message,
		})
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type: freqtradev1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: message,
		})
		tradeBot.Status.Phase = deriveTradeBotPhase(tradeBot.Status.Conditions)
		tradeBot.Status.Message = message
	}); patchErr != nil {
		logger.Error(patchErr, "Failed to update TradeBot status")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, patchErr
	}
	if r.Recorder != nil {
		r.Recorder.Event(tradeBot, corev1.EventTypeWarning, reason, message)
	}
	shared.ReconcileErrorsTotal.WithLabelValues("tradebot", reason).Inc()
	return ctrl.Result{RequeueAfter: failReconcileRequeueAfter}, err
}
