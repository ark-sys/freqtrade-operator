package configbuilder

import (
	"encoding/json"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildExchangeConfig builds the "exchange" section for Freqtrade config.json
func BuildExchangeConfig(
	exchange *v1alpha1.Exchange,
	pairWhitelist *v1alpha1.PairList,
	pairBlacklist *v1alpha1.PairList,
) map[string]interface{} {
	if exchange == nil {
		return nil
	}
	cfg := map[string]interface{}{
		"name":   exchange.Spec.Name,
		"key":    exchange.Spec.ApiKey,
		"secret": exchange.Spec.Secret,
	}

	// Handle CCXT config
	if exchange.Spec.CcxtConfig.Raw != nil {
		var ccxtConfig interface{}
		// We're not handling the error here as the CRD validation should ensure this is valid JSON
		_ = json.Unmarshal(exchange.Spec.CcxtConfig.Raw, &ccxtConfig)
		cfg["ccxt_config"] = ccxtConfig
	}

	// Handle CCXT async config
	if exchange.Spec.CcxtAsyncConfig.Raw != nil {
		var ccxtAsyncConfig interface{}
		// We're not handling the error here as the CRD validation should ensure this is valid JSON
		_ = json.Unmarshal(exchange.Spec.CcxtAsyncConfig.Raw, &ccxtAsyncConfig)
		cfg["ccxt_async_config"] = ccxtAsyncConfig
	}

	// Handle pair lists
	if pairWhitelist != nil {
		cfg["pair_whitelist"] = pairWhitelist.Spec.Pairs
	}
	if pairBlacklist != nil {
		cfg["pair_blacklist"] = pairBlacklist.Spec.Pairs
	}

	return cfg
}
