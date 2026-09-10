package configbuilder

import (
	"strconv"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildTradeBotConfig builds the base bot configuration for Freqtrade config.json.
// jwtSecretKey is api_server.jwt_secret_key already fully resolved (plaintext
// fallback, secretRef override, and absent-value generation all settled) by
// resolveAPIServerJWTSecretKey - this function only writes it through.
func BuildTradeBotConfig(
	tradeBotName string,
	tradeBotConfig *v1alpha1.TradeBotConfig,
	apiCredentials map[string][]byte,
	jwtSecretKey string,
	extraCorsHosts []string,
) (map[string]interface{}, error) {
	if tradeBotConfig == nil {
		return nil, nil
	}

	cfg := map[string]interface{}{}

	if tradeBotConfig.Spec.Bot != nil {
		for k, v := range buildBotConfigFields(tradeBotConfig.Spec.Bot, tradeBotName) {
			cfg[k] = v
		}
	}

	if tradeBotConfig.Spec.AI != nil {
		cfg["freqai"] = buildFreqAIConfig(tradeBotConfig.Spec.AI)
	}

	if tradeBotConfig.Spec.Data != nil {
		for k, v := range buildDataConfigFields(tradeBotConfig.Spec.Data) {
			cfg[k] = v
		}
	}

	if tradeBotConfig.Spec.Advanced != nil {
		for k, v := range buildAdvancedConfigFields(tradeBotConfig.Spec.Advanced) {
			cfg[k] = v
		}
	}

	if tradeBotConfig.Spec.Timeout != nil {
		if timeoutConfig := buildUnfilledTimeoutConfig(tradeBotConfig.Spec.Timeout); len(timeoutConfig) > 0 {
			cfg["unfilledtimeout"] = timeoutConfig
		}
	}

	if tradeBotConfig.Spec.Internals != nil {
		if internalsConfig := buildInternalsConfigFields(tradeBotConfig.Spec.Internals); len(internalsConfig) > 0 {
			cfg["internals"] = internalsConfig
		}
	}

	if tradeBotConfig.Spec.APIServer != nil {
		cfg["api_server"] = buildAPIServerConfig(tradeBotConfig.Spec.APIServer, apiCredentials, jwtSecretKey, extraCorsHosts)
	}

	// ExperimentalConfig - freqtrade nests this under "experimental", not a
	// top-level key (schema.json: properties.experimental.properties.block_bad_exchanges).
	if tradeBotConfig.Spec.Experimental != nil && tradeBotConfig.Spec.Experimental.BlockBadExchanges != nil {
		cfg["experimental"] = map[string]interface{}{
			"block_bad_exchanges": *tradeBotConfig.Spec.Experimental.BlockBadExchanges,
		}
	}

	// LoggingConfig
	if tradeBotConfig.Spec.Logging != nil && tradeBotConfig.Spec.Logging.Version != nil {
		cfg["logging"] = map[string]interface{}{
			"version": *tradeBotConfig.Spec.Logging.Version,
		}
	}

	return cfg, nil
}

// buildBotConfigFields builds the top-level (non-nested) config keys from
// TradeBotConfigSpec.Bot.
func buildBotConfigFields(bot *v1alpha1.BotConfig, tradeBotName string) map[string]interface{} {
	cfg := map[string]interface{}{}

	if bot.BotName == "" {
		cfg["bot_name"] = tradeBotName
	} else {
		cfg["bot_name"] = bot.BotName
	}
	if bot.TradingMode != "" {
		cfg["trading_mode"] = bot.TradingMode
	}
	if bot.DryRun != nil {
		cfg["dry_run"] = *bot.DryRun
	}
	if bot.DryRunWallet != nil {
		cfg["dry_run_wallet"] = *bot.DryRunWallet
	}
	if bot.StakeCurrency != "" {
		cfg["stake_currency"] = bot.StakeCurrency
	}
	if bot.StakeAmount != "" {
		cfg["stake_amount"] = renderStakeAmount(bot.StakeAmount)
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
	if bot.DisableParamExport != nil {
		cfg["disableparamexport"] = *bot.DisableParamExport
	}
	if bot.DisableDataframeChecks != nil {
		cfg["disable_dataframe_checks"] = *bot.DisableDataframeChecks
	}

	return cfg
}

// renderStakeAmount converts BotConfig.StakeAmount - a Kubernetes API string,
// since the CRD field has to be a plain string type to accept freqtrade's own
// "unlimited" keyword - into whatever JSON type freqtrade's own config schema
// actually requires for the given value. freqtrade's schema types
// stake_amount as `number` or `string`, but constrains the string variant to
// match exactly "unlimited" - any other numeric-looking string (e.g. "100")
// fails freqtrade's own config validation at startup with "does not match
// 'unlimited'", since a quoted JSON string is never a number no matter what
// characters it contains. Verified directly: a real TradeBot pod with
// stake_amount: "100" crash-looped on exactly that error before this fix.
func renderStakeAmount(raw string) interface{} {
	if raw == "unlimited" {
		return raw
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f
	}
	// Not a number and not "unlimited" - pass through as-is; freqtrade's own
	// config validation will reject it with a clear error rather than this
	// function guessing at a fallback that might silently mask a typo.
	return raw
}

// buildFreqAIConfig builds the "freqai" section from TradeBotConfigSpec.AI.
func buildFreqAIConfig(ai *v1alpha1.AIConfig) map[string]interface{} {
	aiConfig := map[string]interface{}{}

	if ai.Enabled != nil {
		aiConfig["enabled"] = *ai.Enabled
	}
	if ai.Identifier != "" {
		aiConfig["identifier"] = ai.Identifier
	}
	if ai.WriteMetricsToDisk != nil {
		aiConfig["write_metrics_to_disk"] = *ai.WriteMetricsToDisk
	}
	if ai.PurgeOldModels != nil {
		aiConfig["purge_old_models"] = *ai.PurgeOldModels
	}
	if ai.ConvWidth != nil {
		aiConfig["conv_width"] = *ai.ConvWidth
	}
	if ai.TrainPeriodDays != nil {
		aiConfig["train_period_days"] = *ai.TrainPeriodDays
	}
	if ai.BacktestPeriodDays != nil {
		aiConfig["backtest_period_days"] = *ai.BacktestPeriodDays
	}
	if ai.LiveRetrainHours != nil {
		aiConfig["live_retrain_hours"] = *ai.LiveRetrainHours
	}
	if ai.ExpirationHours != nil {
		aiConfig["expiration_hours"] = *ai.ExpirationHours
	}
	if ai.SaveBacktestModels != nil {
		aiConfig["save_backtest_models"] = *ai.SaveBacktestModels
	}
	if ai.FitLivePredictionsCandles != nil {
		aiConfig["fit_live_predictions_candles"] = *ai.FitLivePredictionsCandles
	}
	if ai.DataKitchenThreadCount != nil {
		aiConfig["data_kitchen_thread_count"] = *ai.DataKitchenThreadCount
	}
	if ai.ActivateTensorboard != nil {
		aiConfig["activate_tensorboard"] = *ai.ActivateTensorboard
	}
	if ai.WaitForTrainingIterationOnReload != nil {
		aiConfig["wait_for_training_iteration_on_reload"] = *ai.WaitForTrainingIterationOnReload
	}
	if ai.ContinueLearning != nil {
		aiConfig["continual_learning"] = *ai.ContinueLearning
	}
	if ai.Keras != nil {
		aiConfig["keras"] = *ai.Keras
	}

	if ai.FeatureParameters != nil {
		aiConfig["feature_parameters"] = buildFeatureParametersConfig(ai.FeatureParameters)
	}

	if ai.DataSplitParameters != nil {
		aiConfig["data_split_parameters"] = buildDataSplitParametersConfig(ai.DataSplitParameters)
	}

	if ai.ModelTrainingParameters != nil {
		// If ModelTrainingParameters has fields, map them here.
		aiConfig["model_training_parameters"] = ai.ModelTrainingParameters
	}

	if ai.RLConfig != nil {
		aiConfig["rl_config"] = buildRLConfigMap(ai.RLConfig)
	}

	return aiConfig
}

// buildFeatureParametersConfig builds freqai.feature_parameters from
// AIConfig.FeatureParameters.
func buildFeatureParametersConfig(fp *v1alpha1.FeatureParameters) map[string]interface{} {
	fpConfig := map[string]interface{}{}

	if len(fp.IncludeCorrPairlist) > 0 {
		fpConfig["include_corr_pairlist"] = fp.IncludeCorrPairlist
	}
	if len(fp.IncludeTimeframes) > 0 {
		fpConfig["include_timeframes"] = fp.IncludeTimeframes
	}
	if fp.LabelPeriodCandles != nil {
		fpConfig["label_period_candles"] = *fp.LabelPeriodCandles
	}
	if fp.IncludeShiftedCandles != nil {
		fpConfig["include_shifted_candles"] = *fp.IncludeShiftedCandles
	}
	if fp.DIThreshold != nil {
		fpConfig["DI_threshold"] = *fp.DIThreshold
	}
	if fp.WeightFactor != nil {
		fpConfig["weight_factor"] = *fp.WeightFactor
	}
	if fp.PrincipalComponentAnalysis != nil {
		fpConfig["principal_component_analysis"] = *fp.PrincipalComponentAnalysis
	}
	if len(fp.IndicatorPeriodsCandles) > 0 {
		fpConfig["indicator_periods_candles"] = fp.IndicatorPeriodsCandles
	}
	if fp.UseSVMToRemoveOutliers != nil {
		fpConfig["use_SVM_to_remove_outliers"] = *fp.UseSVMToRemoveOutliers
	}
	if fp.PlotFeatureImportances != nil {
		fpConfig["plot_feature_importances"] = *fp.PlotFeatureImportances
	}
	if fp.SVMParams != nil {
		fpConfig["svm_params"] = buildSVMParamsConfig(fp.SVMParams)
	}
	if fp.ShuffleAfterSplit != nil {
		fpConfig["shuffle_after_split"] = *fp.ShuffleAfterSplit
	}
	if fp.BufferTrainDataCandles != nil {
		fpConfig["buffer_train_data_candles"] = *fp.BufferTrainDataCandles
	}

	return fpConfig
}

// buildSVMParamsConfig builds feature_parameters.svm_params from
// FeatureParameters.SVMParams.
func buildSVMParamsConfig(svm *v1alpha1.SVMParams) map[string]interface{} {
	svmConfig := map[string]interface{}{}
	if svm.Shuffle != nil {
		svmConfig["shuffle"] = *svm.Shuffle
	}
	if svm.Nu != nil {
		svmConfig["nu"] = *svm.Nu
	}
	return svmConfig
}

// buildDataSplitParametersConfig builds freqai.data_split_parameters from
// AIConfig.DataSplitParameters.
func buildDataSplitParametersConfig(dsp *v1alpha1.DataSplitParameters) map[string]interface{} {
	dspConfig := map[string]interface{}{}
	if dsp.TestSize != nil {
		dspConfig["test_size"] = *dsp.TestSize
	}
	if dsp.RandomState != nil {
		dspConfig["random_state"] = *dsp.RandomState
	}
	if dsp.Shuffle != nil {
		dspConfig["shuffle"] = *dsp.Shuffle
	}
	return dspConfig
}

// buildRLConfigMap builds freqai.rl_config from AIConfig.RLConfig.
func buildRLConfigMap(rl *v1alpha1.RLConfig) map[string]interface{} {
	rlConfig := map[string]interface{}{}

	if rl.DropOHLCFromFeatures != nil {
		rlConfig["drop_ohlc_from_features"] = *rl.DropOHLCFromFeatures
	}
	if rl.TrainCycles != nil {
		rlConfig["train_cycles"] = *rl.TrainCycles
	}
	if rl.MaxTradeDurationCandles != nil {
		rlConfig["max_trade_duration_candles"] = *rl.MaxTradeDurationCandles
	}
	if rl.AddStateInfo != nil {
		rlConfig["add_state_info"] = *rl.AddStateInfo
	}
	if rl.MaxTrainingDrawdownPct != nil {
		rlConfig["max_training_drawdown_pct"] = *rl.MaxTrainingDrawdownPct
	}
	if rl.CPUCount != nil {
		rlConfig["cpu_count"] = *rl.CPUCount
	}
	if rl.ModelType != "" {
		rlConfig["model_type"] = rl.ModelType
	}
	if rl.PolicyType != "" {
		rlConfig["policy_type"] = rl.PolicyType
	}
	if len(rl.NetArch) > 0 {
		rlConfig["net_arch"] = rl.NetArch
	}
	if rl.RandomizeStartingPosition != nil {
		rlConfig["randomize_starting_position"] = *rl.RandomizeStartingPosition
	}
	if rl.ProgressBar != nil {
		rlConfig["progress_bar"] = *rl.ProgressBar
	}
	if rl.ModelRewardParameters != nil {
		rlConfig["model_reward_parameters"] = buildModelRewardParametersConfig(rl.ModelRewardParameters)
	}

	return rlConfig
}

// buildModelRewardParametersConfig builds rl_config.model_reward_parameters
// from RLConfig.ModelRewardParameters.
func buildModelRewardParametersConfig(mrp *v1alpha1.ModelRewardParameters) map[string]interface{} {
	mrpConfig := map[string]interface{}{}
	if mrp.RR != nil {
		mrpConfig["rr"] = *mrp.RR
	}
	if mrp.ProfitAim != nil {
		mrpConfig["profit_aim"] = *mrp.ProfitAim
	}
	return mrpConfig
}

// buildDataConfigFields builds the top-level config keys from
// TradeBotConfigSpec.Data.
func buildDataConfigFields(data *v1alpha1.DataConfig) map[string]interface{} {
	cfg := map[string]interface{}{}

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
		cfg["new_pairs_days"] = *data.NewPairsDaysAgo
	}
	if data.DownloadTrades != nil {
		cfg["download_trades"] = *data.DownloadTrades
	}
	if data.MaxEntryPositionAdjustment != nil {
		cfg["max_entry_position_adjustment"] = *data.MaxEntryPositionAdjustment
	}
	if data.AvailableCapital != nil {
		cfg["available_capital"] = *data.AvailableCapital
	}
	if data.AmendLastStakeAmount != nil {
		cfg["amend_last_stake_amount"] = *data.AmendLastStakeAmount
	}
	if data.LastStakeAmountMinRatio != nil {
		cfg["last_stake_amount_min_ratio"] = data.LastStakeAmountMinRatio
	}
	if data.ProcessOnlyNewCandles != nil {
		cfg["process_only_new_candles"] = *data.ProcessOnlyNewCandles
	}
	if data.AmountReservePercent != nil {
		cfg["amount_reserve_percent"] = *data.AmountReservePercent
	}
	if data.ReduceDfFootprint != nil {
		cfg["reduce_df_footprint"] = *data.ReduceDfFootprint
	}
	if data.CustomPriceMaxDistanceRatio != nil {
		cfg["custom_price_max_distance_ratio"] = *data.CustomPriceMaxDistanceRatio
	}

	return cfg
}

// buildAdvancedConfigFields builds the top-level config keys from
// TradeBotConfigSpec.Advanced.
func buildAdvancedConfigFields(adv *v1alpha1.AdvancedConfig) map[string]interface{} {
	cfg := map[string]interface{}{}

	if adv.TradableBalanceRatio != nil {
		cfg["tradable_balance_ratio"] = *adv.TradableBalanceRatio
	}
	if adv.CancelOpenOrdersOnExit != nil && *adv.CancelOpenOrdersOnExit {
		cfg["cancel_open_orders_on_exit"] = *adv.CancelOpenOrdersOnExit
	}
	if adv.MarginMode != "" {
		cfg["margin_mode"] = adv.MarginMode
	}
	if adv.InitialState != "" {
		cfg["initial_state"] = adv.InitialState
	}
	if adv.ForceEntryEnable != nil {
		cfg["force_entry_enable"] = *adv.ForceEntryEnable
	}

	return cfg
}

// buildUnfilledTimeoutConfig builds the "unfilledtimeout" section from
// TradeBotConfigSpec.Timeout.
func buildUnfilledTimeoutConfig(timeout *v1alpha1.UnfilledTimeoutConfig) map[string]interface{} {
	timeoutConfig := map[string]interface{}{}
	if timeout.Entry != nil {
		timeoutConfig["entry"] = *timeout.Entry
	}
	if timeout.Exit != nil {
		timeoutConfig["exit"] = *timeout.Exit
	}
	if timeout.ExitTimeoutCount != nil {
		timeoutConfig["exit_timeout_count"] = *timeout.ExitTimeoutCount
	}
	if timeout.Unit != "" {
		timeoutConfig["unit"] = timeout.Unit
	}
	return timeoutConfig
}

// buildInternalsConfigFields builds the "internals" section from
// TradeBotConfigSpec.Internals.
func buildInternalsConfigFields(internals *v1alpha1.InternalsConfig) map[string]interface{} {
	internalsConfig := map[string]interface{}{}
	if internals.ProcessThrottleSecs != nil {
		internalsConfig["process_throttle_secs"] = *internals.ProcessThrottleSecs
	}
	if internals.Interval != nil {
		internalsConfig["interval"] = *internals.Interval
	}
	if internals.SdNotify != nil {
		internalsConfig["sd_notify"] = *internals.SdNotify
	}
	return internalsConfig
}

// buildAPIServerConfig builds the "api_server" section from
// TradeBotConfigSpec.APIServer. apiCredentials (secretRef) always wins over
// the deprecated plaintext username/password fields; jwtSecretKey is
// pre-resolved by resolveAPIServerJWTSecretKey (P3-4) - plaintext/secretRef
// precedence, minimum-length validation, and generate-if-absent are all
// already settled by the time it gets here.
func buildAPIServerConfig(
	apiServer *v1alpha1.APIServerConfig,
	apiCredentials map[string][]byte,
	jwtSecretKey string,
	extraCorsHosts []string,
) map[string]interface{} {
	cfg := map[string]interface{}{}

	if apiServer.Enabled != nil {
		cfg["enabled"] = *apiServer.Enabled
	}
	if apiServer.ListenIP != "" {
		cfg["listen_ip_address"] = apiServer.ListenIP
	}
	if apiServer.ListenPort != nil {
		cfg["listen_port"] = *apiServer.ListenPort
	}
	if apiServer.Verbosity != "" {
		cfg["verbosity"] = apiServer.Verbosity
	}
	if apiServer.EnableOpenAPI != nil {
		cfg["enable_openapi"] = *apiServer.EnableOpenAPI
	}

	// Deprecated plaintext credential fields (P3-1) are read first, as a
	// fallback only - secretRef always wins below when it supplies a
	// value. The webhook rejects setting Password/JWTSecretKey at all
	// unless freqtrade.io/allow-plaintext-credentials is set, so the
	// normal path never reaches here with anything to read.
	if apiServer.Username != "" {
		cfg["username"] = apiServer.Username
	}
	if apiServer.Password != "" {
		cfg["password"] = apiServer.Password
	}

	if apiCredentials != nil {
		if apiCredentials["user"] != nil {
			cfg["username"] = string(apiCredentials["user"])
		}
		if apiCredentials["password"] != nil {
			cfg["password"] = string(apiCredentials["password"])
		}
	}

	if jwtSecretKey != "" {
		cfg["jwt_secret_key"] = jwtSecretKey
	}

	corsOrigins := apiServer.CORSOrigins
	if len(extraCorsHosts) > 0 {
		corsSet := make(map[string]struct{}, len(corsOrigins))
		for _, v := range corsOrigins {
			corsSet[v] = struct{}{}
		}
		for _, v := range extraCorsHosts {
			if _, exists := corsSet[v]; !exists {
				corsOrigins = append(corsOrigins, v)
				corsSet[v] = struct{}{}
			}
		}
	}
	if len(corsOrigins) > 0 {
		cfg["CORS_origins"] = corsOrigins
	}

	return cfg
}
