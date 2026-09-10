package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TradeBotConfigSpec defines the desired state of TradeBotConfig
type TradeBotConfigSpec struct {
	// NOTE: Base schema configuration reference is at https://schema.freqtrade.io/schema.json

	Bot       *BotConfig             `json:"bot"`
	AI        *AIConfig              `json:"ai,omitempty"`
	Data      *DataConfig            `json:"data,omitempty"`
	Advanced  *AdvancedConfig        `json:"advanced,omitempty"`
	Timeout   *UnfilledTimeoutConfig `json:"timeout,omitempty"`
	Internals *InternalsConfig       `json:"internals,omitempty"`

	Exchange       *ExchangeSpec        `json:"exchange,omitempty"`
	Pairlist       *PairListSpec        `json:"pairlist,omitempty"`
	PairlistMethod *PairlistMethodsSpec `json:"pairlist_method,omitempty"`
	EntryPricing   *PricingSpec         `json:"entry_pricing,omitempty"`
	ExitPricing    *PricingSpec         `json:"exit_pricing,omitempty"`
	Order          *OrderSpec           `json:"order,omitempty"`
	RiskManagement *RiskManagementSpec  `json:"risk_management,omitempty"`
	Notification   *NotificationSpec    `json:"notification,omitempty"`

	APIServer    *APIServerConfig    `json:"apiServer,omitempty"`
	Experimental *ExperimentalConfig `json:"experimental,omitempty"`
	Logging      *LoggingConfig      `json:"logging,omitempty"`
}

type BotConfig struct {
	// Bot configuration
	BotName string `json:"bot_name,omitempty"`
	// +kubebuilder:validation:Enum=spot;margin;futures
	TradingMode string `json:"trading_mode,omitempty"`
	DryRun      *bool  `json:"dry_run,omitempty"`
	// +kubebuilder:validation:Minimum=0
	DryRunWallet  *float64 `json:"dry_run_wallet,omitempty"`
	StakeCurrency string   `json:"stake_currency,omitempty"`
	StakeAmount   string   `json:"stake_amount,omitempty"`
	// +kubebuilder:validation:Minimum=-1
	MaxOpenTrades          *int   `json:"max_open_trades,omitempty"`
	FiatDisplayCurrency    string `json:"fiat_display_currency,omitempty"`
	DBUrl                  string `json:"db_url,omitempty"`
	Export                 string `json:"export,omitempty"`
	DisableParamExport     *bool  `json:"disable_param_export,omitempty"`
	DisableDataframeChecks *bool  `json:"disable_dataframe_checks,omitempty"`
}

type AIConfig struct {
	// AI configuration
	Enabled                          *bool                    `json:"enabled,omitempty"`               // Whether AI is enabled
	Identifier                       string                   `json:"identifier,omitempty"`            // Identifier for the AI model
	WriteMetricsToDisk               *bool                    `json:"write_metrics_to_disk,omitempty"` // Whether to write AI metrics to disk
	PurgeOldModels                   *int                     `json:"purge_old_models,omitempty"`      // Whether to purge old AI models
	ConvWidth                        *int                     `json:"conv_width,omitempty"`            // Width of the convolutional layers
	TrainPeriodDays                  *int                     `json:"train_period_days,omitempty"`     // Training period in days
	BacktestPeriodDays               *int                     `json:"backtest_period_days,omitempty"`
	LiveRetrainHours                 *int                     `json:"live_retrain_hours,omitempty"` // Hours between live retraining
	ExpirationHours                  *int                     `json:"expiration_hours,omitempty"`
	SaveBacktestModels               *bool                    `json:"save_backtest_models,omitempty"`                  // Whether to save backtest models
	FitLivePredictionsCandles        *int                     `json:"fit_live_predictions_candles,omitempty"`          // Whether to fit live predictions on candles
	DataKitchenThreadCount           *int                     `json:"data_kitchen_thread_count,omitempty"`             // Number of threads for data kitchen processing
	ActivateTensorboard              *bool                    `json:"activate_tensorboard,omitempty"`                  // Whether to activate TensorBoard for AI model training
	WaitForTrainingIterationOnReload *bool                    `json:"wait_for_training_iteration_on_reload,omitempty"` // Whether to wait for training iteration on reload
	ContinueLearning                 *bool                    `json:"continue_learning,omitempty"`                     // Whether to continue learning from the last model
	Keras                            *bool                    `json:"keras,omitempty"`
	FeatureParameters                *FeatureParameters       `json:"feature_parameters,omitempty"`        // Parameters for feature extraction
	DataSplitParameters              *DataSplitParameters     `json:"data_split_parameters,omitempty"`     // Parameters for data splitting
	ModelTrainingParameters          *ModelTrainingParameters `json:"model_training_parameters,omitempty"` // Parameters for model training
	RLConfig                         *RLConfig                `json:"rl_config,omitempty"`                 // Reinforcement Learning configuration
}

