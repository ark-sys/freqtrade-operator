package tradebot

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ark-sys/freqtrade-operator/controllers/tradebotconfig/configbuilder"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
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
}

// collectCORSHostsForTradeBot returns a deduplicated, normalized list of CORS hosts.
func collectCORSHostsForTradeBot(tradeBot *freqtradev1alpha1.TradeBot, frequiList *freqtradev1alpha1.FreqUIList) []string {
	corsHosts := []string{}
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
	logger.Info("=== RECONCILE TRIGGERED ===", "namespacedName", req.NamespacedName, "reason", "unknown - need to check controller setup")

	// 1. Fetch TradeBot
	var tradeBot freqtradev1alpha1.TradeBot
	if err := r.Get(ctx, req.NamespacedName, &tradeBot); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("TradeBot resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get TradeBot resource")
		logger.Info("Requeue requested", "reason", "get error", "error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("=== RECONCILING TRADEBOT ===",
		"name", tradeBot.Name,
		"deletionTimestamp", tradeBot.GetDeletionTimestamp(),
		"finalizers", tradeBot.GetFinalizers(),
		"phase", tradeBot.Status.Phase,
		"message", tradeBot.Status.Message,
		"spec", tradeBot.Spec,
		"generation", tradeBot.Generation,
		"resourceVersion", tradeBot.ResourceVersion,
	)

	statusChanged := false

	// 2. Handle deletion if needed
	if tradeBot.GetDeletionTimestamp() != nil {
		logger.Info("TradeBot is being deleted", "name", tradeBot.Name)
		if controllerutil.ContainsFinalizer(&tradeBot, BotFinalizer) {
			logger.Info("Running finalization logic for TradeBot", "name", tradeBot.Name)
			if err := r.finalizeTradeBot(ctx, &tradeBot); err != nil {
				logger.Error(err, "Failed to finalize TradeBot")
				logger.Info("Requeue requested", "reason", "finalization error", "error", err)
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}

			var latestTradeBot freqtradev1alpha1.TradeBot
			if getErr := r.Get(ctx, client.ObjectKey{Namespace: tradeBot.Namespace, Name: tradeBot.Name}, &latestTradeBot); getErr != nil {
				if errors.IsNotFound(getErr) {
					logger.Info("TradeBot resource not found during finalizer removal, ignoring")
					return ctrl.Result{}, nil
				}
				logger.Error(getErr, "Failed to get latest TradeBot before finalizer removal")
				logger.Info("Requeue requested", "reason", "finalizer removal get error", "error", getErr)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, getErr
			}

			controllerutil.RemoveFinalizer(&latestTradeBot, BotFinalizer)
			if err := r.Update(ctx, &latestTradeBot); err != nil {
				logger.Error(err, "Failed to remove finalizer from TradeBot")
				logger.Info("Requeue requested", "reason", "finalizer update error", "error", err)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
			logger.Info("Finalizer removed from TradeBot", "name", latestTradeBot.Name)
		}
		logger.Info("TradeBot deletion handling complete", "name", tradeBot.Name)
		return ctrl.Result{}, nil
	}

	// 3. Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&tradeBot, BotFinalizer) {
		logger.Info("Adding finalizer to TradeBot", "name", tradeBot.Name)
		controllerutil.AddFinalizer(&tradeBot, BotFinalizer)
		if err := r.Update(ctx, &tradeBot); err != nil {
			logger.Error(err, "Failed to add finalizer to TradeBot")
			logger.Info("Requeue requested", "reason", "add finalizer error", "error", err)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		logger.Info("Finalizer added, returning to avoid further processing until update is processed", "name", tradeBot.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	// 4. Fetch all referenced CRDs
	logger.Info("Fetching referenced resources for TradeBot", "strategy", tradeBot.Spec.Strategy, "config", tradeBot.Spec.Config)
	resources, err := r.fetchReferencedResources(ctx, &tradeBot, req.Namespace)
	if err != nil {
		tradeBot.Status.Phase = "Error"
		tradeBot.Status.Message = fmt.Sprintf("Failed to fetch referenced resources: %v", err)
		statusChanged = true
		logger.Error(err, "Failed to fetch referenced resources")
		logger.Info("Requeue requested", "reason", "referenced resources error", "error", err)
		return r.finishReconciliation(ctx, &tradeBot, statusChanged, 30*time.Second, err)
	}
	logger.Info("Fetched referenced resources", "strategy", tradeBot.Spec.Strategy, "config", tradeBot.Spec.Config)

	// 5. Collect CORS hosts from all FreqUI referencing this TradeBot
	var frequiList freqtradev1alpha1.FreqUIList
	if err := r.List(ctx, &frequiList, client.InNamespace(tradeBot.Namespace)); err != nil {
		logger.Error(err, "Failed to list FreqUI resources")
		logger.Info("Requeue requested", "reason", "FreqUI list error", "error", err)
		return ctrl.Result{}, fmt.Errorf("failed to list FreqUI resources: %w", err)
	}
	corsHosts := collectCORSHostsForTradeBot(&tradeBot, &frequiList)
	logger.Info("Collected CORS hosts", "corsHosts", corsHosts)
	var newStatusMessage string
	if len(corsHosts) == 0 {
		logger.Info("No CORS hosts found for TradeBot", "name", tradeBot.Name)
		newStatusMessage = "Warning: No CORS hosts configured. API may not be accessible from UIs. Setting default CORS hosts (localhost and BOTNAME.SERVICE.svc.cluster.local)."
	} else if err := validateCORSHosts(corsHosts); err != nil {
		logger.Info("CORS host validation warning", "error", err)
		newStatusMessage = fmt.Sprintf("Warning: Invalid CORS host: %v", err)
	}

	logger.Info("=== STATUS UPDATE CHECK ===", "oldPhase", tradeBot.Status.Phase, "oldMessage", tradeBot.Status.Message, "newMessage", newStatusMessage)
	if newStatusMessage != "" && tradeBot.Status.Message != newStatusMessage {
		logger.Info("=== STATUS MESSAGE CHANGE DETECTED ===", "oldMessage", tradeBot.Status.Message, "newMessage", newStatusMessage)
		tradeBot.Status.Message = newStatusMessage
		statusChanged = true
	} else if newStatusMessage != "" {
		logger.Info("=== STATUS MESSAGE UNCHANGED ===", "message", tradeBot.Status.Message)
	} else {
		logger.Info("=== NO STATUS MESSAGE TO SET ===")
	}

	// 6. Build config.json from TradeBot and referenced TradeBotConfig using configbuilder
	logger.Info("Building config.json for TradeBot", "name", tradeBot.Name)
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
		statusChanged = true
		logger.Info("Requeue requested", "reason", "config build error", "error", err)
		return r.finishReconciliation(ctx, &tradeBot, statusChanged, 30*time.Second, err)
	}
	logger.Info("Config built successfully", "configDataKeys", keys(configData))

	// 7. Create or update required resources
	logger.Info("Reconciling resources", "configDataKeys", keys(configData))
	if err := r.reconcileResources(ctx, &tradeBot, configData); err != nil {
		logger.Error(err, "Failed to reconcile resources")
		tradeBot.Status.Phase = "ResourceError"
		tradeBot.Status.Message = fmt.Sprintf("Failed to reconcile resources: %v", err)
		statusChanged = true
		logger.Info("Requeue requested", "reason", "resource reconcile error", "error", err)
		return r.finishReconciliation(ctx, &tradeBot, statusChanged, 30*time.Second, err)
	}
	logger.Info("Resources reconciled successfully", "name", tradeBot.Name)

	logger.Info("TradeBot reconciliation completed successfully", "name", tradeBot.Name, "statusChanged", statusChanged, "phase", tradeBot.Status.Phase, "message", tradeBot.Status.Message)

	if !statusChanged {
		logger.Info("=== NO STATUS CHANGE - NOT REQUEUING ===", "name", tradeBot.Name, "currentPhase", tradeBot.Status.Phase, "currentMessage", tradeBot.Status.Message)
		return ctrl.Result{}, nil
	}

	logger.Info("=== STATUS CHANGED - FINISHING RECONCILIATION ===", "name", tradeBot.Name, "statusChanged", statusChanged)
	return r.finishReconciliation(ctx, &tradeBot, statusChanged, 0, nil)
}

// finishReconciliation handles status updates and returns the appropriate result
func (r *Reconciler) finishReconciliation(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	statusChanged bool,
	requeueAfter time.Duration,
	err error) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	if statusChanged {
		logger.Info("=== UPDATING STATUS ===", "name", tradeBot.Name, "newPhase", tradeBot.Status.Phase, "newMessage", tradeBot.Status.Message)

		var latestTradeBot freqtradev1alpha1.TradeBot
		if getErr := r.Get(ctx, client.ObjectKey{Namespace: tradeBot.Namespace, Name: tradeBot.Name}, &latestTradeBot); getErr != nil {
			logger.Error(getErr, "Failed to get latest TradeBot before status update")
			logger.Info("Requeue requested", "reason", "status update get error", "error", getErr)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, getErr
		}

		logger.Info("Current status before update", "currentPhase", latestTradeBot.Status.Phase, "currentMessage", latestTradeBot.Status.Message)
		latestTradeBot.Status = tradeBot.Status

		updateErr := r.Status().Update(ctx, &latestTradeBot)
		if updateErr != nil {
			logger.Error(updateErr, "Failed to update TradeBot status")
			logger.Info("Requeue requested", "reason", "status update error", "error", updateErr)
			return ctrl.Result{RequeueAfter: 5 * time.Second}, updateErr
		}
		logger.Info("=== STATUS UPDATE SUCCESSFUL ===", "phase", latestTradeBot.Status.Phase, "message", latestTradeBot.Status.Message)
	} else {
		logger.Info("=== NO STATUS UPDATE NEEDED ===", "name", tradeBot.Name)
	}

	if err != nil {
		logger.Info("Returning with error after reconciliation", "error", err, "requeueAfter", requeueAfter)
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	if statusChanged && requeueAfter == 0 {
		logger.Info("Status changed, no requeue needed", "statusChanged", statusChanged, "requeueAfter", requeueAfter)
		return ctrl.Result{}, nil
	}

	if statusChanged && requeueAfter > 0 {
		logger.Info("Status changed, requeuing", "statusChanged", statusChanged, "requeueAfter", requeueAfter)
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	logger.Info("No status change, no requeue needed", "statusChanged", statusChanged, "requeueAfter", requeueAfter)
	return ctrl.Result{}, nil
}
