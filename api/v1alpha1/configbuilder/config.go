package configbuilder

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AssembleConfig merges all configuration sections into a complete Freqtrade config
func AssembleConfig(
	ctx context.Context,
	k8sClient client.Client,
	tradeBot *v1alpha1.TradeBot,
	exchange *v1alpha1.Exchange,
	entryPricing *v1alpha1.Pricing,
	exitPricing *v1alpha1.Pricing,
	orderTypes *v1alpha1.Order,
	riskManagement *v1alpha1.RiskManagement,
	notification *v1alpha1.Notification,
	strategy *v1alpha1.Strategy,
	pairlistMethods *v1alpha1.PairlistMethods,
	existingJWTKey string,
) (map[string]string, string, error) {
	// Create the main config map
	config := make(map[string]interface{})

	// Retrieve apiCredentials if any
	apiCredentials, err := GetSecretData(ctx, k8sClient, tradeBot.Namespace, tradeBot.Spec.APIServer.SecretRef)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get API credentials: %w", err)
	}

	// Add bot-level configuration
	botConfig, jwtSecretKey, err := BuildTradeBotConfig(tradeBot, existingJWTKey, apiCredentials)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build TradeBot config: %w", err)
	}

	for k, v := range botConfig {
		config[k] = v
	}

	// Add exchange configuration
	exchangeConfig, err := BuildExchangeConfig(ctx, k8sClient, exchange)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build Exchange config: %w", err)
	}

	if exchangeConfig != nil {
		config["exchange"] = exchangeConfig
	}

	// Add pairlists configuration if specified
	if pairlistMethods != nil {
		pairlistsConfig := BuildPairlistMethodsConfig(pairlistMethods)
		if pairlistsConfig != nil {
			for k, v := range pairlistsConfig {
				config[k] = v
			}
		}
	}

	// Add entry pricing configuration
	entryPricingConfig := BuildPricingConfig(entryPricing)
	if entryPricingConfig != nil {
		config["entry_pricing"] = entryPricingConfig
	}

	// Add exit pricing configuration
	exitPricingConfig := BuildPricingConfig(exitPricing)
	if exitPricingConfig != nil {
		config["exit_pricing"] = exitPricingConfig
	}

	// Add order types configuration
	orderTypesConfig := BuildOrderTypesConfig(orderTypes)
	if orderTypesConfig != nil {
		config["order_types"] = orderTypesConfig
	}

	// Add order time in force configuration
	orderTimeInForceConfig := BuildOrderTimeInForceConfig(orderTypes)
	if orderTimeInForceConfig != nil {
		config["order_time_in_force"] = orderTimeInForceConfig
	}

	// Add orderflow configuration
	orderflowConfig := BuildOrderflowConfig(orderTypes)
	if orderflowConfig != nil {
		config["orderflow"] = orderflowConfig
	}

	// Add risk management configuration
	riskConfig := BuildRiskManagementConfig(riskManagement)
	if riskConfig != nil {
		for k, v := range riskConfig {
			config[k] = v
		}
	}

	// Add notification configuration
	notificationConfig, err := BuildNotificationConfig(ctx, k8sClient, notification)
	if err != nil {
		return nil, "", fmt.Errorf("failed to build Notification config: %w", err)
	}

	if notificationConfig != nil {
		for k, v := range notificationConfig {
			config[k] = v
		}
	}

	// Add strategy configuration
	strategyConfig := BuildStrategyConfig(strategy)
	if strategyConfig != nil {
		for k, v := range strategyConfig {
			config[k] = v
		}
	}

	// Marshal to JSON
	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	return map[string]string{"config.json": string(configBytes)}, jwtSecretKey, nil
}

// GetSecretData retrieves data from a Kubernetes Secret
func GetSecretData(ctx context.Context, k8sClient client.Client, namespace, name string) (map[string][]byte, error) {
	if name == "" {
		return nil, nil
	}

	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
	}

	return secret.Data, nil
}
