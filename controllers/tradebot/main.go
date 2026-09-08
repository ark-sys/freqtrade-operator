package tradebot

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/ark-sys/freqtrade-operator/controllers/tradebotconfig/configbuilder"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

// Reconcile handles the reconciliation loop for TradeBot resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("=== RECONCILE TRIGGERED ===", "namespacedName", req.NamespacedName, "reason", "unknown - need to check controller setup")

	// 1. Fetch TradeBot
	var tradeBot freqtradev1alpha1.TradeBot
	if err := r.Get(ctx, req.NamespacedName, &tradeBot); err != nil {
		if errors.IsNotFound(err) {
			logger.V(1).Info("TradeBot resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.V(1).Error(err, "Failed to get TradeBot resource")
		logger.V(2).Info("Requeue requested", "reason", "get error", "error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.V(1).Info("=== RECONCILING TRADEBOT ===",
		"name", tradeBot.Name,
		"deletionTimestamp", tradeBot.GetDeletionTimestamp(),
		"finalizers", tradeBot.GetFinalizers(),
		"phase", tradeBot.Status.Phase,
		"message", tradeBot.Status.Message,
		"spec", tradeBot.Spec,
		"generation", tradeBot.Generation,
		"resourceVersion", tradeBot.ResourceVersion,
	)

	// Snapshot status on entry; finishReconciliation diffs against this to decide
	// whether a status write is needed, instead of hand-tracking a bool across
	// every branch below (see controllers/frequi/main.go for the same pattern).
	originalStatus := tradeBot.Status.DeepCopy()

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
		tradeBot.Status.Phase = "Error"
		tradeBot.Status.Message = fmt.Sprintf("Failed to fetch referenced resources: %v", err)
		logger.Error(err, "Failed to fetch referenced resources")
		return r.finishReconciliation(ctx, &tradeBot, originalStatus, 30*time.Second, err)
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
		tradeBot.Status.Phase = "ConfigError"
		tradeBot.Status.Message = fmt.Sprintf("Failed to build configuration: %v", err)
		return r.finishReconciliation(ctx, &tradeBot, originalStatus, 30*time.Second, err)
	}
	logger.V(2).Info("Config built successfully", "configDataKeys", keys(configData))

	// 7. Create or update required resources
	logger.V(2).Info("Reconciling resources", "configDataKeys", keys(configData))
	if err := r.reconcileResources(ctx, &tradeBot, resources.strategy, configData); err != nil {
		logger.Error(err, "Failed to reconcile resources")
		tradeBot.Status.Phase = "ResourceError"
		tradeBot.Status.Message = fmt.Sprintf("Failed to reconcile resources: %v", err)
		return r.finishReconciliation(ctx, &tradeBot, originalStatus, 30*time.Second, err)
	}
	logger.V(2).Info("Resources reconciled successfully", "name", tradeBot.Name)

	// 8. Success path: reflect the underlying workload's state into status.
	// This is what actually clears a stale ConfigError/ResourceError phase
	// left over from a previous failed reconcile.
	requeueAfter, err := r.updateWorkloadStatus(ctx, &tradeBot)
	if err != nil {
		logger.Error(err, "Failed to read workload status")
		tradeBot.Status.Phase = "Error"
		tradeBot.Status.Message = fmt.Sprintf("Failed to read workload status: %v", err)
		return r.finishReconciliation(ctx, &tradeBot, originalStatus, 30*time.Second, err)
	}
	// A CORS misconfiguration doesn't stop the bot from running, but it is
	// actionable operator feedback, so it takes priority over a routine
	// "Running"/"Pending" message.
	if corsWarning != "" {
		tradeBot.Status.Message = corsWarning
	}

	logger.V(1).Info("TradeBot reconciliation completed successfully", "name", tradeBot.Name, "phase", tradeBot.Status.Phase, "message", tradeBot.Status.Message)
	return r.finishReconciliation(ctx, &tradeBot, originalStatus, requeueAfter, nil)
}

// updateWorkloadStatus reflects the state of the workload TradeBot owns (a
// StatefulSet in trade mode, a Job otherwise) into tradeBot.Status, and
// returns how soon to requeue to re-check a not-yet-ready workload.
func (r *Reconciler) updateWorkloadStatus(ctx context.Context, tradeBot *freqtradev1alpha1.TradeBot) (time.Duration, error) {
	effectiveCmd := strings.TrimSpace(tradeBot.Spec.FreqtradeCommand)
	if effectiveCmd == "" {
		effectiveCmd = "trade"
	}

	key := client.ObjectKeyFromObject(tradeBot)

	if effectiveCmd == "trade" {
		var sts appsv1.StatefulSet
		if err := r.Get(ctx, key, &sts); err != nil {
			return 0, fmt.Errorf("failed to get StatefulSet: %w", err)
		}
		if sts.Status.ReadyReplicas < 1 {
			tradeBot.Status.Phase = "Pending"
			tradeBot.Status.Message = fmt.Sprintf("Waiting for StatefulSet to become ready (%d ready)", sts.Status.ReadyReplicas)
			return 15 * time.Second, nil
		}
		tradeBot.Status.Phase = "Running"
		tradeBot.Status.Message = ""
		return 0, nil
	}

	var job batchv1.Job
	if err := r.Get(ctx, key, &job); err != nil {
		return 0, fmt.Errorf("failed to get Job: %w", err)
	}
	switch {
	case job.Status.Succeeded > 0:
		tradeBot.Status.Phase = "Succeeded"
		tradeBot.Status.Message = ""
		return 0, nil
	case job.Status.Failed > 0:
		tradeBot.Status.Phase = "Failed"
		tradeBot.Status.Message = "Job failed; check pod logs"
		return 0, nil
	case job.Status.Active > 0:
		tradeBot.Status.Phase = "Running"
		tradeBot.Status.Message = ""
		return 15 * time.Second, nil
	default:
		tradeBot.Status.Phase = "Pending"
		tradeBot.Status.Message = "Waiting for Job to start"
		return 15 * time.Second, nil
	}
}

// finishReconciliation writes tradeBot.Status if it differs from originalStatus
// (the snapshot taken at the top of Reconcile) and returns the requeue result.
func (r *Reconciler) finishReconciliation(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	originalStatus *freqtradev1alpha1.TradeBotStatus,
	requeueAfter time.Duration,
	err error) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	if !reflect.DeepEqual(originalStatus, &tradeBot.Status) {
		var latestTradeBot freqtradev1alpha1.TradeBot
		if getErr := r.Get(ctx, client.ObjectKeyFromObject(tradeBot), &latestTradeBot); getErr != nil {
			logger.Error(getErr, "Failed to get latest TradeBot before status update")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, getErr
		}

		latestTradeBot.Status = tradeBot.Status
		if updateErr := r.Status().Update(ctx, &latestTradeBot); updateErr != nil {
			logger.Error(updateErr, "Failed to update TradeBot status")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, updateErr
		}
		logger.V(1).Info("Updated TradeBot status", "name", tradeBot.Name, "phase", latestTradeBot.Status.Phase)
	}

	if err != nil {
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}
