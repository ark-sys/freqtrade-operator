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
	strategy       *freqtradev1alpha1.Strategy
	tradebotconfig *freqtradev1alpha1.TradeBotConfig
}

// fetchReferencedResources retrieves all resources referenced by the TradeBot
func (r *Reconciler) fetchReferencedResources(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	namespace string) (*referencedResources, error) {

	logger := log.FromContext(ctx)
	result := &referencedResources{}

	// Fetch TradeBotConfig
	tradeBotConfig := &freqtradev1alpha1.TradeBotConfig{}
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.Config, Namespace: namespace}, tradeBotConfig); err != nil {
		logger.Error(err, "Failed to fetch TradeBotConfig")
		return nil, err
	}
	result.tradebotconfig = tradeBotConfig

	// Fetch Strategy
	strategy := &freqtradev1alpha1.Strategy{}
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.Strategy, Namespace: namespace}, strategy); err != nil {
		logger.Error(err, "Failed to fetch Strategy")
		return nil, err
	}
	result.strategy = strategy

	return result, nil
}

// reconcileResources creates or updates all resources needed by the TradeBot
func (r *Reconciler) reconcileResources(
	ctx context.Context,
	tradeBot *freqtradev1alpha1.TradeBot,
	configData map[string]string) error {

	logger := log.FromContext(ctx)
	logger.V(2).Info("Reconciling resources for TradeBot", "name", tradeBot.Name)

	// 1. Create a config.json Secret with the TradeBotConfig data
	configSecret := resources.BuildSecret(*tradeBot, configData)
	if err := resources.ApplySecret(ctx, r.Client, &configSecret); err != nil {
		logger.Error(err, "Failed to apply Secret")
		return fmt.Errorf("failed to apply Secret: %w", err)
	}
	logger.V(2).Info("Secret applied successfully", "name", configSecret.Name)

	// 2. Create a ConfigMap for the Strategy script
	strategy := &freqtradev1alpha1.Strategy{}
	if err := r.Get(ctx, types.NamespacedName{Name: tradeBot.Spec.Strategy, Namespace: tradeBot.Namespace}, strategy); err != nil {
		logger.Error(err, "Failed to fetch Strategy for ConfigMap creation")
		return fmt.Errorf("failed to fetch Strategy for ConfigMap creation: %w", err)
	}

	strategyConfigMap := resources.BuildStrategyConfigMap(*tradeBot, *strategy)
	if err := resources.ApplyConfigMap(ctx, r.Client, &strategyConfigMap); err != nil {
		logger.Error(err, "Failed to apply Strategy ConfigMap")
		return fmt.Errorf("failed to apply Strategy ConfigMap: %w", err)
	}
	logger.V(2).Info("Strategy ConfigMap applied successfully", "name", strategyConfigMap.Name)

	// 3. Create or update PVC for user_data
	pvc := resources.BuildUserDataPVC(*tradeBot)
	if err := resources.ApplyPVC(ctx, r.Client, &pvc); err != nil {
		logger.Error(err, "Failed to apply PVC")
		return fmt.Errorf("failed to apply PVC: %w", err)
	}
	logger.V(2).Info("PVC applied successfully", "name", pvc.Name)

	// 4. Create or update StatefulSet
	sts := resources.BuildStatefulSet(ctx, r.Client, *tradeBot, configSecret.Name, strategyConfigMap.Name, pvc.Name)
	if err := resources.ApplyStatefulSet(ctx, r.Client, &sts); err != nil {
		logger.Error(err, "Failed to apply StatefulSet")
		return fmt.Errorf("failed to apply StatefulSet: %w", err)
	}
	logger.V(2).Info("StatefulSet applied successfully", "name", sts.Name)

	// 5. Create or update Service
	svc := resources.BuildService(*tradeBot)
	if err := resources.ApplyService(ctx, r.Client, &svc); err != nil {
		logger.Error(err, "Failed to apply Service")
		return fmt.Errorf("failed to apply Service: %w", err)
	}
	logger.V(2).Info("Service applied successfully", "name", svc.Name)

	return nil
}
