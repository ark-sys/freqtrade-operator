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

	// BotConfig
	bot := tradeBot.Spec.Bot
	cfg["bot_name"] = bot.BotName
	if bot.TradingMode != "" {
		cfg["trading_mode"] = bot.TradingMode
	}
	cfg["dry_run"] = bot.DryRun
	if bot.DryRunWallet != nil {
		cfg["dry_run_wallet"] = *bot.DryRunWallet
	}
	if bot.StakeCurrency != "" {
		cfg["stake_currency"] = bot.StakeCurrency
	}
	if bot.StakeAmount != "" {
		cfg["stake_amount"] = bot.StakeAmount
	}
	if bot.MaxOpenTrades != nil {
		cfg["max_open_trades"] = *bot.MaxOpenTrades
	}
	if bot.FiatDisplayCurrency != "" {
		cfg["fiat_display_currency"] = bot.FiatDisplayCurrency
	}
	if bot.DBUrl != "" {
		cfg["db_url"] = bot.DBUrl
	}
	if bot.Export != "" {
		cfg["export"] = bot.Export
	}
	if bot.DisableParamExport {
		cfg["disable_param_export"] = bot.DisableParamExport
	}
	if bot.DisableDataframeChecks {
		cfg["disable_dataframe_checks"] = bot.DisableDataframeChecks
	}

	// DataConfig
	if tradeBot.Spec.Data != nil {
		data := tradeBot.Spec.Data
		if data.DataformatOHLCV != "" {
			cfg["dataformat_ohlcv"] = data.DataformatOHLCV
		}
		if data.DataformatTrades != "" {
			cfg["dataformat_trades"] = data.DataformatTrades
		}
		if data.PositionAdjustment != "" {
			cfg["position_adjustment"] = data.PositionAdjustment
		}
		if data.NewPairsDaysAgo != nil {
			cfg["new_pairs_days_ago"] = data.NewPairsDaysAgo
		}
		if data.DownloadTrades {
			cfg["download_trades"] = data.DownloadTrades
		}
		if data.MaxEntryPositionAdjustment != nil {
			cfg["max_entry_position_adjustment"] = *data.MaxEntryPositionAdjustment
		}
		if data.AvailableCapital != nil {
			cfg["available_capital"] = *data.AvailableCapital
		}
		if data.AmendLastStakeAmount {
			cfg["amend_last_stake_amount"] = data.AmendLastStakeAmount
		}
		if data.LastStakeAmountMinRatio != nil {
			cfg["last_stake_amount_min_ratio"] = data.LastStakeAmountMinRatio
		}
		if data.ProcessOnlyNewCandles {
			cfg["process_only_new_candles"] = data.ProcessOnlyNewCandles
		}
		if data.AmountReservePercent != nil {
			cfg["amount_reserve_percent"] = *data.AmountReservePercent
		}
		if data.ReduceDfFootprint {
			cfg["reduce_df_footprint"] = data.ReduceDfFootprint
		}
		if data.CustomPriceMaxDistanceRatio != nil {
			cfg["custom_price_max_distance_ratio"] = data.CustomPriceMaxDistanceRatio
		}
	}

	// AdvancedConfig
	if tradeBot.Spec.Advanced != nil {
		adv := tradeBot.Spec.Advanced
		if adv.TradableBalanceRatio != nil {
			cfg["tradable_balance_ratio"] = *adv.TradableBalanceRatio
		}
		if adv.CancelOpenOrdersOnExit {
			cfg["cancel_open_orders_on_exit"] = adv.CancelOpenOrdersOnExit
		}
		if adv.MarginMode != "" {
			cfg["margin_mode"] = adv.MarginMode
		}
		if adv.InitialState != "" {
			cfg["initial_state"] = adv.InitialState
		}
		if adv.ForceEntryEnable {
			cfg["force_entry_enable"] = adv.ForceEntryEnable
		}
	}

	// UnfilledTimeoutConfig
	if tradeBot.Spec.Timeout != nil {
		timeoutConfig := map[string]interface{}{}
		if tradeBot.Spec.Timeout.Entry != nil {
			timeoutConfig["entry"] = *tradeBot.Spec.Timeout.Entry
		}
		if tradeBot.Spec.Timeout.Exit != nil {
			timeoutConfig["exit"] = *tradeBot.Spec.Timeout.Exit
		}
		if tradeBot.Spec.Timeout.ExitTimeoutCount != nil {
			timeoutConfig["exit_timeout_count"] = tradeBot.Spec.Timeout.ExitTimeoutCount
		}
		if tradeBot.Spec.Timeout.Unit != "" {
			timeoutConfig["unit"] = tradeBot.Spec.Timeout.Unit
		}
		if len(timeoutConfig) > 0 {
			cfg["unfilledtimeout"] = timeoutConfig
		}
	}

	// InternalsConfig
	if tradeBot.Spec.Internals != nil {
		internalsConfig := map[string]interface{}{}
		if tradeBot.Spec.Internals.ProcessThrottleSecs != nil {
			internalsConfig["process_throttle_secs"] = *tradeBot.Spec.Internals.ProcessThrottleSecs
		}
		if tradeBot.Spec.Internals.Interval != nil {
			internalsConfig["interval"] = tradeBot.Spec.Internals.Interval
		}
		if tradeBot.Spec.Internals.SdNotify {
			internalsConfig["sd_notify"] = tradeBot.Spec.Internals.SdNotify
		}
		if len(internalsConfig) > 0 {
			cfg["internals"] = internalsConfig
		}
	}

	// API Server
	jwtSecretKey := existingJWTKey
	if tradeBot.Spec.APIServer != nil && tradeBot.Spec.APIServer.Enabled {
		if jwtSecretKey == "" {
			var err error
			jwtSecretKey, err = GenerateJWTSecretKey()
			if err != nil {
				jwtSecretKey = "1ewq2r3t4y5u6i7o8p9asdfghjklzxcvbnm1234567890"
			}
		}
		apiServer := map[string]interface{}{
			"enabled":        true,
			"jwt_secret_key": jwtSecretKey,
		}
		if tradeBot.Spec.APIServer.ListenIP != "" {
			apiServer["listen_ip_address"] = tradeBot.Spec.APIServer.ListenIP
		}
		if tradeBot.Spec.APIServer.ListenPort != nil {
			apiServer["listen_port"] = *tradeBot.Spec.APIServer.ListenPort
		}
		if tradeBot.Spec.APIServer.Verbosity != "" {
			apiServer["verbosity"] = tradeBot.Spec.APIServer.Verbosity
		}
		if tradeBot.Spec.APIServer.EnableOpenAPI {
			apiServer["enable_openapi"] = tradeBot.Spec.APIServer.EnableOpenAPI
		}
		if tradeBot.Spec.APIServer.Username != "" {
			apiServer["username"] = tradeBot.Spec.APIServer.Username
		}
		if tradeBot.Spec.APIServer.Password != "" {
			apiServer["password"] = tradeBot.Spec.APIServer.Password
		}
		if len(tradeBot.Spec.APIServer.CORSOrigins) > 0 {
			apiServer["CORS_origins"] = tradeBot.Spec.APIServer.CORSOrigins
		} else {
			apiServer["CORS_origins"] = []string{}
		}
		cfg["api_server"] = apiServer
	}

	// ExperimentalConfig
	if tradeBot.Spec.Experimental != nil && tradeBot.Spec.Experimental.BlockBadExchanges {
		cfg["block_bad_exchanges"] = tradeBot.Spec.Experimental.BlockBadExchanges
	}

	// LoggingConfig
	if tradeBot.Spec.Logging != nil && tradeBot.Spec.Logging.Version != nil {
		cfg["logging"] = map[string]interface{}{
			"version": *tradeBot.Spec.Logging.Version,
		}
	}

	return cfg, jwtSecretKey, nil
}
