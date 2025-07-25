package pairlistmethods

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

// Reconciler reconciles a PairlistMethods object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	shared.StatusUpdater
}

// Reconcile handles the reconciliation loop for PairlistMethods resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Starting PairlistMethods reconciliation", "namespacedName", req.NamespacedName)

	// Fetch PairlistMethods resource
	var pairlistMethods freqtradev1alpha1.PairlistMethods
	if err := r.Get(ctx, req.NamespacedName, &pairlistMethods); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("PairlistMethods resource not found, ignoring since it must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get PairlistMethods resource")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Initialize status if empty
	if pairlistMethods.Status.Phase == "" {
		if err := r.UpdateConfigStatus(ctx, &pairlistMethods, "Validating", "Starting validation"); err != nil {
			logger.Error(err, "Failed to initialize PairlistMethods status")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
	}

	// Validate PairlistMethods configuration
	if err := r.validatePairlistMethods(ctx, &pairlistMethods); err != nil {
		logger.Error(err, "PairlistMethods validation failed")
		if updateErr := r.UpdateConfigStatus(ctx, &pairlistMethods, "Invalid", err.Error()); updateErr != nil {
			logger.Error(updateErr, "Failed to update PairlistMethods status after validation failure")
		}
		result := shared.FinishReconciliation("Invalid", err, 30*time.Second)
		return result.Result, result.Error
	}

	// Update status to valid
	if err := r.UpdateConfigStatus(ctx, &pairlistMethods, "Valid", "PairlistMethods configuration is valid"); err != nil {
		logger.Error(err, "Failed to update PairlistMethods status to valid")
		result := shared.FinishReconciliation("Valid", err, 30*time.Second)
		return result.Result, result.Error
	}

	logger.Info("PairlistMethods reconciliation completed successfully", "name", pairlistMethods.Name)
	result := shared.FinishReconciliation("Valid", nil, 5*time.Minute)
	return result.Result, result.Error
}

// validatePairlistMethods performs validation of PairlistMethods configuration
func (r *Reconciler) validatePairlistMethods(ctx context.Context, pairlistMethods *freqtradev1alpha1.PairlistMethods) error {
	logger := log.FromContext(ctx)

	// Validate that methods list is not empty
	if len(pairlistMethods.Spec.Methods) == 0 {
		return fmt.Errorf("methods list cannot be empty")
	}

	// Validate each method configuration
	for i, method := range pairlistMethods.Spec.Methods {
		if err := r.validatePairlistMethod(&method, i); err != nil {
			return fmt.Errorf("invalid method at index %d: %w", i, err)
		}
	}

	// Validate method sequence logic
	if err := r.validateMethodSequence(pairlistMethods.Spec.Methods); err != nil {
		return fmt.Errorf("invalid method sequence: %w", err)
	}

	logger.Info("PairlistMethods validation passed", "name", pairlistMethods.Name, "methodCount", len(pairlistMethods.Spec.Methods))
	return nil
}

// validatePairlistMethod validates a single pairlist method configuration
func (r *Reconciler) validatePairlistMethod(method *freqtradev1alpha1.PairlistConfig, index int) error {
	// Validate method name
	validMethods := []freqtradev1alpha1.PairlistMethod{
		freqtradev1alpha1.StaticPairList,
		freqtradev1alpha1.VolumePairList,
		freqtradev1alpha1.PercentChangePairList,
		freqtradev1alpha1.ProducerPairList,
		freqtradev1alpha1.RemotePairList,
		freqtradev1alpha1.MarketCapPairList,
		freqtradev1alpha1.AgeFilter,
		freqtradev1alpha1.FullTradesFilter,
		freqtradev1alpha1.OffsetFilter,
		freqtradev1alpha1.PerformanceFilter,
		freqtradev1alpha1.PrecisionFilter,
		freqtradev1alpha1.PriceFilter,
		freqtradev1alpha1.ShuffleFilter,
		freqtradev1alpha1.SpreadFilter,
		freqtradev1alpha1.RangeStabilityFilter,
		freqtradev1alpha1.VolatilityFilter,
	}

	isValidMethod := false
	for _, valid := range validMethods {
		if method.Method == valid {
			isValidMethod = true
			break
		}
	}

	if !isValidMethod {
		return fmt.Errorf("unknown method: %s", method.Method)
	}

	// Validate method-specific configurations
	switch method.Method {
	case freqtradev1alpha1.VolumePairList:
		return r.validateVolumePairList(method)
	case freqtradev1alpha1.PercentChangePairList:
		return r.validatePercentChangePairList(method)
	case freqtradev1alpha1.ProducerPairList:
		return r.validateProducerPairList(method)
	case freqtradev1alpha1.RemotePairList:
		return r.validateRemotePairList(method)
	case freqtradev1alpha1.MarketCapPairList:
		return r.validateMarketCapPairList(method)
	case freqtradev1alpha1.AgeFilter:
		return r.validateAgeFilter(method)
	case freqtradev1alpha1.PriceFilter:
		return r.validatePriceFilter(method)
	case freqtradev1alpha1.SpreadFilter:
		return r.validateSpreadFilter(method)
	case freqtradev1alpha1.RangeStabilityFilter:
		return r.validateRangeStabilityFilter(method)
	case freqtradev1alpha1.VolatilityFilter:
		return r.validateVolatilityFilter(method)
	case freqtradev1alpha1.OffsetFilter:
		return r.validateOffsetFilter(method)
	}

	// Validate common fields
	if method.NumberAssets != nil && *method.NumberAssets <= 0 {
		return fmt.Errorf("number_assets must be positive")
	}

	if method.RefreshPeriod != nil && *method.RefreshPeriod < 0 {
		return fmt.Errorf("refresh_period must be non-negative")
	}

	return nil
}

