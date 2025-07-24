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

) (map[string]interface{}, error) {
	if exchange == nil {
		return nil, nil
	}

	cfg := map[string]interface{}{}
	// Set basic exchange properties
	if exchange.Spec.Name != "" {
		cfg["name"] = exchange.Spec.Name
	}
	if exchange.Spec.Key != "" {
		cfg["key"] = exchange.Spec.Key
	}
	if exchange.Spec.Secret != "" {
		cfg["secret"] = exchange.Spec.Secret
	}
	if exchange.Spec.Password != "" {
		cfg["password"] = exchange.Spec.Password
	}
	if exchange.Spec.UID != "" {
		cfg["uid"] = exchange.Spec.UID
	}
	if exchange.Spec.AccountID != "" {
		cfg["account_id"] = exchange.Spec.AccountID
	}
	if exchange.Spec.WalletAddress != "" {
		cfg["wallet_address"] = exchange.Spec.WalletAddress
	}
	if exchange.Spec.PrivateKey != "" {
		cfg["private_key"] = exchange.Spec.PrivateKey
	}
	if exchange.Spec.LogResponses {
		cfg["log_responses"] = exchange.Spec.LogResponses
	}
	if exchange.Spec.EnableWS {
		cfg["enable_ws"] = exchange.Spec.EnableWS
	}
	if exchange.Spec.UnkownFeeRate {
		cfg["unkown_fee_rate"] = exchange.Spec.UnkownFeeRate
	}
	if exchange.Spec.OutdatedOffset > 0 {
		cfg["outdated_offset"] = exchange.Spec.OutdatedOffset
	}
	if exchange.Spec.MarketRefreshInterval > 0 {
		cfg["market_refresh_interval"] = exchange.Spec.MarketRefreshInterval
	}

	// Get API credentials from Secret if SecretRef is set
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
		if password, ok := secretData["password"]; ok {
			cfg["password"] = string(password)
		}
		if uid, ok := secretData["uid"]; ok {
			cfg["uid"] = string(uid)
		}
		if accountID, ok := secretData["account_id"]; ok {
			cfg["account_id"] = string(accountID)
		}
		if walletAddress, ok := secretData["wallet_address"]; ok {
			cfg["wallet_address"] = string(walletAddress)
		}
		if privateKey, ok := secretData["private_key"]; ok {
			cfg["private_key"] = string(privateKey)
		}
	}

	// Handle CCXT config
	if exchange.Spec.CcxtConfig.Raw != nil {
		var ccxtConfig interface{}
		_ = json.Unmarshal(exchange.Spec.CcxtConfig.Raw, &ccxtConfig)
		cfg["ccxt_config"] = ccxtConfig
	}

	if exchange.Spec.CcxtAsyncConfig.Raw != nil {
		var ccxtAsyncConfig interface{}
		_ = json.Unmarshal(exchange.Spec.CcxtAsyncConfig.Raw, &ccxtAsyncConfig)
		cfg["ccxt_async_config"] = ccxtAsyncConfig
	}

	if exchange.Spec.CcxtSyncConfig.Raw != nil {
		var ccxtSyncConfig interface{}
		_ = json.Unmarshal(exchange.Spec.CcxtSyncConfig.Raw, &ccxtSyncConfig)
		cfg["ccxt_sync_config"] = ccxtSyncConfig
	}

	if exchange.Spec.WhitelistRef != "" {
		cfg["pair_whitelist"] = exchange.Spec.WhitelistRef
	}
	if exchange.Spec.BlacklistRef != "" {
		cfg["pair_blacklist"] = exchange.Spec.BlacklistRef
	}

	return cfg, nil
}