type FeatureParameters struct {
	IncludeCorrPairlist        []string   `json:"include_corr_pairlist,omitempty"` // List of correlated pairs to include in the features
	IncludeTimeframes          []string   `json:"include_timeframes,omitempty"`    // A list
	LabelPeriodCandles         *int       `json:"label_period_candles,omitempty"`
	IncludeShiftedCandles      *int       `json:"include_shifted_candles,omitempty"`      // Add features from previous candles to subsequent candles
	DIThreshold                *float64   `json:"di_threshold,omitempty"`                 // Activates the
	WeightFactor               *float64   `json:"weight_factor,omitempty"`                // Weight training data points according to their recency
	PrincipalComponentAnalysis *bool      `json:"principal_component_analysis,omitempty"` // Automatically reduce the Dimensionality of the data set using Principal Component Analysis
	IndicatorPeriodsCandles    []int      `json:"indicator_periods_candles,omitempty"`    // Time periods to calculate indicators for
	UseSVMToRemoveOutliers     *bool      `json:"use_svm_to_remove_outliers,omitempty"`   // Use SVM to remove outliers from the features
	PlotFeatureImportances     *int       `json:"plot_feature_importances,omitempty"`     // Create feature importance plots for each model
	SVMParams                  *SVMParams `json:"svm_params,omitempty"`                   // All parameters available in Sklearn's `SGDOneClassSVM()`
	ShuffleAfterSplit          *bool      `json:"shuffle_after_split,omitempty"`          // Split the data into train and test sets, and then shuffle both sets individually
	BufferTrainDataCandles     *int       `json:"buffer_train_data_candles,omitempty"`    // Cut `buffer_train_data_c

}

type SVMParams struct {
	Shuffle *bool    `json:"shuffle,omitempty"` // Whether to shuffle data before applying SVM
	Nu      *float64 `json:"nu,omitempty"`      // Nu parameter for S
}

type DataSplitParameters struct {
	TestSize    *float64 `json:"test_size,omitempty"`    // Size of the test set as a fraction of the total dataset
	RandomState *int     `json:"random_state,omitempty"` // Random state for reproducibility
	Shuffle     *bool    `json:"shuffle,omitempty"`      // Whether to shuffle the dataset before splitting
}

type ModelTrainingParameters struct {
	/*
	   "model_training_parameters": {
	     "description": "Flexible dictionary that includes all parameters available by the selected model library. ",
	     "type": "object"
	   },
	*/
}

type RLConfig struct {
	DropOHLCFromFeatures      *bool                  `json:"drop_ohlc_from_features,omitempty"`     // Do not include the normalized OHLC data in the feature set
	TrainCycles               *int                   `json:"train_cycles,omitempty"`                // Number of training
	MaxTradeDurationCandles   *int                   `json:"max_trade_duration_candles,omitempty"`  // Guides the agent training to keep trades below desired length
	AddStateInfo              *bool                  `json:"add_state_info,omitempty"`              // Include state information in the feature set for training and inference
	MaxTrainingDrawdownPct    *float64               `json:"max_training_drawdown_pct,omitempty"`   // Maximum allowed drawdown percentage during training
	CPUCount                  *int                   `json:"cpu_count,omitempty"`                   // Number of threads/CPU's to use for training
	ModelType                 string                 `json:"model_type,omitempty"`                  // Model string from stable_baselines3 or SBcontrib
	PolicyType                string                 `json:"policy_type,omitempty"`                 // One of the available policy types from stable_baselines3
	NetArch                   []int                  `json:"net_arch,omitempty"`                    // Architecture of the neural network
	RandomizeStartingPosition *bool                  `json:"randomize_starting_position,omitempty"` // Randomize the starting point of each episode to avoid overfitting
	ProgressBar               *bool                  `json:"progress_bar,omitempty"`                // Display a progress bar with the current progress
	ModelRewardParameters     *ModelRewardParameters `json:"model_reward_parameters,omitempty"`     // Parameters for configuring the reward model
}

