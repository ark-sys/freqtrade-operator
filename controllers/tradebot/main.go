package tradebot

import (
	"context"
	"fmt"
	"reflect"
	"time"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1/configbuilder"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// TradeBotFinalizer is the finalizer name used for TradeBot resources
	TradeBotFinalizer = "freqtrade.io/finalizer"
)

// Reconciler TradeBotReconciler reconciles a TradeBot object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for TradeBot resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.V(1).Info("Starting TradeBot reconciliation", "namespacedName", req.NamespacedName)

	// 1. Fetch TradeBot
	var tradeBot freqtradev1alpha1.TradeBot
	if err := r.Get(ctx, req.NamespacedName, &tradeBot); err != nil {
		if errors.IsNotFound(err) {
			// Object not found, likely deleted, nothing to do
			logger.Info("TradeBot resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get TradeBot resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Store original status for comparison later
	originalStatus := tradeBot.Status.DeepCopy()
	statusChanged := false

	// 2. Handle deletion if needed
	if tradeBot.GetDeletionTimestamp() != nil {
		logger.Info("TradeBot is being deleted", "name", tradeBot.Name)
		if controllerutil.ContainsFinalizer(&tradeBot, TradeBotFinalizer) {
			// Run finalization logic
			if err := r.finalizeTradeBot(ctx, &tradeBot); err != nil {
				logger.Error(err, "Failed to finalize TradeBot")
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}

			// Get the latest version of the TradeBot before removing the finalizer
			var latestTradeBot freqtradev1alpha1.TradeBot
			if getErr := r.Get(ctx, client.ObjectKey{Namespace: tradeBot.Namespace, Name: tradeBot.Name}, &latestTradeBot); getErr != nil {
				if errors.IsNotFound(getErr) {
					// Object is already gone, nothing to do
					logger.Info("TradeBot resource not found during finalizer removal, ignoring")
					return ctrl.Result{}, nil
				}
				logger.Error(getErr, "Failed to get latest TradeBot before finalizer removal")
				return ctrl.Result{RequeueAfter: 5 * time.Second}, getErr
			}

			// Remove finalizer from the latest version
			controllerutil.RemoveFinalizer(&latestTradeBot, TradeBotFinalizer)
			if err := r.Update(ctx, &latestTradeBot); err != nil {
				logger.Error(err, "Failed to remove finalizer from TradeBot")
				return ctrl.Result{RequeueAfter: 5 * time.Second}, err
			}
			logger.Info("Finalizer removed from TradeBot", "name", latestTradeBot.Name)
		}
		return ctrl.Result{}, nil
	}

	// 3. Add finalizer if it doesn't exist
	if !controllerutil.ContainsFinalizer(&tradeBot, TradeBotFinalizer) {
		logger.Info("Adding finalizer to TradeBot", "name", tradeBot.Name)
		controllerutil.AddFinalizer(&tradeBot, TradeBotFinalizer)
		if err := r.Update(ctx, &tradeBot); err != nil {
			logger.Error(err, "Failed to add finalizer to TradeBot")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, err
		}
		// Return here to avoid processing further until the update is processed
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Fetch all referenced CRDs
	resources, err := r.fetchReferencedResources(ctx, &tradeBot, req.Namespace)
	if err != nil {
		// Update status to reflect error
		tradeBot.Status.Phase = "Error"
		tradeBot.Status.Message = fmt.Sprintf("Failed to fetch referenced resources: %v", err)
		statusChanged = true
		return r.finishReconciliation(ctx, &tradeBot, originalStatus, statusChanged, 30*time.Second, err)
	}

	// 5. Assemble config.json from TradeBot and referenced CRDs using configbuilder
	configData, jwtSecretKey, err := configbuilder.AssembleConfig(
		ctx, r.Client,
		&tradeBot, resources.exchange,
		resources.entryPricing, resources.exitPricing, resources.order,
		resources.riskManagement, resources.notification, resources.strategy,
		resources.pairlistMethods, tradeBot.Status.JWTSecretKey,
	)
	if err != nil {
		logger.Error(err, "Failed to assemble config")
		tradeBot.Status.Phase = "ConfigError"
		tradeBot.Status.Message = fmt.Sprintf("Failed to assemble configuration: %v", err)
		statusChanged = true
		return r.finishReconciliation(ctx, &tradeBot, originalStatus, statusChanged, 30*time.Second, err)
	}

	// 6. Create or update required resources
	if err := r.reconcileResources(ctx, &tradeBot, configData); err != nil {
		logger.Error(err, "Failed to reconcile resources")
		tradeBot.Status.Phase = "ResourceError"
		tradeBot.Status.Message = fmt.Sprintf("Failed to reconcile resources: %v", err)
		statusChanged = true
		return r.finishReconciliation(ctx, &tradeBot, originalStatus, statusChanged, 30*time.Second, err)
	}

	// 7. Update status with success
	tradeBot.Status.Phase = "Running"
	tradeBot.Status.Message = "Bot deployed successfully"
	tradeBot.Status.JWTSecretKey = jwtSecretKey
	statusChanged = !reflect.DeepEqual(originalStatus, &tradeBot.Status)

	logger.Info("TradeBot reconciliation completed successfully", "name", tradeBot.Name)

	// If nothing changed and we're already in Running state, don't requeue
	if !statusChanged {
		return ctrl.Result{}, nil
	}

	// Otherwise, let finishReconciliation handle it
	return r.finishReconciliation(ctx, &tradeBot, originalStatus, statusChanged, 5*time.Minute, nil)
}

// finishReconciliation handles status updates and returns the appropriate result
func (r *Reconciler) finishReconciliation(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	originalStatus *freqtradev1alpha1.TradeBotStatus,
	statusChanged bool,
	requeueAfter time.Duration,
	err error) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	// Only update status if it has changed
	if statusChanged {
		// Get the latest version of the TradeBot before updating the status
		var latestTradeBot freqtradev1alpha1.TradeBot
		if getErr := r.Get(ctx, client.ObjectKey{Namespace: tradeBot.Namespace, Name: tradeBot.Name}, &latestTradeBot); getErr != nil {
			logger.Error(getErr, "Failed to get latest TradeBot before status update")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, getErr
		}

		// Apply our status changes to the latest version
		latestTradeBot.Status = tradeBot.Status

		// Update the status on the latest version
		updateErr := r.Status().Update(ctx, &latestTradeBot)
		if updateErr != nil {
			logger.Error(updateErr, "Failed to update TradeBot status")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, updateErr
		}
		logger.Info("Updated TradeBot status", "phase", latestTradeBot.Status.Phase)
	}

	// Return appropriate result based on error
	if err != nil {
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	if statusChanged {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// Otherwise, requeue after the specified duration
	return ctrl.Result{}, nil
}
