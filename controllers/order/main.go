package order

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
)

// Reconciler reconciles an Order object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	shared.StatusUpdater
}

// Reconcile handles the reconciliation loop for Order resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Starting Order reconciliation", "namespacedName", req.NamespacedName)

	// Fetch Order resource
	var order freqtradev1alpha1.Order
	if err := r.Get(ctx, req.NamespacedName, &order); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Order resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Order resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if empty
	if order.Status.Phase == "" {
		if err := r.UpdateConfigStatus(ctx, &order, "Validating", "Starting validation"); err != nil {
			logger.Error(err, "Failed to initialize Order status")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	// Validate Order configuration
	if err := r.validateOrder(ctx, &order); err != nil {
		logger.Error(err, "Order validation failed")
		if updateErr := r.UpdateConfigStatus(ctx, &order, "Invalid", err.Error()); updateErr != nil {
			logger.Error(updateErr, "Failed to update Order status after validation failure")
		}
		result := shared.FinishReconciliation("Invalid", err, 30*time.Second)
		return result.Result, result.Error
	}

	// Update status to valid
	if err := r.UpdateConfigStatus(ctx, &order, "Valid", "Order configuration is valid"); err != nil {
		logger.Error(err, "Failed to update Order status to valid")
		result := shared.FinishReconciliation("Valid", err, 30*time.Second)
		return result.Result, result.Error
	}

	logger.Info("Order reconciliation completed successfully", "name", order.Name)
	result := shared.FinishReconciliation("Valid", nil, 5*time.Minute)
	return result.Result, result.Error
}

// validateOrder performs validation of Order configuration
func (r *Reconciler) validateOrder(ctx context.Context, order *freqtradev1alpha1.Order) error {
	logger := log.FromContext(ctx)

	// Validate order types
	if order.Spec.Types != nil {
		if err := r.validateOrderTypes(order.Spec.Types); err != nil {
			return fmt.Errorf("order types validation failed: %w", err)
		}
	}

	// Validate time in force
	if order.Spec.TimeInForce != nil {
		if err := r.validateTimeInForce(order.Spec.TimeInForce); err != nil {
			return fmt.Errorf("time in force validation failed: %w", err)
		}
	}

	// Validate order flow
	if order.Spec.Flow != nil {
		if err := r.validateOrderFlow(order.Spec.Flow); err != nil {
			return fmt.Errorf("order flow validation failed: %w", err)
		}
	}

	logger.Info("Order validation passed", "name", order.Name)
	return nil
}

// validateOrderTypes validates order types configuration
func (r *Reconciler) validateOrderTypes(types *freqtradev1alpha1.Types) error {
	validOrderTypes := []string{"market", "limit"}

	// Validate entry order type
	if types.Entry != "" && !isValidOrderType(types.Entry, validOrderTypes) {
		return fmt.Errorf("entry order type must be one of: %v (current: %s)", validOrderTypes, types.Entry)
	}

	// Validate exit order type
	if types.Exit != "" && !isValidOrderType(types.Exit, validOrderTypes) {
		return fmt.Errorf("exit order type must be one of: %v (current: %s)", validOrderTypes, types.Exit)
	}

	// Validate emergency exit order type
	if types.EmergencyExit != "" && !isValidOrderType(types.EmergencyExit, validOrderTypes) {
		return fmt.Errorf("emergency_exit order type must be one of: %v (current: %s)", validOrderTypes, types.EmergencyExit)
	}

	// Validate force entry order type
	if types.ForceEntry != "" && !isValidOrderType(types.ForceEntry, validOrderTypes) {
		return fmt.Errorf("force_entry order type must be one of: %v (current: %s)", validOrderTypes, types.ForceEntry)
	}

	// Validate force exit order type
	if types.ForceExit != "" && !isValidOrderType(types.ForceExit, validOrderTypes) {
		return fmt.Errorf("force_exit order type must be one of: %v (current: %s)", validOrderTypes, types.ForceExit)
	}

	// Validate stoploss order type
	if types.Stoploss != "" && !isValidOrderType(types.Stoploss, validOrderTypes) {
		return fmt.Errorf("stoploss order type must be one of: %v (current: %s)", validOrderTypes, types.Stoploss)
	}

	// Validate stoploss on exchange interval
	if types.StoplossOnExchangeInterval != nil && *types.StoplossOnExchangeInterval <= 0 {
		return fmt.Errorf("stoploss_on_exchange_interval must be positive (current: %d)", *types.StoplossOnExchangeInterval)
	}

	// Validate stoploss on exchange limit ratio
	if types.StoplossOnExchangeLimitRatio != nil {
		if *types.StoplossOnExchangeLimitRatio < 0 || *types.StoplossOnExchangeLimitRatio > 1 {
			return fmt.Errorf("stoploss_on_exchange_limit_ratio must be between 0 and 1 (current: %f)", *types.StoplossOnExchangeLimitRatio)
		}
	}

	return nil
}

// validateTimeInForce validates time in force configuration
func (r *Reconciler) validateTimeInForce(tif *freqtradev1alpha1.TimeInForce) error {
	validTimeInForce := []string{"GTC", "FOK", "IOC"}

	// Validate entry time in force
	if tif.Entry != "" && !isValidTimeInForce(tif.Entry, validTimeInForce) {
		return fmt.Errorf("entry time_in_force must be one of: %v (current: %s)", validTimeInForce, tif.Entry)
	}

	// Validate exit time in force
	if tif.Exit != "" && !isValidTimeInForce(tif.Exit, validTimeInForce) {
		return fmt.Errorf("exit time_in_force must be one of: %v (current: %s)", validTimeInForce, tif.Exit)
	}

	return nil
}

// validateOrderFlow validates order flow configuration
func (r *Reconciler) validateOrderFlow(flow *freqtradev1alpha1.Flow) error {
	// Validate cache size
	if flow.CacheSize != nil && *flow.CacheSize <= 0 {
		return fmt.Errorf("cache_size must be positive (current: %d)", *flow.CacheSize)
	}

	// Validate max candles
	if flow.MaxCandles != nil && *flow.MaxCandles <= 0 {
		return fmt.Errorf("max_candles must be positive (current: %d)", *flow.MaxCandles)
	}

	// Validate scale
	if flow.Scale != nil && *flow.Scale <= 0 {
		return fmt.Errorf("scale must be positive (current: %f)", *flow.Scale)
	}

	// Validate stacked imbalance range
	if flow.StackedImbalanceRange != nil && *flow.StackedImbalanceRange <= 0 {
		return fmt.Errorf("stacked_imbalance_range must be positive (current: %d)", *flow.StackedImbalanceRange)
	}

	// Validate imbalance volume
	if flow.ImbalanceVolume != nil && *flow.ImbalanceVolume <= 0 {
		return fmt.Errorf("imbalance_volume must be positive (current: %d)", *flow.ImbalanceVolume)
	}

	// Validate imbalance ratio
	if flow.ImbalanceRatio != nil && (*flow.ImbalanceRatio < 0 || *flow.ImbalanceRatio > 1) {
		return fmt.Errorf("imbalance_ratio must be between 0 and 1 (current: %f)", *flow.ImbalanceRatio)
	}

	return nil
}

// isValidOrderType checks if the order type is valid
func isValidOrderType(orderType string, validTypes []string) bool {
	for _, valid := range validTypes {
		if orderType == valid {
			return true
		}
	}
	return false
}

// isValidTimeInForce checks if the time in force is valid
func isValidTimeInForce(tif string, validTifs []string) bool {
	for _, valid := range validTifs {
		if tif == valid {
			return true
		}
	}
	return false
}