type ModelRewardParameters struct {
	RR        *float64 `json:"rr,omitempty"`         // Reward ratio parameter
	ProfitAim *float64 `json:"profit_aim,omitempty"` // Profit aim parameter
}
type DataConfig struct {
	DataformatOHLCV             string   `json:"dataformat_ohlcv,omitempty"`
	DataformatTrades            string   `json:"dataformat_trades,omitempty"`
	PositionAdjustment          string   `json:"position_adjustment,omitempty"`
	NewPairsDaysAgo             *int     `json:"new_pairs_days_ago,omitempty"`
	DownloadTrades              *bool    `json:"download_trades,omitempty"`
	MaxEntryPositionAdjustment  *float64 `json:"max_entry_position_adjustment,omitempty"`
	AvailableCapital            *float64 `json:"available_capital,omitempty"`               // Available capital for trading
	AmendLastStakeAmount        *bool    `json:"amend_last_stake_amount,omitempty"`         // Whether to amend the last stake amount
	LastStakeAmountMinRatio     *float64 `json:"last_stake_amount_min_ratio,omitempty"`     // Minimum ratio for the last stake amount
	ProcessOnlyNewCandles       *bool    `json:"process_only_new_candles,omitempty"`        // Process only new candles
	AmountReservePercent        *float64 `json:"amount_reserve_percent,omitempty"`          // Percentage of amount to reserve
	ReduceDfFootprint           *bool    `json:"reduce_df_footprint,omitempty"`             // Reduce DataFrame footprint
	CustomPriceMaxDistanceRatio *float64 `json:"custom_price_max_distance_ratio,omitempty"` // Maximum distance ratio for custom price
}

type AdvancedConfig struct {
	// Advanced trading configuration
	TradableBalanceRatio   *float64 `json:"tradable_balance_ratio,omitempty"`
	CancelOpenOrdersOnExit *bool    `json:"cancel_open_orders_on_exit,omitempty"`
	MarginMode             string   `json:"margin_mode,omitempty"`
	InitialState           string   `json:"initial_state,omitempty"`
	ForceEntryEnable       *bool    `json:"force_entry_enable,omitempty"`
}

// UnfilledTimeoutConfig defines timeout settings for unfilled orders
type UnfilledTimeoutConfig struct {
	Entry            *int   `json:"entry,omitempty"`
	Exit             *int   `json:"exit,omitempty"`
	ExitTimeoutCount *int   `json:"exit_timeout_count,omitempty"`
	Unit             string `json:"unit,omitempty"`
}

// InternalsConfig defines internal processing configuration
type InternalsConfig struct {
	ProcessThrottleSecs *int  `json:"process_throttle_secs,omitempty"`
	Interval            *int  `json:"interval,omitempty"`  // Interval in seconds for internal processing
	SdNotify            *bool `json:"sd_notify,omitempty"` // Enable systemd notification
}

// PairListSpec defines the desired state of PairList
type PairListSpec struct {
	Pairs []string `json:"pairs,omitempty"`
}

// PairlistMethod represents the method used for pair selection
type PairlistMethod string

