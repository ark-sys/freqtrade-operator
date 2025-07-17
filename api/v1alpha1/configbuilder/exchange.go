package configbuilder

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildExchangeConfig builds the "exchange" section for Freqtrade config.json
func BuildExchangeConfig(
	ctx context.Context,
	k8sClient client.Client,
	exchange *v1alpha1.Exchange,
	pairWhitelist *v1alpha1.PairList,
	pairBlacklist *v1alpha1.PairList,
) (map[string]interface{}, error) {
	if exchange == nil {
		return nil, nil
	}

	cfg := map[string]interface{}{
		"name": exchange.Spec.Name,
	}

	// Get API credentials from Secret
	if exchange.Spec.SecretRef != "" {
		secretData, err := GetSecretData(ctx, k8sClient, exchange.Namespace, exchange.Spec.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get exchange secret: %w", err)
		}

		if apiKey, ok := secretData["api-key"]; ok {
			cfg["key"] = string(apiKey)
		}

		if secret, ok := secretData["secret"]; ok {
			cfg["secret"] = string(secret)
		}
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

	return cfg, nil
}
