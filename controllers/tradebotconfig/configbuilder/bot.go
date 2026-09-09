package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildTradeBotConfig builds the base bot configuration for Freqtrade config.json
func BuildTradeBotConfig(tradeBotName string, tradeBotConfig *v1alpha1.TradeBotConfig, apiCredentials map[string][]byte, extraCorsHosts []string) (map[string]interface{}, error) {
	if tradeBotConfig == nil {
		return nil, nil
	}

	cfg := map[string]interface{}{}

	if tradeBotConfig.Spec.Bot != nil {
		// BotConfig
		bot := tradeBotConfig.Spec.Bot

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
		if bot.DisableParamExport != nil {
			cfg["disable_param_export"] = *bot.DisableParamExport
		}
		if bot.DisableDataframeChecks != nil {
			cfg["disable_dataframe_checks"] = *bot.DisableDataframeChecks
		}

	}

	if tradeBotConfig.Spec.AI != nil {
		ai := tradeBotConfig.Spec.AI
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
			aiConfig["continue_learning"] = *ai.ContinueLearning
		}
		if ai.Keras != nil {
			aiConfig["keras"] = *ai.Keras
		}

		// FeatureParameters
		if ai.FeatureParameters != nil {
			fp := ai.FeatureParameters
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
				fpConfig["di_threshold"] = *fp.DIThreshold
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
				fpConfig["use_svm_to_remove_outliers"] = *fp.UseSVMToRemoveOutliers
			}
			if fp.PlotFeatureImportances != nil {
				fpConfig["plot_feature_importances"] = *fp.PlotFeatureImportances
			}
			if fp.SVMParams != nil {
				svm := fp.SVMParams
				svmConfig := map[string]interface{}{}
				if svm.Shuffle != nil {
					svmConfig["shuffle"] = *svm.Shuffle
				}
				if svm.Nu != nil {
					svmConfig["nu"] = *svm.Nu
				}
				fpConfig["svm_params"] = svmConfig
			}
			if fp.ShuffleAfterSplit != nil {
				fpConfig["shuffle_after_split"] = *fp.ShuffleAfterSplit
			}
			if fp.BufferTrainDataCandles != nil {
				fpConfig["buffer_train_data_candles"] = *fp.BufferTrainDataCandles
			}
			aiConfig["feature_parameters"] = fpConfig
		}

		// DataSplitParameters
		if ai.DataSplitParameters != nil {
			dsp := ai.DataSplitParameters
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
			aiConfig["data_split_parameters"] = dspConfig
		}

		// ModelTrainingParameters
		if ai.ModelTrainingParameters != nil {
			// If ModelTrainingParameters has fields, map them here.
			aiConfig["model_training_parameters"] = ai.ModelTrainingParameters
		}

		// RLConfig
		if ai.RLConfig != nil {
			rl := ai.RLConfig
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
				mrp := rl.ModelRewardParameters
				mrpConfig := map[string]interface{}{}
				if mrp.RR != nil {
					mrpConfig["rr"] = *mrp.RR
				}
				if mrp.ProfitAim != nil {
					mrpConfig["profit_aim"] = *mrp.ProfitAim
				}
				rlConfig["model_reward_parameters"] = mrpConfig
			}
			aiConfig["rl_config"] = rlConfig
		}

		cfg["freqai"] = aiConfig
	}

	// DataConfig
	if tradeBotConfig.Spec.Data != nil {
		data := tradeBotConfig.Spec.Data
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
			cfg["new_pairs_days_ago"] = *data.NewPairsDaysAgo
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
	}

	// AdvancedConfig
	if tradeBotConfig.Spec.Advanced != nil {
		adv := tradeBotConfig.Spec.Advanced
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
	}

	// UnfilledTimeoutConfig
	if tradeBotConfig.Spec.Timeout != nil {
		timeoutConfig := map[string]interface{}{}
		if tradeBotConfig.Spec.Timeout.Entry != nil {
			timeoutConfig["entry"] = *tradeBotConfig.Spec.Timeout.Entry
		}
		if tradeBotConfig.Spec.Timeout.Exit != nil {
			timeoutConfig["exit"] = *tradeBotConfig.Spec.Timeout.Exit
		}
		if tradeBotConfig.Spec.Timeout.ExitTimeoutCount != nil {
			timeoutConfig["exit_timeout_count"] = *tradeBotConfig.Spec.Timeout.ExitTimeoutCount
		}
		if tradeBotConfig.Spec.Timeout.Unit != "" {
			timeoutConfig["unit"] = tradeBotConfig.Spec.Timeout.Unit
		}
		if len(timeoutConfig) > 0 {
			cfg["unfilledtimeout"] = timeoutConfig
		}
	}

	// InternalsConfig
	if tradeBotConfig.Spec.Internals != nil {
		internalsConfig := map[string]interface{}{}
		if tradeBotConfig.Spec.Internals.ProcessThrottleSecs != nil {
			internalsConfig["process_throttle_secs"] = *tradeBotConfig.Spec.Internals.ProcessThrottleSecs
		}
		if tradeBotConfig.Spec.Internals.Interval != nil {
			internalsConfig["interval"] = *tradeBotConfig.Spec.Internals.Interval
		}
		if tradeBotConfig.Spec.Internals.SdNotify != nil {
			internalsConfig["sd_notify"] = *tradeBotConfig.Spec.Internals.SdNotify
		}
		if len(internalsConfig) > 0 {
			cfg["internals"] = internalsConfig
		}
	}

	// API Server
	if tradeBotConfig.Spec.APIServer != nil {

		apiServer := map[string]interface{}{}
		if tradeBotConfig.Spec.APIServer.Enabled != nil {
			apiServer["enabled"] = *tradeBotConfig.Spec.APIServer.Enabled
		}
		if tradeBotConfig.Spec.APIServer.ListenIP != "" {
			apiServer["listen_ip_address"] = tradeBotConfig.Spec.APIServer.ListenIP
		}
		if tradeBotConfig.Spec.APIServer.ListenPort != nil {
			apiServer["listen_port"] = *tradeBotConfig.Spec.APIServer.ListenPort
		}
		if tradeBotConfig.Spec.APIServer.Verbosity != "" {
			apiServer["verbosity"] = tradeBotConfig.Spec.APIServer.Verbosity
		}
		if tradeBotConfig.Spec.APIServer.EnableOpenAPI != nil {
			apiServer["enable_openapi"] = *tradeBotConfig.Spec.APIServer.EnableOpenAPI
		}

		// Deprecated plaintext credential fields (P3-1) are read first, as a
		// fallback only - secretRef always wins below when it supplies a
		// value. The webhook rejects setting Password/JWTSecretKey at all
		// unless freqtrade.io/allow-plaintext-credentials is set, so the
		// normal path never reaches here with anything to read.
		if tradeBotConfig.Spec.APIServer.Username != "" {
			apiServer["username"] = tradeBotConfig.Spec.APIServer.Username
		}
		if tradeBotConfig.Spec.APIServer.Password != "" {
			apiServer["password"] = tradeBotConfig.Spec.APIServer.Password
		}
		if tradeBotConfig.Spec.APIServer.JWTSecretKey != "" {
			apiServer["jwt_secret_key"] = tradeBotConfig.Spec.APIServer.JWTSecretKey
		}

		if apiCredentials != nil {
			if apiCredentials["user"] != nil {
				apiServer["username"] = string(apiCredentials["user"])
			}
			if apiCredentials["password"] != nil {
				apiServer["password"] = string(apiCredentials["password"])
			}
			if apiCredentials["jwt_secret_key"] != nil {
				apiServer["jwt_secret_key"] = string(apiCredentials["jwt_secret_key"])
			}
		}

		corsOrigins := tradeBotConfig.Spec.APIServer.CORSOrigins
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
			apiServer["CORS_origins"] = corsOrigins
		}

		cfg["api_server"] = apiServer
	}

	// ExperimentalConfig
	if tradeBotConfig.Spec.Experimental != nil && tradeBotConfig.Spec.Experimental.BlockBadExchanges != nil {
		cfg["block_bad_exchanges"] = *tradeBotConfig.Spec.Experimental.BlockBadExchanges
	}

	// LoggingConfig
	if tradeBotConfig.Spec.Logging != nil && tradeBotConfig.Spec.Logging.Version != nil {
		cfg["logging"] = map[string]interface{}{
			"version": *tradeBotConfig.Spec.Logging.Version,
		}
	}

	return cfg, nil
}