const (
	// StaticPairList uses a statically defined pair whitelist from the configuration
	StaticPairList PairlistMethod = "StaticPairList"
	// VolumePairList employs sorting/filtering of pairs by their trading volume
	VolumePairList PairlistMethod = "VolumePairList"
	// PercentChangePairList selects pairs based on percent change
	PercentChangePairList PairlistMethod = "PercentChangePairList"
	// ProducerPairList reuses the pairlist from a Producer
	ProducerPairList PairlistMethod = "ProducerPairList"
	// RemotePairList fetches a pairlist from a remote server or a locally stored json file
	RemotePairList PairlistMethod = "RemotePairList"
	// MarketCapPairList selects pairs based on market capitalization
	MarketCapPairList PairlistMethod = "MarketCapPairList"
	// AgeFilter removes pairs that have been listed on the exchange for less than min_days_listed
	AgeFilter PairlistMethod = "AgeFilter"
	// FullTradesFilter shrinks whitelist to consist only in-trade pairs when the trade slots are full
	FullTradesFilter PairlistMethod = "FullTradesFilter"
	// OffsetFilter applies an offset to the pairlist
	OffsetFilter PairlistMethod = "OffsetFilter"
	// PerformanceFilter filters pairs by their performance
	PerformanceFilter PairlistMethod = "PerformanceFilter"
	// PrecisionFilter filters pairs by their precision
	PrecisionFilter PairlistMethod = "PrecisionFilter"
	// PriceFilter filters pairs by their price
	PriceFilter PairlistMethod = "PriceFilter"
	// ShuffleFilter shuffles the pairlist
	ShuffleFilter PairlistMethod = "ShuffleFilter"
	// SpreadFilter filters pairs by their spread
	SpreadFilter PairlistMethod = "SpreadFilter"
	// RangeStabilityFilter filters pairs by their range stability
	RangeStabilityFilter PairlistMethod = "RangeStabilityFilter"
	// VolatilityFilter filters pairs by their volatility
	VolatilityFilter PairlistMethod = "VolatilityFilter"
)

// PairlistConfig represents the configuration for a pairlist method
type PairlistConfig struct {
	// Method is the pairlist method to use
	Method PairlistMethod `json:"method"`

	// Common configuration options
	NumberAssets  *int   `json:"number_assets,omitempty"`
	RefreshPeriod *int64 `json:"refresh_period,omitempty"`

	// StaticPairList specific options
	AllowInactive bool `json:"allow_inactive,omitempty"`

	// VolumePairList specific options
	SortKey string `json:"sort_key,omitempty"` // Only "quoteVolume" is supported

	// PercentChangePairList specific options
	LookbackTimeframe string   `json:"lookback_timeframe,omitempty"`
	LookbackPeriod    *int     `json:"lookback_period,omitempty"`
	LookbackDays      *int     `json:"lookback_days,omitempty"`
	MinChangeRate     *float64 `json:"min_change_rate,omitempty"`

	// ProducerPairList specific options
	ProducerName string `json:"producer_name,omitempty"`

	// RemotePairList specific options
	Mode                  string `json:"mode,omitempty"`
	ProcessingMode        string `json:"processing_mode,omitempty"`
	PairlistURL           string `json:"pairlist_url,omitempty"`
	KeepPairlistOnFailure *bool  `json:"keep_pairlist_on_failure,omitempty"`
	ReadTimeout           *int   `json:"read_timeout,omitempty"`
	BearerToken           string `json:"bearer_token,omitempty"`
	SaveToFile            string `json:"save_to_file,omitempty"`

	// MarketCapPairList specific options
	MaxRank    *int     `json:"max_rank,omitempty"`
	Categories []string `json:"categories,omitempty"`

	// AgeFilter specific options
	MinDaysListed *int `json:"min_days_listed,omitempty"`
	MaxDaysListed *int `json:"max_days_listed,omitempty"`

	// PriceFilter specific options
	MinPrice *float64 `json:"min_price,omitempty"`
	MaxPrice *float64 `json:"max_price,omitempty"`

	// SpreadFilter specific options
	MaxSpreadRatio *float64 `json:"max_spread_ratio,omitempty"`

	// RangeStabilityFilter specific options
	LookbackDaysRange *int     `json:"lookback_days_range,omitempty"`
	MinRateOfChange   *float64 `json:"min_rate_of_change,omitempty"`
	MaxRateOfChange   *float64 `json:"max_rate_of_change,omitempty"`

	// VolatilityFilter specific options
	LookbackDaysVolatility *int     `json:"lookback_days_volatility,omitempty"`
	MinVolatility          *float64 `json:"min_volatility,omitempty"`
	MaxVolatility          *float64 `json:"max_volatility,omitempty"`

	// OffsetFilter specific options
	Offset *int `json:"offset,omitempty"`
}

