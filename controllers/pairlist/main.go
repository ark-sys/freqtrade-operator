package pairlist

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
)

// Reconciler reconciles a PairList object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	shared.StatusUpdater
}

// Reconcile handles the reconciliation loop for PairList resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Starting PairList reconciliation", "namespacedName", req.NamespacedName)

	// Fetch PairList resource
	var pairList freqtradev1alpha1.PairList
	if err := r.Get(ctx, req.NamespacedName, &pairList); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PairList resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get PairList resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if empty
	if pairList.Status.Phase == "" {
		if err := r.UpdateConfigStatus(ctx, &pairList, "Validating", "Starting validation"); err != nil {
			logger.Error(err, "Failed to initialize PairList status")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	// Validate PairList configuration
	if err := r.validatePairList(ctx, &pairList); err != nil {
		logger.Error(err, "PairList validation failed")
		if updateErr := r.UpdateConfigStatus(ctx, &pairList, "Invalid", err.Error()); updateErr != nil {
			logger.Error(updateErr, "Failed to update PairList status after validation failure")
		}
		result := shared.FinishReconciliation("Invalid", err, 30*time.Second)
		return result.Result, result.Error
	}

	// Update status to valid
	if err := r.UpdateConfigStatus(ctx, &pairList, "Valid", "PairList configuration is valid"); err != nil {
		logger.Error(err, "Failed to update PairList status to valid")
		result := shared.FinishReconciliation("Valid", err, 30*time.Second)
		return result.Result, result.Error
	}

	logger.Info("PairList reconciliation completed successfully", "name", pairList.Name)
	result := shared.FinishReconciliation("Valid", nil, 5*time.Minute)
	return result.Result, result.Error
}

// validatePairList performs validation of PairList configuration
func (r *Reconciler) validatePairList(ctx context.Context, pairList *freqtradev1alpha1.PairList) error {
	logger := log.FromContext(ctx)

	// Validate that pairs list is not empty
	if len(pairList.Spec.Pairs) == 0 {
		return fmt.Errorf("pairs list cannot be empty")
	}

	// Validate each pair format
	for i, pair := range pairList.Spec.Pairs {
		if err := r.validatePairFormat(pair); err != nil {
			return fmt.Errorf("invalid pair at index %d: %w", i, err)
		}
	}

	// Check for duplicate pairs
	if err := r.checkDuplicatePairs(pairList.Spec.Pairs); err != nil {
		return fmt.Errorf("duplicate pairs found: %w", err)
	}

	logger.Info("PairList validation passed", "name", pairList.Name, "pairCount", len(pairList.Spec.Pairs))
	return nil
}

// validatePairFormat validates the format of a trading pair
func (r *Reconciler) validatePairFormat(pair string) error {
	// Basic validation: pair should contain a separator (/ or _)
	if !strings.Contains(pair, "/") && !strings.Contains(pair, "_") {
		return fmt.Errorf("pair '%s' must contain a separator (/ or _)", pair)
	}

	// Split by common separators
	var parts []string
	if strings.Contains(pair, "/") {
		parts = strings.Split(pair, "/")
	} else {
		parts = strings.Split(pair, "_")
	}

	if len(parts) != 2 {
		return fmt.Errorf("pair '%s' must have exactly two parts separated by / or _", pair)
	}

	// Validate that both parts are not empty and contain only valid characters
	for i, part := range parts {
		if part == "" {
			return fmt.Errorf("pair '%s' has empty part at position %d", pair, i)
		}

		// Check for valid characters (alphanumeric)
		for _, char := range part {
			if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')) {
				return fmt.Errorf("pair '%s' contains invalid character '%c' in part '%s'", pair, char, part)
			}
		}
	}

	return nil
}

// checkDuplicatePairs checks for duplicate pairs in the list
func (r *Reconciler) checkDuplicatePairs(pairs []string) error {
	seen := make(map[string]bool)
	var duplicates []string

	for _, pair := range pairs {
		// Normalize pair format for comparison (convert to uppercase and use / separator)
		normalized := r.normalizePair(pair)
		if seen[normalized] {
			duplicates = append(duplicates, pair)
		} else {
			seen[normalized] = true
		}
	}

	if len(duplicates) > 0 {
		return fmt.Errorf("duplicate pairs found: %v", duplicates)
	}

	return nil
}

// normalizePair normalizes a pair format for comparison
func (r *Reconciler) normalizePair(pair string) string {
	// Convert to uppercase and use / as separator
	normalized := strings.ToUpper(pair)
	normalized = strings.ReplaceAll(normalized, "_", "/")
	return normalized
}
