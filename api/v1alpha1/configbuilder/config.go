package configbuilder

import (
	"encoding/json"
	"fmt"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// AssembleConfig merges all configuration sections into a complete Freqtrade config
func AssembleConfig(
	tradeBot *v1alpha1.TradeBot,
	exchange *v1alpha1.Exchange,
	pairWhitelist *v1alpha1.PairList,
	pairBlacklist *v1alpha1.PairList,
	entryPricing *v1alpha1.EntryPricing,
	exitPricing *v1alpha1.ExitPricing,
	orderTypes *v1alpha1.OrderTypes,
	riskManagement *v1alpha1.RiskManagement,
	notification *v1alpha1.Notification,
	strategy *v1alpha1.Strategy,
) (map[string]string, error) {
	// Create the main config map
	config := make(map[string]interface{})

	// Add bot-level configuration
	botConfig := BuildTradeBotConfig(tradeBot)
	for k, v := range botConfig {
		config[k] = v
	}

	// Add exchange configuration
	exchangeConfig := BuildExchangeConfig(exchange, pairWhitelist, pairBlacklist)
	if exchangeConfig != nil {
		config["exchange"] = exchangeConfig
	}

	// Add entry pricing configuration
	entryPricingConfig := BuildEntryPricingConfig(entryPricing)
	if entryPricingConfig != nil {
		config["entry_pricing"] = entryPricingConfig
	}

	// Add exit pricing configuration
	exitPricingConfig := BuildExitPricingConfig(exitPricing)
	if exitPricingConfig != nil {
		config["exit_pricing"] = exitPricingConfig
	}

	// Add order types configuration
	orderTypesConfig := BuildOrderTypesConfig(orderTypes)
	if orderTypesConfig != nil {
		config["order_types"] = orderTypesConfig
	}

	// Add risk management configuration
	riskConfig := BuildRiskManagementConfig(riskManagement)
	if riskConfig != nil {
		for k, v := range riskConfig {
			config[k] = v
		}
	}

	// Add notification configuration
	notificationConfig := BuildNotificationConfig(notification)
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
		return nil, fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	return map[string]string{"config.json": string(configBytes)}, nil
}
