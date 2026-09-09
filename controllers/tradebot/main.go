package tradebot

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebotconfig/configbuilder"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
}

// collectCORSHostsForTradeBot returns a deduplicated, normalized list of CORS hosts.
func collectCORSHostsForTradeBot(tradeBot *freqtradev1alpha1.TradeBot, frequiList *freqtradev1alpha1.FreqUIList) []string {
	var corsHosts []string
	corsSet := make(map[string]struct{})

	for _, frequi := range frequiList.Items {
		for _, ref := range frequi.Spec.TradeBotRefs {
			if ref == tradeBot.Name {
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

					if !strings.HasPrefix(hostname, "localhost") {
						botSubdomain := fmt.Sprintf("%s://%s.%s", scheme, tradeBot.Name, hostname)
						if _, exists := corsSet[botSubdomain]; !exists {
							corsHosts = append(corsHosts, botSubdomain)
							corsSet[botSubdomain] = struct{}{}
						}
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
	var k []string
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
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;patch;delete
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

	logger.V(1).Info("Reconciling TradeBot", "name", tradeBot.Name, "generation", tradeBot.Generation, "phase", tradeBot.Status.Phase)

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
			var latestTradeBot freqtradev1alpha1.TradeBot
			if getErr := r.Get(ctx, client.ObjectKey{Namespace: tradeBot.Namespace, Name: tradeBot.Name}, &latestTradeBot); getErr != nil {
				if errors.IsNotFound(getErr) {
					logger.V(2).Info("TradeBot resource not found during finalizer removal, ignoring")
					return ctrl.Result{}, nil
				}
				logger.V(1).Error(getErr, "Failed to get latest TradeBot before finalizer removal")
				logger.V(2).Info("Requeue requested", "reason", "finalizer removal get error", "error", getErr)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, getErr
			}
			controllerutil.RemoveFinalizer(&latestTradeBot, BotFinalizer)
			if err := r.Update(ctx, &latestTradeBot); err != nil {
				logger.V(1).Error(err, "Failed to remove finalizer from TradeBot")
				logger.V(2).Info("Requeue requested", "reason", "finalizer update error", "error", err)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
			logger.V(1).Info("Finalizer removed from TradeBot", "name", latestTradeBot.Name)
		}
		logger.V(1).Info("TradeBot deletion handling complete", "name", tradeBot.Name)
		return ctrl.Result{}, nil
	}

	// 3. Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&tradeBot, BotFinalizer) {
		logger.V(1).Info("Adding finalizer to TradeBot", "name", tradeBot.Name)
		controllerutil.AddFinalizer(&tradeBot, BotFinalizer)
		if err := r.Update(ctx, &tradeBot); err != nil {
			logger.V(1).Error(err, "Failed to add finalizer to TradeBot")
			logger.V(2).Info("Requeue requested", "reason", "add finalizer error", "error", err)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		logger.V(1).Info("Finalizer added, returning to avoid further processing until update is processed", "name", tradeBot.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 4. Fetch all referenced CRDs
	logger.V(2).Info("Fetching referenced resources for TradeBot", "strategy", tradeBot.Spec.Strategy, "config", tradeBot.Spec.Config)
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
	var frequiList freqtradev1alpha1.FreqUIList
	if err := r.List(ctx, &frequiList, client.InNamespace(tradeBot.Namespace)); err != nil {
		logger.Error(err, "Failed to list FreqUI resources")
		return ctrl.Result{}, fmt.Errorf("failed to list FreqUI resources: %w", err)
	}
	corsHosts := collectCORSHostsForTradeBot(&tradeBot, &frequiList)
	logger.V(2).Info("Collected CORS hosts", "corsHosts", corsHosts)
	var corsWarning string
	if len(corsHosts) == 0 {
		corsWarning = "Warning: No CORS hosts configured. API may not be accessible from UIs. Setting default CORS hosts (localhost and BOTNAME.SERVICE.svc.cluster.local)."
	} else if err := validateCORSHosts(corsHosts); err != nil {
		corsWarning = fmt.Sprintf("Warning: Invalid CORS host: %v", err)
	}

	// 6. Build config.json from TradeBot and referenced TradeBotConfig using configbuilder
	logger.V(2).Info("Building config.json for TradeBot", "name", tradeBot.Name)
	configData, err := configbuilder.BuildConfig(
		ctx, r.Client,
		&tradeBot,
		resources.tradebotconfig,
		corsHosts,
	)
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
	jobSpecChanged, err := r.reconcileResources(ctx, &tradeBot, resources.strategy, configData)
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

	if err := shared.PatchStatus(ctx, r.Client, &tradeBot, func() {
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type:    freqtradev1alpha1.ConditionConfigResolved,
			Status:  metav1.ConditionTrue,
			Reason:  freqtradev1alpha1.ReasonAsExpected,
			Message: "",
		})
		var status metav1.ConditionStatus
		var reason string
		switch {
		case workload.failed:
			status, reason = metav1.ConditionFalse, freqtradev1alpha1.ReasonWorkloadFailed
		case workload.succeeded:
			status, reason = metav1.ConditionTrue, freqtradev1alpha1.ReasonWorkloadSucceeded
		case workload.ready:
			status, reason = metav1.ConditionTrue, freqtradev1alpha1.ReasonWorkloadHealthy
		default:
			status, reason = metav1.ConditionFalse, freqtradev1alpha1.ReasonWorkloadProgressing
		}
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type: freqtradev1alpha1.ConditionWorkloadReady, Status: status, Reason: reason, Message: workload.message,
		})
		meta.SetStatusCondition(&tradeBot.Status.Conditions, metav1.Condition{
			Type: freqtradev1alpha1.ConditionReady, Status: status, Reason: reason, Message: message,
		})
		meta.SetStatusCondition(&tradeBot.Status.Conditions,
			workloadImmutableCondition(tradeBot.Name, tradeBot.Generation, jobSpecChanged))
		tradeBot.Status.Phase = deriveTradeBotPhase(tradeBot.Status.Conditions)
		tradeBot.Status.Message = message
	}); err != nil {
		logger.Error(err, "Failed to update TradeBot status")
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	logger.V(1).Info("TradeBot reconciliation completed successfully",
		"name", tradeBot.Name, "phase", tradeBot.Status.Phase, "message", message)
	return ctrl.Result{RequeueAfter: workload.requeueAfter}, nil
}

// workloadStatus is the raw outcome of inspecting the workload TradeBot
// owns, before it's translated into conditions. Exactly one of ready,
// succeeded, or failed is true unless the workload is still progressing (an
// unstarted/not-yet-ready StatefulSet or Job), in which case none are.
type workloadStatus struct {
	ready        bool
	succeeded    bool
	failed       bool
	message      string
	requeueAfter time.Duration
}

// computeWorkloadStatus inspects the workload TradeBot owns (a StatefulSet
// in trade mode, a Job otherwise).
func (r *Reconciler) computeWorkloadStatus(
	ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot,
) (workloadStatus, error) {
	effectiveCmd := strings.TrimSpace(tradeBot.Spec.FreqtradeCommand)
	if effectiveCmd == "" {
		effectiveCmd = "trade"
	}

	key := client.ObjectKeyFromObject(tradeBot)

	if effectiveCmd == "trade" {
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

	var job batchv1.Job
	if err := r.Get(ctx, key, &job); err != nil {
		return workloadStatus{}, fmt.Errorf("failed to get Job: %w", err)
	}
	switch {
	case job.Status.Succeeded > 0:
		return workloadStatus{succeeded: true}, nil
	case job.Status.Failed > 0:
		return workloadStatus{failed: true, message: "Job failed; check pod logs"}, nil
	case job.Status.Active > 0:
		return workloadStatus{message: "Job is running", requeueAfter: 15 * time.Second}, nil
	default:
		return workloadStatus{message: "Waiting for Job to start", requeueAfter: 15 * time.Second}, nil
	}
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
	case freqtradev1alpha1.ReasonWorkloadSucceeded:
		return "Succeeded"
	case freqtradev1alpha1.ReasonWorkloadFailed:
		return "Failed"
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
	if patchErr := shared.PatchStatus(ctx, r.Client, tradeBot, func() {
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
	return ctrl.Result{RequeueAfter: failReconcileRequeueAfter}, err
}
