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

// BuildConfig merges all configuration sections into a complete Freqtrade config
func BuildConfig(
	ctx context.Context,
	k8sClient client.Client,
	tradeBot *v1alpha1.TradeBot,
	tradeBotConfig *v1alpha1.TradeBotConfig,
	extraCorsHosts []string,
) (map[string]string, error) {
	// Create the main config map
	config := make(map[string]interface{})

	// Retrieve apiCredentials if any
	apiCredentials, err := GetSecretData(ctx, k8sClient, tradeBot.Namespace, tradeBotConfig.Spec.APIServer.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get API credentials: %w", err)
	}

	tradeBotName := tradeBot.Name

	// Add bot-level configuration
	botConfig, err := BuildTradeBotConfig(tradeBotName, tradeBotConfig, apiCredentials, extraCorsHosts)
	if err != nil {
		return nil, fmt.Errorf("failed to build TradeBot config: %w", err)
	}

	for k, v := range botConfig {
		config[k] = v
	}

	// Add exchange configuration
	exchangeSecretData, err := GetSecretData(ctx, k8sClient, tradeBot.Namespace, tradeBotConfig.Spec.Exchange.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange secret data: %w", err)

	} else {
		fmt.Println("Exchange secret data retrieved successfully")

	}

	exchangeConfig, err := BuildExchangeConfig(tradeBotConfig.Spec.Exchange, exchangeSecretData)
	if err != nil {
		return nil, fmt.Errorf("failed to build Exchange config: %w", err)
	}

	if exchangeConfig != nil {
		config["exchange"] = exchangeConfig
	}

	// Add pairlists configuration if specified
	pairlistsConfig := BuildPairlistMethodsConfig(tradeBotConfig.Spec.PairlistMethod)
	if pairlistsConfig != nil {
		for k, v := range pairlistsConfig {
			config[k] = v
		}
	}

	// Add entry pricing configuration
	entryPricingConfig := BuildPricingConfig(tradeBotConfig.Spec.EntryPricing)
	if entryPricingConfig != nil {
		config["entry_pricing"] = entryPricingConfig
	}

	// Add exit pricing configuration
	exitPricingConfig := BuildPricingConfig(tradeBotConfig.Spec.ExitPricing)
	if exitPricingConfig != nil {
		config["exit_pricing"] = exitPricingConfig
	}

	// Add order types configuration
	orderTypesConfig := BuildOrderTypesConfig(tradeBotConfig.Spec.Order)
	if orderTypesConfig != nil {
		config["order_types"] = orderTypesConfig
	}

	// Add order time in force configuration
	orderTimeInForceConfig := BuildOrderTimeInForceConfig(tradeBotConfig.Spec.Order)
	if orderTimeInForceConfig != nil {
		config["order_time_in_force"] = orderTimeInForceConfig
	}

	// Add orderflow configuration
	orderflowConfig := BuildOrderflowConfig(tradeBotConfig.Spec.Order)
	if orderflowConfig != nil {
		config["orderflow"] = orderflowConfig
	}

	// Add risk management configuration
	riskConfig := BuildRiskManagementConfig(tradeBotConfig.Spec.RiskManagement)
	if riskConfig != nil {
		for k, v := range riskConfig {
			config[k] = v
		}
	}

	// Add notification configuration
	notificationSecretData, err := GetSecretData(ctx, k8sClient, tradeBot.Namespace, tradeBotConfig.Spec.Notification.Telegram.SecretRef)
	notificationConfig, err := BuildNotificationConfig(tradeBotConfig.Spec.Notification, notificationSecretData)
	if err != nil {
		return nil, fmt.Errorf("failed to build Notification config: %w", err)
	}

	if notificationConfig != nil {
		for k, v := range notificationConfig {
			config[k] = v
		}
	}

	// Marshal to JSON
	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	return map[string]string{"config.json": string(configBytes)}, nil
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
