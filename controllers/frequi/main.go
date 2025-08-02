package frequi

import (
	"context"
	"fmt"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/frequi/resources"
)

// Reconciler FreqUIReconciler reconciles a FreqUI object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile handles the reconciliation loop for FreqUI resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.V(1).Info("Starting FreqUI reconciliation", "namespacedName", req.NamespacedName)

	// 1. Fetch FreqUI resource
	var frequi freqtradev1alpha1.FreqUI
	if err := r.Get(ctx, req.NamespacedName, &frequi); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FreqUI resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get FreqUI resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Store original status for comparison later
	originalStatus := frequi.Status.DeepCopy()
	statusChanged := false

	// Initialize status if empty
	if frequi.Status.Phase == "" {
		frequi.Status.Phase = "Initializing"
		frequi.Status.Message = "Starting FreqUI deployment"
		statusChanged = true
	}

	// 2. Reconcile all resources
	if err := r.reconcileAllResources(ctx, &frequi); err != nil {
		logger.Error(err, "Failed to reconcile resources")
		frequi.Status.Phase = "ResourceError"
		frequi.Status.Message = fmt.Sprintf("Failed to reconcile resources: %v", err)
		statusChanged = true
		return r.finishReconciliation(ctx, &frequi, statusChanged, 30*time.Second, err)
	}

	// 3. Check if deployment is ready
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: frequi.Name, Namespace: frequi.Namespace}, deployment); err != nil {
		logger.Error(err, "Failed to get deployment status")
		frequi.Status.Phase = "DeploymentError"
		frequi.Status.Message = fmt.Sprintf("Failed to get deployment status: %v", err)
		statusChanged = true
		return r.finishReconciliation(ctx, &frequi, statusChanged, 30*time.Second, err)
	}

	// 4. Update status based on deployment readiness
	if isDeploymentReady(deployment) {
		frequi.Status.Phase = "Running"
		frequi.Status.Message = "FreqUI deployed successfully"

		// Set URL based on service
		frequi.Status.URL = fmt.Sprintf("http://%s.%s.svc.cluster.local", frequi.Name, frequi.Namespace)

		statusChanged = !reflect.DeepEqual(originalStatus, &frequi.Status)

		logger.Info("FreqUI reconciliation completed successfully", "name", frequi.Name)

		// If nothing changed and we're already in Running state, don't requeue
		if !statusChanged {
			logger.V(1).Info("FreqUI is running and stable, no requeue needed")
			return ctrl.Result{}, nil
		}
		// Otherwise, requeue after a longer period
		return r.finishReconciliation(ctx, &frequi, true, 5*time.Minute, nil)
	}

	// Deployment is not ready yet, update status and requeue sooner
	newPhase := "Pending"
	newMessage := fmt.Sprintf("Waiting for deployment to be ready (%d/%d replicas)",
		deployment.Status.AvailableReplicas, deployment.Status.Replicas)

	// Only update status if phase or message actually changed to reduce reconciliation triggers
	if frequi.Status.Phase != newPhase || frequi.Status.Message != newMessage {
		frequi.Status.Phase = newPhase
		frequi.Status.Message = newMessage
		statusChanged = true
		logger.Info("App not ready yet, updating status and requeuing",
			"name", frequi.Name,
			"availableReplicas", deployment.Status.AvailableReplicas,
			"replicas", deployment.Status.Replicas,
			"newMessage", newMessage)
	} else {
		// Status unchanged, just requeue without status update
		statusChanged = false
		logger.V(1).Info("App still not ready, requeuing without status update",
			"name", frequi.Name,
			"availableReplicas", deployment.Status.AvailableReplicas,
			"replicas", deployment.Status.Replicas)
	}

	// Increase requeue time to reduce frequent checks (from 10s to 15s)
	return r.finishReconciliation(ctx, &frequi, statusChanged, 15*time.Second, nil)
}

// finishReconciliation handles status updates and returns the appropriate result
func (r *Reconciler) finishReconciliation(
	ctx context.Context,
	frequi *freqtradev1alpha1.FreqUI,
	statusChanged bool,
	requeueAfter time.Duration,
	err error) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	// Only update status if it has changed
	if statusChanged {
		updateErr := r.retryUpdateFreqUIStatus(ctx, frequi, 3)
		if updateErr != nil {
			logger.Error(updateErr, "Failed to update FreqUI status after retries")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, updateErr
		}
		logger.Info("Updated FreqUI status", "phase", frequi.Status.Phase)
	}

	// Return appropriate result based on error
	if err != nil {
		// When returning an error, don't specify RequeueAfter as controller-runtime will use exponential backoff
		return ctrl.Result{}, err
	}

	// If status is "Running" and no changes were made, don't requeue
	if frequi.Status.Phase == "Running" && !statusChanged {
		logger.V(1).Info("FreqUI is running and stable, no requeue needed")
		return ctrl.Result{}, nil
	}

	// Otherwise, requeue after the specified duration
	if statusChanged {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	return ctrl.Result{}, nil
}