// PairlistMethodsSpec defines the desired state of PairlistMethods
type PairlistMethodsSpec struct {
	// Methods is an array of pairlist configurations
	Methods []PairlistConfig `json:"methods"`
}

// PricingCheckDepthOfMarket defines the check_depth_of_market settings
type PricingCheckDepthOfMarket struct {
	Enabled        *bool    `json:"enabled,omitempty"`
	BidsToAskDelta *float64 `json:"bids_to_ask_delta,omitempty"`
}

// PricingSpec defines the desired state of Pricing
type PricingSpec struct {
	PriceSide          string                     `json:"price_side,omitempty"`
	PriceLastBalance   *float64                   `json:"price_last_balance,omitempty"`
	UseOrderBook       *bool                      `json:"use_order_book,omitempty"`
	OrderBookTop       *int                       `json:"order_book_top,omitempty"`
	CheckDepthOfMarket *PricingCheckDepthOfMarket `json:"check_depth_of_market,omitempty"`
}

type ExchangeSpec struct {
	Name string `json:"name"`
	// Deprecated: stored unencrypted in etcd and readable by anyone who can
	// get this TradeBotConfig. Use secretRef instead (P3-1); this field is
	// rejected by the validating webhook unless
	// freqtrade.io/allow-plaintext-credentials is set, and will be removed
	// in v1beta1.
	Key string `json:"key,omitempty"`
	// Deprecated: see Key.
	Secret string `json:"secret,omitempty"`
	// Deprecated: see Key.
	Password string `json:"password,omitempty"`
	// Deprecated: see Key.
	UID       string `json:"uid,omitempty"`
	AccountID string `json:"account_id,omitempty"` // Not a credential (an identifier, not a secret) - not deprecated.
	// Deprecated: see Key.
	WalletAddress string `json:"wallet_address,omitempty"`
	// Deprecated: see Key.
	PrivateKey            string               `json:"private_key,omitempty"`
	SecretRef             string               `json:"secretRef,omitempty"` // Reference to Secret containing API credentials.
	CcxtConfig            apiextensionsv1.JSON `json:"ccxt_config,omitempty"`
	CcxtAsyncConfig       apiextensionsv1.JSON `json:"ccxt_async_config,omitempty"`
	CcxtSyncConfig        apiextensionsv1.JSON `json:"ccxt_sync_config,omitempty"` // CCXT sync config for the exchange
	Whitelist             *PairListSpec        `json:"whitelist,omitempty"`        // Reference to PairList
	Blacklist             *PairListSpec        `json:"blacklist,omitempty"`        // Reference to PairList
	LogResponses          *bool                `json:"log_responses,omitempty"`
	EnableWS              *bool                `json:"enable_ws,omitempty"` // Enable WebSocket support
	UnkownFeeRate         *bool                `json:"unkown_fee_rate,omitempty"`
	OutdatedOffset        *int                 `json:"outdated_offset,omitempty"`         // Offset in minutes for outdated data
	MarketRefreshInterval *int                 `json:"market_refresh_interval,omitempty"` // Interval in seconds to refresh market data

}

type OrderSpec struct {
	// Types contains configuration for different order types
	// +optional
	Types *Types `json:"types,omitempty"`

	// TimeInForce contains configuration for order time in force options
	// +optional
	TimeInForce *TimeInForce `json:"time_in_force,omitempty"`

	// Flow contains configuration for order flow processing
	// +optional
	Flow *Flow `json:"flow,omitempty"`
}

// Types defines various order type configurations
type Types struct {
	Entry                        string   `json:"entry,omitempty"`
	Exit                         string   `json:"exit,omitempty"`
	EmergencyExit                string   `json:"emergency_exit,omitempty"`
	ForceEntry                   string   `json:"force_entry,omitempty"`
	ForceExit                    string   `json:"force_exit,omitempty"`
	Stoploss                     string   `json:"stoploss,omitempty"`
	StoplossOnExchange           *bool    `json:"stoploss_on_exchange,omitempty"`
	StoplossOnExchangeInterval   *int     `json:"stoploss_on_exchange_interval,omitempty"`
	StoplossOnExchangeLimitRatio *float64 `json:"stoploss_on_exchange_limit_ratio,omitempty"`
}

