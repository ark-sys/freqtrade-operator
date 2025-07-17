package configbuilder

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// GenerateJWTSecretKey generates a random JWT secret key
func GenerateJWTSecretKey() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// BuildTradeBotConfig builds the base bot configuration for Freqtrade config.json
func BuildTradeBotConfig(tradeBot *v1alpha1.TradeBot) (map[string]interface{}, string, error) {
	if tradeBot == nil {
		return nil, "", nil
	}

	cfg := map[string]interface{}{}

	// Set basic bot configuration
	cfg["bot_name"] = tradeBot.Spec.BotName

	if tradeBot.Spec.TradingMode != "" {
		cfg["trading_mode"] = tradeBot.Spec.TradingMode
	}

	cfg["dry_run"] = tradeBot.Spec.DryRun

	if tradeBot.Spec.DryRunWallet > 0 {
		cfg["dry_run_wallet"] = tradeBot.Spec.DryRunWallet
	}

	if tradeBot.Spec.StakeCurrency != "" {
		cfg["stake_currency"] = tradeBot.Spec.StakeCurrency
	}

	if tradeBot.Spec.StakeAmount != "" {
		cfg["stake_amount"] = tradeBot.Spec.StakeAmount
	}

	if tradeBot.Spec.MaxOpenTrades > 0 {
		cfg["max_open_trades"] = tradeBot.Spec.MaxOpenTrades
	}

	if tradeBot.Spec.FiatDisplayCurrency != "" {
		cfg["fiat_display_currency"] = tradeBot.Spec.FiatDisplayCurrency
	}

	// Add API configuration if API is enabled
	jwtSecretKey := ""
	if tradeBot.Spec.APIEnabled {
		// Generate a new JWT secret key
		var err error
		jwtSecretKey, err = GenerateJWTSecretKey()
		if err != nil {
			// If there's an error, use a fallback
			jwtSecretKey = "s0m3_s3cr3t_k3y"
		}
		cfg["jwt_secret_key"] = jwtSecretKey

		// Set CORS origins
		if len(tradeBot.Spec.CORSOrigins) > 0 {
			cfg["CORS_origins"] = tradeBot.Spec.CORSOrigins
		}
	}

	return cfg, jwtSecretKey, nil
}