// validateMethodSequence validates the logical sequence of methods
func (r *Reconciler) validateMethodSequence(methods []freqtradev1alpha1.PairlistConfig) error {
	if len(methods) == 0 {
		return fmt.Errorf("methods list cannot be empty")
	}

	// First method should typically be a pair source (StaticPairList, VolumePairList, etc.)
	firstMethod := methods[0].Method
	validFirstMethods := []freqtradev1alpha1.PairlistMethod{
		freqtradev1alpha1.StaticPairList,
		freqtradev1alpha1.VolumePairList,
		freqtradev1alpha1.PercentChangePairList,
		freqtradev1alpha1.RemotePairList,
		freqtradev1alpha1.MarketCapPairList,
	}

	isValidFirst := false
	for _, valid := range validFirstMethods {
		if firstMethod == valid {
			isValidFirst = true
			break
		}
	}

	if !isValidFirst {
		return fmt.Errorf("first method should be a pair source, got: %s", firstMethod)
	}

	return nil
}

// Method-specific validation functions
func (r *Reconciler) validateVolumePairList(method *freqtradev1alpha1.PairlistConfig) error {
	if method.SortKey != "" && method.SortKey != "quoteVolume" {
		return fmt.Errorf("sort_key for VolumePairList must be 'quoteVolume' if specified")
	}
	return nil
}

func (r *Reconciler) validatePercentChangePairList(method *freqtradev1alpha1.PairlistConfig) error {
	if method.LookbackPeriod != nil && *method.LookbackPeriod <= 0 {
		return fmt.Errorf("lookback_period must be positive")
	}
	if method.LookbackDays != nil && *method.LookbackDays <= 0 {
		return fmt.Errorf("lookback_days must be positive")
	}
	return nil
}

func (r *Reconciler) validateProducerPairList(method *freqtradev1alpha1.PairlistConfig) error {
	if method.ProducerName == "" {
		return fmt.Errorf("producer_name is required for ProducerPairList")
	}
	return nil
}

func (r *Reconciler) validateRemotePairList(method *freqtradev1alpha1.PairlistConfig) error {
	if method.PairlistURL == "" {
		return fmt.Errorf("pairlist_url is required for RemotePairList")
	}
	return nil
}

func (r *Reconciler) validateMarketCapPairList(method *freqtradev1alpha1.PairlistConfig) error {
	if method.MaxRank != nil && *method.MaxRank <= 0 {
		return fmt.Errorf("max_rank must be positive")
	}
	return nil
}

func (r *Reconciler) validateAgeFilter(method *freqtradev1alpha1.PairlistConfig) error {
	if method.MinDaysListed != nil && *method.MinDaysListed < 0 {
		return fmt.Errorf("min_days_listed must be non-negative")
	}
	if method.MaxDaysListed != nil && *method.MaxDaysListed < 0 {
		return fmt.Errorf("max_days_listed must be non-negative")
	}
	if method.MinDaysListed != nil && method.MaxDaysListed != nil && *method.MinDaysListed > *method.MaxDaysListed {
		return fmt.Errorf("min_days_listed cannot be greater than max_days_listed")
	}
	return nil
}

func (r *Reconciler) validatePriceFilter(method *freqtradev1alpha1.PairlistConfig) error {
	if method.MinPrice != nil && *method.MinPrice < 0 {
		return fmt.Errorf("min_price must be non-negative")
	}
	if method.MaxPrice != nil && *method.MaxPrice < 0 {
		return fmt.Errorf("max_price must be non-negative")
	}
	if method.MinPrice != nil && method.MaxPrice != nil && *method.MinPrice > *method.MaxPrice {
		return fmt.Errorf("min_price cannot be greater than max_price")
	}
	return nil
}

func (r *Reconciler) validateSpreadFilter(method *freqtradev1alpha1.PairlistConfig) error {
	if method.MaxSpreadRatio != nil && (*method.MaxSpreadRatio < 0 || *method.MaxSpreadRatio > 1) {
		return fmt.Errorf("max_spread_ratio must be between 0 and 1")
	}
	return nil
}

func (r *Reconciler) validateRangeStabilityFilter(method *freqtradev1alpha1.PairlistConfig) error {
	if method.LookbackDaysRange != nil && *method.LookbackDaysRange <= 0 {
		return fmt.Errorf("lookback_days_range must be positive")
	}
	if method.MinRateOfChange != nil && method.MaxRateOfChange != nil && *method.MinRateOfChange > *method.MaxRateOfChange {
		return fmt.Errorf("min_rate_of_change cannot be greater than max_rate_of_change")
	}
	return nil
}

func (r *Reconciler) validateVolatilityFilter(method *freqtradev1alpha1.PairlistConfig) error {
	if method.LookbackDaysVolatility != nil && *method.LookbackDaysVolatility <= 0 {
		return fmt.Errorf("lookback_days_volatility must be positive")
	}
	if method.MinVolatility != nil && method.MaxVolatility != nil && *method.MinVolatility > *method.MaxVolatility {
		return fmt.Errorf("min_volatility cannot be greater than max_volatility")
	}
	return nil
}

func (r *Reconciler) validateOffsetFilter(method *freqtradev1alpha1.PairlistConfig) error {
	if method.Offset != nil && *method.Offset < 0 {
		return fmt.Errorf("offset must be non-negative")
	}
	return nil
}
