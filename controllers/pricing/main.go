package pricing

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

// Reconciler reconciles a Pricing object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	shared.StatusUpdater
}

// Reconcile handles the reconciliation loop for Pricing resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Starting Pricing reconciliation", "namespacedName", req.NamespacedName)

	// Fetch Pricing resource
	var pricing freqtradev1alpha1.Pricing
	if err := r.Get(ctx, req.NamespacedName, &pricing); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Pricing resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Pricing resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if empty
	if pricing.Status.Phase == "" {
		if err := r.UpdateConfigStatus(ctx, &pricing, "Validating", "Starting validation"); err != nil {
			logger.Error(err, "Failed to initialize Pricing status")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	// Validate Pricing configuration
	if err := r.validatePricing(ctx, &pricing); err != nil {
		logger.Error(err, "Pricing validation failed")
		if updateErr := r.UpdateConfigStatus(ctx, &pricing, "Invalid", err.Error()); updateErr != nil {
			logger.Error(updateErr, "Failed to update Pricing status after validation failure")
		}
		result := shared.FinishReconciliation("Invalid", err, 30*time.Second)
		return result.Result, result.Error
	}

	// Update status to valid
	if err := r.UpdateConfigStatus(ctx, &pricing, "Valid", "Pricing configuration is valid"); err != nil {
		logger.Error(err, "Failed to update Pricing status to valid")
		result := shared.FinishReconciliation("Valid", err, 30*time.Second)
		return result.Result, result.Error
	}

	logger.Info("Pricing reconciliation completed successfully", "name", pricing.Name)
	result := shared.FinishReconciliation("Valid", nil, 5*time.Minute)
	return result.Result, result.Error
}

// validatePricing performs validation of Pricing configuration
func (r *Reconciler) validatePricing(ctx context.Context, pricing *freqtradev1alpha1.Pricing) error {
	logger := log.FromContext(ctx)

	// Validate price_side
	if pricing.Spec.PriceSide != "" {
		validPriceSides := []string{"bid", "ask", "same", "other"}
		isValid := false
		for _, valid := range validPriceSides {
			if pricing.Spec.PriceSide == valid {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("price_side must be one of: %v (current: %s)", validPriceSides, pricing.Spec.PriceSide)
		}
	}

	// Validate order_book_top
	if pricing.Spec.OrderBookTop != nil {
		if *pricing.Spec.OrderBookTop <= 0 {
			return fmt.Errorf("order_book_top must be positive (current: %d)", *pricing.Spec.OrderBookTop)
		}
		if *pricing.Spec.OrderBookTop > 20 {
			return fmt.Errorf("order_book_top should not exceed 20 for performance reasons (current: %d)", *pricing.Spec.OrderBookTop)
		}
	}

	// Validate check_depth_of_market settings
	if pricing.Spec.CheckDepthOfMarket != nil && pricing.Spec.CheckDepthOfMarket.Enabled {
		if pricing.Spec.CheckDepthOfMarket.BidsToAskDelta < 0 {
			return fmt.Errorf("bids_to_ask_delta must be non-negative (current: %f)", pricing.Spec.CheckDepthOfMarket.BidsToAskDelta)
		}
		if pricing.Spec.CheckDepthOfMarket.BidsToAskDelta > 1 {
			return fmt.Errorf("bids_to_ask_delta should not exceed 1 (current: %f)", pricing.Spec.CheckDepthOfMarket.BidsToAskDelta)
		}
	}

	// Validate logical consistency
	if pricing.Spec.UseOrderBook && pricing.Spec.OrderBookTop == nil {
		return fmt.Errorf("order_book_top must be specified when use_order_book is true")
	}

	logger.Info("Pricing validation passed", "name", pricing.Name)
	return nil
}