// TimeInForce defines options for order time in force
type TimeInForce struct {
	Entry string `json:"entry,omitempty"`
	Exit  string `json:"exit,omitempty"`
}

type Flow struct {
	CacheSize             *int     `json:"cache_size,omitempty"` // Size of the cache for order flow
	MaxCandles            *int     `json:"max_candles,omitempty"`
	Scale                 *float64 `json:"scale,omitempty"`
	StackedImbalanceRange *int     `json:"stacked_imbalance_range,omitempty"` // Range for stacked imbalance
	ImbalanceVolume       *int     `json:"imbalance_volume,omitempty"`
	ImbalanceRatio        *float64 `json:"imbalance_ratio,omitempty"` // Ratio for imbalance

}

// RiskManagementSpec defines the desired state of RiskManagement
// +kubebuilder:validation:XValidation:rule="!has(self.trailing_stop_positive_offset) || !has(self.trailing_stop_positive) || self.trailing_stop_positive_offset > self.trailing_stop_positive",message="trailing_stop_positive_offset must be greater than trailing_stop_positive"
type RiskManagementSpec struct {
	MinimalROI                  map[string]*float64 `json:"minimal_roi,omitempty"`
	Stoploss                    *float64            `json:"stoploss,omitempty"`
	TrailingStop                bool                `json:"trailing_stop,omitempty"`
	TrailingStopPositive        *float64            `json:"trailing_stop_positive,omitempty"`
	TrailingStopPositiveOffset  *float64            `json:"trailing_stop_positive_offset,omitempty"`
	TrailingOnlyOffsetIsReached bool                `json:"trailing_only_offset_is_reached,omitempty"`

	UseExitSignal                   bool     `json:"use_exit_signal,omitempty"`
	ExitProfitOnly                  bool     `json:"exit_profit_only,omitempty"`
	ExitProfitOffset                *float64 `json:"exit_profit_offset,omitempty"`
	Fee                             *float64 `json:"fee,omitempty"`
	IgnoreRoiIfEntrySignal          bool     `json:"ignore_roi_if_entry_signal,omitempty"`
	IgnoreBuyingExpiredCandleAfter  *int     `json:"ignore_buying_expired_candle_after,omitempty"`
	MinimumTradeAmount              *int     `json:"minimum_trade_amount,omitempty"`
	TargetedTradeAmount             *int     `json:"targeted_trade_amount,omitempty"`
	LookaheadAnalysisExportFilename string   `json:"lookahead_analysis_export_filename,omitempty"`
	StartupCandle                   *[]int   `json:"startup_candle,omitempty"`
	LiquidationBuffer               *float64 `json:"liquidation_buffer,omitempty"`
	BacktestBreakdown               []string `json:"backtest_breakdown,omitempty"`
}

