package riskmanagement

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
)

// Reconciler reconciles a RiskManagement object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	shared.StatusUpdater
}

// Reconcile handles the reconciliation loop for RiskManagement resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Starting RiskManagement reconciliation", "namespacedName", req.NamespacedName)

	// Fetch RiskManagement resource
	var riskMgmt freqtradev1alpha1.RiskManagement
	if err := r.Get(ctx, req.NamespacedName, &riskMgmt); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("RiskManagement resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get RiskManagement resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if empty
	if riskMgmt.Status.Phase == "" {
		if err := r.UpdateConfigStatus(ctx, &riskMgmt, "Validating", "Starting validation"); err != nil {
			logger.Error(err, "Failed to initialize RiskManagement status")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	// Validate RiskManagement configuration
	if err := r.validateRiskManagement(ctx, &riskMgmt); err != nil {
		logger.Error(err, "RiskManagement validation failed")
		if updateErr := r.UpdateConfigStatus(ctx, &riskMgmt, "Invalid", err.Error()); updateErr != nil {
			logger.Error(updateErr, "Failed to update RiskManagement status after validation failure")
		}
		result := shared.FinishReconciliation("Invalid", err, 30*time.Second)
		return result.Result, result.Error
	}

	// Update status to valid
	if err := r.UpdateConfigStatus(ctx, &riskMgmt, "Valid", "RiskManagement configuration is valid"); err != nil {
		logger.Error(err, "Failed to update RiskManagement status to valid")
		result := shared.FinishReconciliation("Valid", err, 30*time.Second)
		return result.Result, result.Error
	}

	logger.Info("RiskManagement reconciliation completed successfully", "name", riskMgmt.Name)
	result := shared.FinishReconciliation("Valid", nil, 5*time.Minute)
	return result.Result, result.Error
}

// validateRiskManagement performs validation of RiskManagement configuration
func (r *Reconciler) validateRiskManagement(ctx context.Context, riskMgmt *freqtradev1alpha1.RiskManagement) error {
	logger := log.FromContext(ctx)

	// Validate stoploss value
	if riskMgmt.Spec.Stoploss != nil {
		if *riskMgmt.Spec.Stoploss >= 0 {
			return fmt.Errorf("stoploss must be negative (current: %f)", *riskMgmt.Spec.Stoploss)
		}
		if *riskMgmt.Spec.Stoploss < -1 {
			return fmt.Errorf("stoploss cannot be less than -1 (current: %f)", *riskMgmt.Spec.Stoploss)
		}
	}

	// Validate trailing stop values
	if riskMgmt.Spec.TrailingStopPositive != nil {
		if *riskMgmt.Spec.TrailingStopPositive <= 0 {
			return fmt.Errorf("trailing_stop_positive must be positive (current: %f)", *riskMgmt.Spec.TrailingStopPositive)
		}
	}

	if riskMgmt.Spec.TrailingStopPositiveOffset != nil {
		if *riskMgmt.Spec.TrailingStopPositiveOffset <= 0 {
			return fmt.Errorf("trailing_stop_positive_offset must be positive (current: %f)", *riskMgmt.Spec.TrailingStopPositiveOffset)
		}
	}

	// Validate trailing stop consistency
	if riskMgmt.Spec.TrailingStopPositive != nil && riskMgmt.Spec.TrailingStopPositiveOffset != nil {
		if *riskMgmt.Spec.TrailingStopPositiveOffset >= *riskMgmt.Spec.TrailingStopPositive {
			return fmt.Errorf("trailing_stop_positive_offset (%f) must be less than trailing_stop_positive (%f)",
				*riskMgmt.Spec.TrailingStopPositiveOffset, *riskMgmt.Spec.TrailingStopPositive)
		}
	}

	// Validate fee
	if riskMgmt.Spec.Fee != nil {
		if *riskMgmt.Spec.Fee < 0 {
			return fmt.Errorf("fee must be non-negative (current: %f)", *riskMgmt.Spec.Fee)
		}
		if *riskMgmt.Spec.Fee > 1 {
			return fmt.Errorf("fee must be less than or equal to 1 (current: %f)", *riskMgmt.Spec.Fee)
		}
	}

	// Validate minimal ROI
	if riskMgmt.Spec.MinimalROI != nil {
		if len(riskMgmt.Spec.MinimalROI) == 0 {
			return fmt.Errorf("minimal_roi must not be empty")
		}

		for keystr, value := range riskMgmt.Spec.MinimalROI {
			key, err := strconv.Atoi(keystr)
			if err != nil {
				return fmt.Errorf("minimal_roi keys must be integers (current: %s)", keystr)
			}

			if key < 0 {
				return fmt.Errorf("minimal_roi keys must be 0 or greater (current: %d)", key)
			}
			if *value < -1.0 || *value > 1.0 {
				return fmt.Errorf("minimal_roi values must be between -1 and 1 (current: %f)", *value)
			}
		}

	}

	// Validate trade amounts
	if riskMgmt.Spec.MinimumTradeAmount != nil && *riskMgmt.Spec.MinimumTradeAmount <= 0 {
		return fmt.Errorf("minimum_trade_amount must be positive (current: %d)", *riskMgmt.Spec.MinimumTradeAmount)
	}

	if riskMgmt.Spec.TargetedTradeAmount != nil && *riskMgmt.Spec.TargetedTradeAmount <= 0 {
		return fmt.Errorf("targeted_trade_amount must be positive (current: %d)", *riskMgmt.Spec.TargetedTradeAmount)
	}

	// Validate consistency between minimum and targeted trade amounts
	if riskMgmt.Spec.MinimumTradeAmount != nil && riskMgmt.Spec.TargetedTradeAmount != nil {
		if *riskMgmt.Spec.MinimumTradeAmount > *riskMgmt.Spec.TargetedTradeAmount {
			return fmt.Errorf("minimum_trade_amount (%d) cannot be greater than targeted_trade_amount (%d)",
				*riskMgmt.Spec.MinimumTradeAmount, *riskMgmt.Spec.TargetedTradeAmount)
		}
	}

	// Validate liquidation buffer
	if riskMgmt.Spec.LiquidationBuffer != nil {
		if *riskMgmt.Spec.LiquidationBuffer < 0 {
			return fmt.Errorf("liquidation_buffer must be non-negative (current: %f)", *riskMgmt.Spec.LiquidationBuffer)
		}
	}

	logger.Info("RiskManagement validation passed", "name", riskMgmt.Name)
	return nil
}
