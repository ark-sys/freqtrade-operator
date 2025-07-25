package tradebot

import (
	"context"
	"fmt"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/tradebot/resources"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// referencedResources holds all the resources referenced by a TradeBot
type referencedResources struct {
	exchange        *freqtradev1alpha1.Exchange
	pairWhitelist   *freqtradev1alpha1.PairList
	pairBlacklist   *freqtradev1alpha1.PairList
	entryPricing    *freqtradev1alpha1.Pricing
	exitPricing     *freqtradev1alpha1.Pricing
	order           *freqtradev1alpha1.Order
	riskManagement  *freqtradev1alpha1.RiskManagement
	notification    *freqtradev1alpha1.Notification
	strategy        *freqtradev1alpha1.Strategy
	pairlistMethods *freqtradev1alpha1.PairlistMethods
}

// fetchReferencedResources retrieves all resources referenced by the TradeBot
func (r *Reconciler) fetchReferencedResources(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	namespace string) (*referencedResources, error) {

	logger := log.FromContext(ctx)
	result := &referencedResources{}

	// Fetch Exchange
	exchange := &freqtradev1alpha1.Exchange{}
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.References.ExchangeRef, Namespace: namespace}, exchange); err != nil {
		logger.Error(err, "Failed to fetch Exchange")
		return nil, err
	}
	result.exchange = exchange

	// Fetch PairLists referenced by Exchange
	if exchange.Spec.WhitelistRef != "" {
		wl := &freqtradev1alpha1.PairList{}
		if err := r.Get(ctx, types.NamespacedName{Name: exchange.Spec.WhitelistRef, Namespace: namespace}, wl); err != nil {
			logger.Error(err, "Failed to fetch PairWhitelist")
			return nil, err
		}
		result.pairWhitelist = wl
	}

	if exchange.Spec.BlacklistRef != "" {
		bl := &freqtradev1alpha1.PairList{}
		if err := r.Get(ctx, types.NamespacedName{Name: exchange.Spec.BlacklistRef, Namespace: namespace}, bl); err != nil {
			logger.Error(err, "Failed to fetch PairBlacklist")
			return nil, err
		}
		result.pairBlacklist = bl
	}

	// Fetch EntryPricing
	if tradeBot.Spec.References.EntryPricingRef != "" {
		ep := &freqtradev1alpha1.Pricing{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.References.EntryPricingRef, Namespace: namespace}, ep); err != nil {
			logger.Error(err, "Failed to fetch EntryPricing")
			return nil, err
		}
		result.entryPricing = ep
	}

	// Fetch ExitPricing
	if tradeBot.Spec.References.ExitPricingRef != "" {
		ep := &freqtradev1alpha1.Pricing{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.References.ExitPricingRef, Namespace: namespace}, ep); err != nil {
			logger.Error(err, "Failed to fetch ExitPricing")
			return nil, err
		}
		result.exitPricing = ep
	}

	// Fetch OrderTypes
	if tradeBot.Spec.References.OrderTypesRef != "" {
		ot := &freqtradev1alpha1.Order{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.References.OrderTypesRef, Namespace: namespace}, ot); err != nil {
			logger.Error(err, "Failed to fetch OrderTypes")
			return nil, err
		}
		result.order = ot
	}

	// Fetch RiskManagement
	if tradeBot.Spec.References.RiskManagementRef != "" {
		rm := &freqtradev1alpha1.RiskManagement{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.References.RiskManagementRef, Namespace: namespace}, rm); err != nil {
			logger.Error(err, "Failed to fetch RiskManagement")
			return nil, err
		}
		result.riskManagement = rm
	}

	// Fetch Notification
	if tradeBot.Spec.References.NotificationRef != "" {
		n := &freqtradev1alpha1.Notification{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.References.NotificationRef, Namespace: namespace}, n); err != nil {
			logger.Error(err, "Failed to fetch Notification")
			return nil, err
		}
		result.notification = n
	}

	// Fetch Strategy
	strategy := &freqtradev1alpha1.Strategy{}
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.References.StrategyRef, Namespace: namespace}, strategy); err != nil {
		logger.Error(err, "Failed to fetch Strategy")
		return nil, err
	}
	result.strategy = strategy

	// Fetch PairlistMethods
	if tradeBot.Spec.References.PairlistMethodsRef != "" {
		pm := &freqtradev1alpha1.PairlistMethods{}
		if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.References.PairlistMethodsRef, Namespace: namespace}, pm); err != nil {
			logger.Error(err, "Failed to fetch PairlistMethods")
			return nil, err
		}
		result.pairlistMethods = pm
	}

	return result, nil
}

// reconcileResources creates or updates all resources needed by the TradeBot
func (r *Reconciler) reconcileResources(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	configData map[string]string) error {

	logger := log.FromContext(ctx)
	logger.Info("Reconciling resources for TradeBot", "name", tradeBot.Name)

	// 1. Create or update ConfigMap for config.json
	configMap := resources.BuildConfigMap(*tradeBot, configData)
	if err := resources.ApplyConfigMap(ctx, r.Client, &configMap); err != nil {
		logger.Error(err, "Failed to apply ConfigMap")
		return fmt.Errorf("failed to apply ConfigMap: %w", err)
	}
	logger.Info("ConfigMap applied successfully", "name", configMap.Name)

	// 2. Create or update ConfigMap for strategy
	strategy := &freqtradev1alpha1.Strategy{}
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.References.StrategyRef, Namespace: tradeBot.Namespace}, strategy); err != nil {
		logger.Error(err, "Failed to fetch Strategy for ConfigMap creation")
		return fmt.Errorf("failed to fetch Strategy for ConfigMap creation: %w", err)
	}

	strategyConfigMap := resources.BuildStrategyConfigMap(*tradeBot, *strategy)
	if err := resources.ApplyConfigMap(ctx, r.Client, &strategyConfigMap); err != nil {
		logger.Error(err, "Failed to apply Strategy ConfigMap")
		return fmt.Errorf("failed to apply Strategy ConfigMap: %w", err)
	}
	logger.Info("Strategy ConfigMap applied successfully", "name", strategyConfigMap.Name)

	// 3. Create or update PVC for user_data
	pvc := resources.BuildUserDataPVC(*tradeBot)
	if err := resources.ApplyPVC(ctx, r.Client, &pvc); err != nil {
		logger.Error(err, "Failed to apply PVC")
		return fmt.Errorf("failed to apply PVC: %w", err)
	}
	logger.Info("PVC applied successfully", "name", pvc.Name)

	// 4. Create or update StatefulSet
	sts := resources.BuildStatefulSet(ctx, r.Client, *tradeBot, configMap.Name, strategyConfigMap.Name, pvc.Name)
	if err := resources.ApplyStatefulSet(ctx, r.Client, &sts); err != nil {
		logger.Error(err, "Failed to apply StatefulSet")
		return fmt.Errorf("failed to apply StatefulSet: %w", err)
	}
	logger.Info("StatefulSet applied successfully", "name", sts.Name)

	// 5. Create or update Service
	svc := resources.BuildService(*tradeBot)
	if err := resources.ApplyService(ctx, r.Client, &svc); err != nil {
		logger.Error(err, "Failed to apply Service")
		return fmt.Errorf("failed to apply Service: %w", err)
	}
	logger.Info("Service applied successfully", "name", svc.Name)

	return nil
}