type APIServerConfig struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	ListenIP string `json:"listen_ip_address,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ListenPort    *int   `json:"listen_port,omitempty"`
	Verbosity     string `json:"verbosity,omitempty"`
	EnableOpenAPI *bool  `json:"enable_openapi,omitempty"`
	Username      string `json:"username,omitempty"`
	// Deprecated: stored unencrypted in etcd and readable by anyone who can
	// get this TradeBotConfig. Use secretRef instead (P3-1); this field is
	// rejected by the validating webhook unless
	// freqtrade.io/allow-plaintext-credentials is set, and will be removed
	// in v1beta1.
	Password string `json:"password,omitempty"`
	// Deprecated: see Password.
	JWTSecretKey string   `json:"jwtSecretKey,omitempty"`
	SecretRef    string   `json:"secretRef,omitempty"`
	CORSOrigins  []string `json:"cors_origins,omitempty"`
}

type ExperimentalConfig struct {
	BlockBadExchanges *bool `json:"block_bad_exchanges,omitempty"`
}

type LoggingConfig struct {
	Version *int `json:"version,omitempty"`
}

type NotificationSpec struct {
	Telegram *NotificationTelegram `json:"telegram,omitempty"`
	Webhook  *NotificationWebhook  `json:"webhook,omitempty"`
	Discord  *NotificationDiscord  `json:"discord,omitempty"`
}

type NotificationTelegram struct {
	Enabled *bool `json:"enabled,omitempty"`
	// Deprecated: stored unencrypted in etcd and readable by anyone who can
	// get this TradeBotConfig. Use secretRef instead (P3-1); this field is
	// rejected by the validating webhook unless
	// freqtrade.io/allow-plaintext-credentials is set, and will be removed
	// in v1beta1.
	Token               string   `json:"token,omitempty"`
	SecretRef           string   `json:"secretRef,omitempty"` // Reference to Secret containing Telegram credentials
	BalanceDustLevel    *float64 `json:"balance_dust_level,omitempty"`
	Reload              *bool    `json:"reload,omitempty"`
	AllowCustomMessages *bool    `json:"allow_custom_messages,omitempty"`
	ChatID              string   `json:"chat_id,omitempty"`  // Telegram chat ID to send notifications
	TopicID             string   `json:"topic_id,omitempty"` // Topic ID for grouping notifications
	AuthorizedUsers     []string `json:"authorized_users,omitempty"`

	Settings *NotificationTelegramSettings `json:"settings,omitempty"` // Custom settings for Telegram notifications

}

type NotificationTelegramSettings struct {
	Status                  string `json:"status,omitempty"`
	Warning                 string `json:"warning,omitempty"`
	Startup                 string `json:"startup,omitempty"`                   // Startup message
	Entry                   string `json:"entry,omitempty"`                     // Entry message
	EntryFill               string `json:"entry_fill,omitempty"`                // Entry fill message
	EntryCancel             string `json:"entry_cancel,omitempty"`              // Entry cancel message
	Exit                    string `json:"exit,omitempty"`                      // Exit message
	ExitFill                string `json:"exit_fill,omitempty"`                 // Exit fill message
	ExitCancel              string `json:"exit_cancel,omitempty"`               // Exit cancel message
	ProtectionTrigger       string `json:"protection_trigger,omitempty"`        // Protection trigger message
	ProtectionTriggerGlobal string `json:"protection_trigger_global,omitempty"` // Global protection trigger message

}

type NotificationWebhook struct {
	Enabled     *bool  `json:"enabled,omitempty"`
	URL         string `json:"url,omitempty"`
	Entry       string `json:"entry,omitempty"`
	EntryCancel string `json:"entry_cancel,omitempty"`
	EntryFill   string `json:"entry_fill,omitempty"`
	Exit        string `json:"exit,omitempty"`
	ExitCancel  string `json:"exit_cancel,omitempty"`
	ExitFill    string `json:"exit_fill,omitempty"`
	Status      string `json:"status,omitempty"`
	// Deprecated: not a real Freqtrade webhook option - allow_custom_messages
	// only exists under `telegram` (see NotificationTelegram.AllowCustomMessages).
	// configbuilder no longer renders this field into config.json; it has no
	// effect regardless of its value.
	AllowCustomMessages *bool `json:"allow_custom_messages,omitempty"`
}

type NotificationDiscord struct {
	Enabled    *bool               `json:"enabled,omitempty"`
	WebhookURL string              `json:"webhook_url,omitempty"` // Discord webhook URL, recommended to be set via environment variable
	ExitFill   []map[string]string `json:"exit_fill,omitempty"`   // Exit fill message template
	EntryFill  []map[string]string `json:"entry_fill,omitempty"`  // Entry fill message template

}

// TradeBotConfigStatus defines the observed state of TradeBotConfig
type TradeBotConfigStatus struct {
	// Phase is a derived, human-facing summary for the printer column only -
	// it is never the source of truth. Conditions are.
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`

	// +optional
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the most recent metadata.generation this status
	// was computed from.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:deprecatedversion:warning="freqtrade.io/v1alpha1 TradeBotConfig is deprecated; use freqtrade.io/v1beta1. Plaintext credential fields must move to a Secret + secretRef first - see https://github.com/ark-sys/freqtrade-operator#upgrading-to-v1beta1"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Exchange",type=string,JSONPath=`.spec.exchange.name`
// +kubebuilder:printcolumn:name="DryRun",type=boolean,JSONPath=`.spec.bot.dry_run`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=tbc

type TradeBotConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TradeBotConfigSpec   `json:"spec,omitempty"`
	Status TradeBotConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type TradeBotConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TradeBotConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TradeBotConfig{}, &TradeBotConfigList{})
}