// reconcileAllResources creates or updates all resources needed by FreqUI
func (r *Reconciler) reconcileAllResources(ctx context.Context, frequi *freqtradev1alpha1.FreqUI) error {
	logger := log.FromContext(ctx)

	// 1. Reconcile Deployment
	deployment := resources.BuildFreqUIDeployment(*frequi)
	if err := resources.ApplyDeployment(ctx, r.Client, &deployment); err != nil {
		return fmt.Errorf("failed to apply deployment: %w", err)
	}
	logger.V(1).Info("Deployment reconciled", "name", deployment.Name)

	// 2. Reconcile Service
	service := resources.BuildFreqUIService(*frequi)
	if err := resources.ApplyService(ctx, r.Client, &service); err != nil {
		return fmt.Errorf("failed to apply service: %w", err)
	}
	logger.V(1).Info("Service reconciled", "name", service.Name)

	// 3. Reconcile Ingress if configured
	var apiRoutes []resources.TradeBotAPIRoute
	for _, tradeBotRef := range frequi.Spec.TradeBotRefs {
		var tradeBot freqtradev1alpha1.TradeBot
		err := r.Get(ctx, types.NamespacedName{Name: tradeBotRef, Namespace: frequi.Namespace}, &tradeBot)
		if err == nil {
			// Retrieve TradeBotConfig for the TradeBot
			var tradeBotConfig freqtradev1alpha1.TradeBotConfig
			if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.Config, Namespace: frequi.Namespace}, &tradeBotConfig); err != nil {
				logger.Error(err, "Failed to get TradeBotConfig for TradeBot", "tradeBot", tradeBotRef)
				continue
			}

			// Only add API route if TradeBotConfig is valid and API server is enabled
			if tradeBotConfig.Status.Phase == "Valid" && tradeBotConfig.Spec.APIServer != nil &&
				tradeBotConfig.Spec.APIServer.Enabled != nil && *tradeBotConfig.Spec.APIServer.Enabled {
				apiRoutes = append(apiRoutes, resources.TradeBotAPIRoute{
					Name:        tradeBotRef,
					ServiceName: tradeBotRef,
					PathPrefix:  tradeBotRef,
				})
			} else {
				logger.V(1).Info("Skipping TradeBot API route", "tradeBot", tradeBotRef,
					"reason", "TradeBotConfig is not valid or API server is disabled")
			}
		} else if !errors.IsNotFound(err) {
			logger.Error(err, "Failed to get TradeBot for API route", "tradeBot", tradeBotRef)
			return fmt.Errorf("failed to get TradeBot %s: %w", tradeBotRef, err)
		} else {
			logger.V(1).Info("TradeBot not found for API route", "tradeBot", tradeBotRef)
			// If TradeBot is not found, we skip adding it to the routes
			continue
		}

	}

	ingress := resources.BuildFreqUIIngress(*frequi, apiRoutes)
	if err := resources.ApplyIngress(ctx, r.Client, &ingress); err != nil {
		return fmt.Errorf("failed to apply ingress: %w", err)
	}
	logger.V(1).Info("Ingress reconciled", "name", ingress.Name)

	return nil
}

// retryUpdateFreqUIStatus updates the FreqUI status with retry logic to handle resource version conflicts
func (r *Reconciler) retryUpdateFreqUIStatus(ctx context.Context, frequi *freqtradev1alpha1.FreqUI, maxRetries int) error {
	logger := log.FromContext(ctx)

	for i := 0; i < maxRetries; i++ {
		// Get the latest version to avoid conflicts
		var latest freqtradev1alpha1.FreqUI
		if err := r.Get(ctx, client.ObjectKeyFromObject(frequi), &latest); err != nil {
			return fmt.Errorf("failed to get latest FreqUI version: %w", err)
		}

		// Update the status on the latest version
		latest.Status = frequi.Status

		// Try to update the status
		if err := r.Status().Update(ctx, &latest); err != nil {
			if errors.IsConflict(err) && i < maxRetries-1 {
				logger.V(1).Info("FreqUI status update conflict, retrying", "attempt", i+1, "maxRetries", maxRetries)
				time.Sleep(100 * time.Millisecond) // Brief backoff
				continue
			}
			return fmt.Errorf("failed to update FreqUI status: %w", err)
		}

		// Update successful
		if i > 0 {
			logger.Info("FreqUI status update succeeded after retry", "attempts", i+1)
		}
		return nil
	}

	return fmt.Errorf("failed to update FreqUI status after %d retries", maxRetries)
}

// isDeploymentReady checks if a deployment is ready
func isDeploymentReady(deployment *appsv1.Deployment) bool {
	return deployment.Status.AvailableReplicas == deployment.Status.Replicas &&
		deployment.Status.Replicas > 0
}
