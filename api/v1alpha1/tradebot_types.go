package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TradeBotSpec defines the desired state of TradeBot
type TradeBotSpec struct {
	// NOTE: Base schema configuration reference is at https://schema.freqtrade.io/schema.json

	Bot          *BotConfig             `json:"bot"`
	AI           *AIConfig              `json:"ai,omitempty"`
	Data         *DataConfig            `json:"data,omitempty"`
	Advanced     *AdvancedConfig        `json:"advanced,omitempty"`
	Timeout      *UnfilledTimeoutConfig `json:"timeout,omitempty"`
	Internals    *InternalsConfig       `json:"internals,omitempty"`
	References   *References            `json:"references"`
	APIServer    *APIServerConfig       `json:"apiServer,omitempty"`
	Experimental *ExperimentalConfig    `json:"experimental,omitempty"`
	Logging      *LoggingConfig         `json:"logging,omitempty"`
	App          *TBAppConfig           `json:"app,omitempty"`
}

type BotConfig struct {
	// Bot configuration
	BotName                string   `json:"bot_name"`
	TradingMode            string   `json:"trading_mode,omitempty"`
	DryRun                 *bool    `json:"dry_run,omitempty"`
	DryRunWallet           *float64 `json:"dry_run_wallet,omitempty"`
	StakeCurrency          string   `json:"stake_currency,omitempty"`
	StakeAmount            *int     `json:"stake_amount,omitempty"`
	MaxOpenTrades          *int     `json:"max_open_trades,omitempty"`
	FiatDisplayCurrency    string   `json:"fiat_display_currency,omitempty"`
	DBUrl                  string   `json:"db_url,omitempty"`
	Export                 string   `json:"export,omitempty"`
	DisableParamExport     *bool    `json:"disable_param_export,omitempty"`
	DisableDataframeChecks *bool    `json:"disable_dataframe_checks,omitempty"`
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

type References struct {
	// References to other resources
	PairlistMethodsRef string `json:"pairlistMethodsRef,omitempty"`
	ExchangeRef        string `json:"exchangeRef"`
	EntryPricingRef    string `json:"entryPricingRef,omitempty"`
	ExitPricingRef     string `json:"exitPricingRef,omitempty"`
	OrderTypesRef      string `json:"orderTypesRef,omitempty"`
	RiskManagementRef  string `json:"riskManagementRef,omitempty"`
	NotificationRef    string `json:"notificationRef,omitempty"`
	StrategyRef        string `json:"strategyRef"`
}
type APIServerConfig struct {
	Enabled       *bool    `json:"enabled,omitempty"`
	ListenIP      string   `json:"listen_ip_address,omitempty"`
	ListenPort    *int     `json:"listen_port,omitempty"`
	Verbosity     string   `json:"verbosity,omitempty"`
	EnableOpenAPI *bool    `json:"enable_openapi,omitempty"`
	Username      string   `json:"username,omitempty"`
	Password      string   `json:"password,omitempty"`
	JWTSecretKey  string   `json:"jwtSecretKey,omitempty"`
	SecretRef     string   `json:"secretRef,omitempty"`
	CORSOrigins   []string `json:"cors_origins,omitempty"`
}

type ExperimentalConfig struct {
	BlockBadExchanges *bool `json:"block_bad_exchanges,omitempty"`
}

type LoggingConfig struct {
	Version *int `json:"version,omitempty"`
	//Formatters map[string]map[string]interface{} `json:"formatters,omitempty"`
	//Handlers   map[string]map[string]interface{} `json:"handlers,omitempty"`
	//Root       map[string]interface{}            `json:"root,omitempty"`
}
type TBAppConfig struct {
	StatefulSetSpec appsv1.StatefulSetSpec       `json:"spec,omitempty"`    // StatefulSet for the application
	ServiceSpec     v1.ServiceSpec               `json:"service,omitempty"` // Service for the application
	PVCSpec         v1.PersistentVolumeClaimSpec `json:"pvc,omitempty"`     // Persistent Volume Claim for the application
}

// TradeBotStatus defines the observed state of TradeBot
type TradeBotStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type TradeBot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TradeBotSpec   `json:"spec,omitempty"`
	Status TradeBotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type TradeBotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TradeBot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TradeBot{}, &TradeBotList{})
}
