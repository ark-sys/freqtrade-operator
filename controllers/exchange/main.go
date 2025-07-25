package exchange

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

// Reconciler reconciles an Exchange object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	shared.StatusUpdater
}

// Reconcile handles the reconciliation loop for Exchange resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Starting Exchange reconciliation", "namespacedName", req.NamespacedName)

	// Fetch Exchange resource
	var exchange freqtradev1alpha1.Exchange
	if err := r.Get(ctx, req.NamespacedName, &exchange); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Exchange resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Exchange resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if empty
	if exchange.Status.Phase == "" {
		if err := r.UpdateConfigStatus(ctx, &exchange, "Validating", "Starting validation"); err != nil {
			logger.Error(err, "Failed to initialize Exchange status")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	// Validate Exchange configuration
	if err := r.validateExchange(ctx, &exchange); err != nil {
		logger.Error(err, "Exchange validation failed")
		if updateErr := r.UpdateConfigStatus(ctx, &exchange, "Invalid", err.Error()); updateErr != nil {
			logger.Error(updateErr, "Failed to update Exchange status after validation failure")
		}
		result := shared.FinishReconciliation("Invalid", err, 30*time.Second)
		return result.Result, result.Error
	}

	// Update status to valid
	if err := r.UpdateConfigStatus(ctx, &exchange, "Valid", "Exchange configuration is valid"); err != nil {
		logger.Error(err, "Failed to update Exchange status to valid")
		result := shared.FinishReconciliation("Valid", err, 30*time.Second)
		return result.Result, result.Error
	}

	logger.Info("Exchange reconciliation completed successfully", "name", exchange.Name)
	result := shared.FinishReconciliation("Valid", nil, 5*time.Minute)
	return result.Result, result.Error
}

// validateExchange performs validation of Exchange configuration
func (r *Reconciler) validateExchange(ctx context.Context, exchange *freqtradev1alpha1.Exchange) error {
	logger := log.FromContext(ctx)

	// Validate required fields
	if exchange.Spec.Name == "" {
		return fmt.Errorf("exchange name is required")
	}

	// Validate that either direct credentials or secretRef is provided
	hasDirectCreds := exchange.Spec.Key != "" || exchange.Spec.Secret != ""
	hasSecretRef := exchange.Spec.SecretRef != ""

	if !hasDirectCreds && !hasSecretRef {
		return fmt.Errorf("either direct credentials (key/secret) or secretRef must be provided")
	}

	// If secretRef is provided, validate that the secret exists
	if hasSecretRef {
		if err := r.validateSecretRef(ctx, exchange); err != nil {
			return fmt.Errorf("secretRef validation failed: %w", err)
		}
	}

	// Validate PairList references if provided
	if exchange.Spec.WhitelistRef != "" {
		if err := r.validatePairListRef(ctx, exchange, exchange.Spec.WhitelistRef, "whitelist"); err != nil {
			return fmt.Errorf("whitelist validation failed: %w", err)
		}
	}

	if exchange.Spec.BlacklistRef != "" {
		if err := r.validatePairListRef(ctx, exchange, exchange.Spec.BlacklistRef, "blacklist"); err != nil {
			return fmt.Errorf("blacklist validation failed: %w", err)
		}
	}

	// Validate numeric fields
	if exchange.Spec.OutdatedOffset != nil && *exchange.Spec.OutdatedOffset < 0 {
		return fmt.Errorf("outdated_offset must be non-negative")
	}

	if exchange.Spec.MarketRefreshInterval != nil && *exchange.Spec.MarketRefreshInterval < 0 {
		return fmt.Errorf("market_refresh_interval must be non-negative")
	}

	logger.Info("Exchange validation passed", "name", exchange.Name)
	return nil
}

// validateSecretRef validates that the referenced secret exists and has required keys
func (r *Reconciler) validateSecretRef(ctx context.Context, exchange *freqtradev1alpha1.Exchange) error {
	// For now, we just check if the secretRef is not empty
	// In a full implementation, we would fetch the secret and validate its contents
	//if exchange.Spec.SecretRef == "" {
	//	return fmt.Errorf("secretRef cannot be empty")
	//}

	// TODO: Add actual secret validation when needed
	// var secret corev1.Secret
	// if err := r.Get(ctx, types.NamespacedName{Name: exchange.Spec.SecretRef, Namespace: exchange.Namespace}, &secret); err != nil {
	//     return fmt.Errorf("failed to get secret %s: %w", exchange.Spec.SecretRef, err)
	// }

	return nil
}

// validatePairListRef validates that the referenced PairList exists
func (r *Reconciler) validatePairListRef(ctx context.Context, exchange *freqtradev1alpha1.Exchange, pairListName, pairListType string) error {
	var pairList freqtradev1alpha1.PairList
	if err := r.Get(ctx, client.ObjectKey{Name: pairListName, Namespace: exchange.Namespace}, &pairList); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("%s PairList '%s' not found", pairListType, pairListName)
		}
		return fmt.Errorf("failed to get %s PairList '%s': %w", pairListType, pairListName, err)
	}

	// Check if the PairList itself is valid
	if pairList.Status.Phase == "Invalid" {
		return fmt.Errorf("%s PairList '%s' is in invalid state: %s", pairListType, pairListName, pairList.Status.Message)
	}

	return nil
}
