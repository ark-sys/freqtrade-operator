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
func BuildTradeBotConfig(tradeBot *v1alpha1.TradeBot, existingJWTKey string) (map[string]interface{}, string, error) {
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

	// Add advanced trading configuration from TradeBotSpec
	if tradeBot.Spec.TradableBalanceRatio != nil {
		cfg["tradable_balance_ratio"] = *tradeBot.Spec.TradableBalanceRatio
	}

	if tradeBot.Spec.CancelOpenOrdersOnExit != nil {
		cfg["cancel_open_orders_on_exit"] = *tradeBot.Spec.CancelOpenOrdersOnExit
	}

	if tradeBot.Spec.InitialState != "" {
		cfg["initial_state"] = tradeBot.Spec.InitialState
	}

	if tradeBot.Spec.ForceEntryEnable != nil {
		cfg["force_entry_enable"] = *tradeBot.Spec.ForceEntryEnable
	}

	// Add margin_mode for futures trading
	if tradeBot.Spec.MarginMode != "" {
		cfg["margin_mode"] = tradeBot.Spec.MarginMode
	}

	// Add unfilledtimeout configuration from TradeBotSpec
	if tradeBot.Spec.UnfilledTimeout != nil {
		timeoutConfig := map[string]interface{}{}

		if tradeBot.Spec.UnfilledTimeout.Entry > 0 {
			timeoutConfig["entry"] = tradeBot.Spec.UnfilledTimeout.Entry
		}

		if tradeBot.Spec.UnfilledTimeout.Exit > 0 {
			timeoutConfig["exit"] = tradeBot.Spec.UnfilledTimeout.Exit
		}

		if tradeBot.Spec.UnfilledTimeout.ExitTimeoutCount > 0 {
			timeoutConfig["exit_timeout_count"] = tradeBot.Spec.UnfilledTimeout.ExitTimeoutCount
		}

		if tradeBot.Spec.UnfilledTimeout.Unit != "" {
			timeoutConfig["unit"] = tradeBot.Spec.UnfilledTimeout.Unit
		}

		if len(timeoutConfig) > 0 {
			cfg["unfilledtimeout"] = timeoutConfig
		}
	}

	// Add edge configuration from TradeBotSpec
	if tradeBot.Spec.Edge != nil {
		edgeConfig := map[string]interface{}{}

		if tradeBot.Spec.Edge.Enabled != nil {
			edgeConfig["enabled"] = *tradeBot.Spec.Edge.Enabled
		}

		if tradeBot.Spec.Edge.ProcessThrottleSecs > 0 {
			edgeConfig["process_throttle_secs"] = tradeBot.Spec.Edge.ProcessThrottleSecs
		}

		if tradeBot.Spec.Edge.CalculateSinceNumberOfDays > 0 {
			edgeConfig["calculate_since_number_of_days"] = tradeBot.Spec.Edge.CalculateSinceNumberOfDays
		}

		if tradeBot.Spec.Edge.AllowedRisk > 0 {
			edgeConfig["allowed_risk"] = tradeBot.Spec.Edge.AllowedRisk
		}

		if tradeBot.Spec.Edge.StoplossRangeMin != 0 {
			edgeConfig["stoploss_range_min"] = tradeBot.Spec.Edge.StoplossRangeMin
		}

		if tradeBot.Spec.Edge.StoplossRangeMax != 0 {
			edgeConfig["stoploss_range_max"] = tradeBot.Spec.Edge.StoplossRangeMax
		}

		if tradeBot.Spec.Edge.StoplossRangeStep != 0 {
			edgeConfig["stoploss_range_step"] = tradeBot.Spec.Edge.StoplossRangeStep
		}

		if tradeBot.Spec.Edge.MinimumWinrate > 0 {
			edgeConfig["minimum_winrate"] = tradeBot.Spec.Edge.MinimumWinrate
		}

		if tradeBot.Spec.Edge.MinimumExpectancy > 0 {
			edgeConfig["minimum_expectancy"] = tradeBot.Spec.Edge.MinimumExpectancy
		}

		if tradeBot.Spec.Edge.MinTradeNumber > 0 {
			edgeConfig["min_trade_number"] = tradeBot.Spec.Edge.MinTradeNumber
		}

		if tradeBot.Spec.Edge.MaxTradeDurationMinute > 0 {
			edgeConfig["max_trade_duration_minute"] = tradeBot.Spec.Edge.MaxTradeDurationMinute
		}

		if tradeBot.Spec.Edge.RemovePumps != nil {
			edgeConfig["remove_pumps"] = *tradeBot.Spec.Edge.RemovePumps
		}

		if len(edgeConfig) > 0 {
			cfg["edge"] = edgeConfig
		}
	}

	// Add internals configuration from TradeBotSpec
	if tradeBot.Spec.Internals != nil {
		internalsConfig := map[string]interface{}{}

		if tradeBot.Spec.Internals.ProcessThrottleSecs > 0 {
			internalsConfig["process_throttle_secs"] = tradeBot.Spec.Internals.ProcessThrottleSecs
		}

		if len(internalsConfig) > 0 {
			cfg["internals"] = internalsConfig
		}
	}

	// Build API server configuration if API is enabled
	jwtSecretKey := existingJWTKey
	if tradeBot.Spec.APIEnabled {
		// Use existing JWT key or generate a new one if none exists
		if jwtSecretKey == "" {
			var err error
			jwtSecretKey, err = GenerateJWTSecretKey()
			if err != nil {
				// If there's an error, use a fallback
				jwtSecretKey = "1ewq2r3t4y5u6i7o8p9asdfghjklzxcvbnm1234567890"
			}
		}

		// Build api_server section with only configured values
		apiServer := map[string]interface{}{
			"enabled":        true,
			"jwt_secret_key": jwtSecretKey,
		}

		// Set listen IP address only if specified
		if tradeBot.Spec.APIServer != nil && tradeBot.Spec.APIServer.ListenIP != "" {
			apiServer["listen_ip_address"] = tradeBot.Spec.APIServer.ListenIP
		}

		// Set listen port only if specified
		if tradeBot.Spec.APIServer != nil && tradeBot.Spec.APIServer.ListenPort > 0 {
			apiServer["listen_port"] = tradeBot.Spec.APIServer.ListenPort
		}

		// Set verbosity only if specified
		if tradeBot.Spec.APIServer != nil && tradeBot.Spec.APIServer.Verbosity != "" {
			apiServer["verbosity"] = tradeBot.Spec.APIServer.Verbosity
		}

		// Set enable_openapi only if specified
		if tradeBot.Spec.APIServer != nil && tradeBot.Spec.APIServer.EnableOpenAPI != nil {
			apiServer["enable_openapi"] = *tradeBot.Spec.APIServer.EnableOpenAPI
		}

		// Set username only if specified
		if tradeBot.Spec.APIServer != nil && tradeBot.Spec.APIServer.Username != "" {
			apiServer["username"] = tradeBot.Spec.APIServer.Username
		}

		// Set password only if specified
		if tradeBot.Spec.APIServer != nil && tradeBot.Spec.APIServer.Password != "" {
			apiServer["password"] = tradeBot.Spec.APIServer.Password
		}

		// Set CORS origins
		if len(tradeBot.Spec.CORSOrigins) > 0 {
			apiServer["CORS_origins"] = tradeBot.Spec.CORSOrigins
		} else {
			apiServer["CORS_origins"] = []string{}
		}

		cfg["api_server"] = apiServer
	}

	return cfg, jwtSecretKey, nil
}
