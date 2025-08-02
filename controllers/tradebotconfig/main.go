package tradebotconfig

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
)

// Reconciler reconciles a Tradebotconfig object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	shared.StatusUpdater
}

// Reconcile handles the reconciliation loop for Tradebotconfig resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Starting Tradebotconfig reconciliation", "namespacedName", req.NamespacedName)

	// Fetch Tradebotconfig resource
	var tradebotconfig freqtradev1alpha1.TradeBotConfig
	if err := r.Get(ctx, req.NamespacedName, &tradebotconfig); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Tradebotconfig resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Tradebotconfig resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if empty
	if tradebotconfig.Status.Phase == "" {
		if err := r.RetryUpdateConfigStatus(ctx, &tradebotconfig, "Validating", "Starting validation", 3); err != nil {
			logger.Error(err, "Failed to initialize Tradebotconfig status")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	// Validate Tradebotconfig configuration
	if err := r.validateTradeBotConfig(ctx, &tradebotconfig); err != nil {
		logger.Error(err, "Tradebotconfig validation failed")
		if updateErr := r.RetryUpdateConfigStatus(ctx, &tradebotconfig, "Invalid", err.Error(), 3); updateErr != nil {
			logger.Error(updateErr, "Failed to update Tradebotconfig status after validation failure")
		}
		result := shared.FinishReconciliation("Invalid", err, 30*time.Second)
		return result.Result, result.Error
	}

	// Update status to valid
	if err := r.RetryUpdateConfigStatus(ctx, &tradebotconfig, "Valid", "Tradebotconfig configuration is valid", 3); err != nil {
		logger.Error(err, "Failed to update Tradebotconfig status to valid")
		result := shared.FinishReconciliation("Valid", err, 30*time.Second)
		return result.Result, result.Error
	}

	logger.Info("Tradebotconfig reconciliation completed successfully", "name", tradebotconfig.Name)
	result := shared.FinishReconciliation("Valid", nil, 5*time.Minute)
	return result.Result, result.Error
}

// validateTradebotconfig performs validation of Tradebotconfig configuration
func (r *Reconciler) validateTradeBotConfig(ctx context.Context, tradebotconfig *freqtradev1alpha1.TradeBotConfig) error {
	logger := log.FromContext(ctx)

	//// Validate required fields
	//if tradebotconfig.Spec.Bot.BotName == "" {
	//	return fmt.Errorf("tradebotconfig name is required")
	//}

	// TODO Check other required fields

	logger.Info("Tradebotconfig validation passed", "name", tradebotconfig.Name)
	return nil
}
