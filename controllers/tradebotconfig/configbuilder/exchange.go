package configbuilder

import (
	"encoding/json"
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildExchangeConfig builds the "exchange" section for Freqtrade config.json
func BuildExchangeConfig(
	exchange *v1alpha1.ExchangeSpec,
	secretData map[string][]byte,
) (map[string]interface{}, error) {
	if exchange == nil {
		return nil, nil
	}

	cfg := map[string]interface{}{}

	if exchange.Name != "" {
		cfg["name"] = exchange.Name
	}

	// 1. Deprecated plaintext credential fields (P3-1) are read first, as a
	// fallback only - the webhook rejects setting these at all unless
	// freqtrade.io/allow-plaintext-credentials is set, so the normal path
	// never reaches this block with anything to read. account_id isn't
	// treated as a credential (it's an identifier, not a secret), so it has
	// no webhook-enforced deprecation and no secretData equivalent below.
	if exchange.Key != "" {
		cfg["key"] = exchange.Key
	}
	if exchange.Secret != "" {
		cfg["secret"] = exchange.Secret
	}
	if exchange.Password != "" {
		cfg["password"] = exchange.Password
	}
	if exchange.UID != "" {
		cfg["uid"] = exchange.UID
	}
	if exchange.AccountID != "" {
		cfg["account_id"] = exchange.AccountID
	}
	if exchange.WalletAddress != "" {
		cfg["wallet_address"] = exchange.WalletAddress
	}
	if exchange.PrivateKey != "" {
		cfg["private_key"] = exchange.PrivateKey
	}

	// 2. secretRef always wins over a plaintext value for the keys it
	// supplies - do not error if the Secret is missing a given key.
	if exchange.SecretRef != "" && secretData != nil {
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

	if exchange.LogResponses != nil {
		cfg["log_responses"] = *exchange.LogResponses
	}
	if exchange.EnableWS != nil {
		cfg["enable_ws"] = *exchange.EnableWS
	}
	if exchange.UnkownFeeRate != nil {
		cfg["unknown_fee_rate"] = *exchange.UnkownFeeRate
	}
	if exchange.OutdatedOffset != nil {
		cfg["outdated_offset"] = *exchange.OutdatedOffset
	}
	if exchange.MarketRefreshInterval != nil {
		cfg["market_refresh_interval"] = *exchange.MarketRefreshInterval
	}

	// Handle CCXT config
	if exchange.CcxtConfig.Raw != nil {
		var ccxtConfig interface{}
		_ = json.Unmarshal(exchange.CcxtConfig.Raw, &ccxtConfig)
		cfg["ccxt_config"] = ccxtConfig
	}
	if exchange.CcxtAsyncConfig.Raw != nil {
		var ccxtAsyncConfig interface{}
		_ = json.Unmarshal(exchange.CcxtAsyncConfig.Raw, &ccxtAsyncConfig)
		cfg["ccxt_async_config"] = ccxtAsyncConfig
	}
	if exchange.CcxtSyncConfig.Raw != nil {
		var ccxtSyncConfig interface{}
		_ = json.Unmarshal(exchange.CcxtSyncConfig.Raw, &ccxtSyncConfig)
		cfg["ccxt_sync_config"] = ccxtSyncConfig
	}

	// Handle whitelist and blacklist
	if exchange.Whitelist != nil && exchange.Whitelist.Pairs != nil {
		cfg["pair_whitelist"] = exchange.Whitelist.Pairs
	}
	if exchange.Blacklist != nil && exchange.Blacklist.Pairs != nil {
		cfg["pair_blacklist"] = exchange.Blacklist.Pairs
	}

	return cfg, nil
}
