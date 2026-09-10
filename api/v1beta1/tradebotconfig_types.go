package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// TradeBotConfigSpec defines the desired state of TradeBotConfig. B2
// (REMAINING-WORK.md): every plaintext credential field v1alpha1 carried
// behind the freqtrade.io/allow-plaintext-credentials gate is gone here -
// secretRef (now a typed, same-namespace-only corev1.LocalObjectReference
// rather than a bare string) is the only path. Everything else is
// field-for-field identical to v1alpha1.TradeBotConfigSpec.
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
	Enabled                          *bool                    `json:"enabled,omitempty"`
	Identifier                       string                   `json:"identifier,omitempty"`
	WriteMetricsToDisk               *bool                    `json:"write_metrics_to_disk,omitempty"`
	PurgeOldModels                   *int                     `json:"purge_old_models,omitempty"`
	ConvWidth                        *int                     `json:"conv_width,omitempty"`
	TrainPeriodDays                  *int                     `json:"train_period_days,omitempty"`
	BacktestPeriodDays               *int                     `json:"backtest_period_days,omitempty"`
	LiveRetrainHours                 *int                     `json:"live_retrain_hours,omitempty"`
	ExpirationHours                  *int                     `json:"expiration_hours,omitempty"`
	SaveBacktestModels               *bool                    `json:"save_backtest_models,omitempty"`
	FitLivePredictionsCandles        *int                     `json:"fit_live_predictions_candles,omitempty"`
	DataKitchenThreadCount           *int                     `json:"data_kitchen_thread_count,omitempty"`
	ActivateTensorboard              *bool                    `json:"activate_tensorboard,omitempty"`
	WaitForTrainingIterationOnReload *bool                    `json:"wait_for_training_iteration_on_reload,omitempty"`
	ContinueLearning                 *bool                    `json:"continue_learning,omitempty"`
	Keras                            *bool                    `json:"keras,omitempty"`
	FeatureParameters                *FeatureParameters       `json:"feature_parameters,omitempty"`
	DataSplitParameters              *DataSplitParameters     `json:"data_split_parameters,omitempty"`
	ModelTrainingParameters          *ModelTrainingParameters `json:"model_training_parameters,omitempty"`
	RLConfig                         *RLConfig                `json:"rl_config,omitempty"`
}

type FeatureParameters struct {
	IncludeCorrPairlist        []string   `json:"include_corr_pairlist,omitempty"`
	IncludeTimeframes          []string   `json:"include_timeframes,omitempty"`
	LabelPeriodCandles         *int       `json:"label_period_candles,omitempty"`
	IncludeShiftedCandles      *int       `json:"include_shifted_candles,omitempty"`
	DIThreshold                *float64   `json:"di_threshold,omitempty"`
	WeightFactor               *float64   `json:"weight_factor,omitempty"`
	PrincipalComponentAnalysis *bool      `json:"principal_component_analysis,omitempty"`
	IndicatorPeriodsCandles    []int      `json:"indicator_periods_candles,omitempty"`
	UseSVMToRemoveOutliers     *bool      `json:"use_svm_to_remove_outliers,omitempty"`
	PlotFeatureImportances     *int       `json:"plot_feature_importances,omitempty"`
	SVMParams                  *SVMParams `json:"svm_params,omitempty"`
	ShuffleAfterSplit          *bool      `json:"shuffle_after_split,omitempty"`
	BufferTrainDataCandles     *int       `json:"buffer_train_data_candles,omitempty"`
}

type SVMParams struct {
	Shuffle *bool    `json:"shuffle,omitempty"`
	Nu      *float64 `json:"nu,omitempty"`
}

type DataSplitParameters struct {
	TestSize    *float64 `json:"test_size,omitempty"`
	RandomState *int     `json:"random_state,omitempty"`
	Shuffle     *bool    `json:"shuffle,omitempty"`
}

type ModelTrainingParameters struct {
}

type RLConfig struct {
	DropOHLCFromFeatures      *bool                  `json:"drop_ohlc_from_features,omitempty"`
	TrainCycles               *int                   `json:"train_cycles,omitempty"`
	MaxTradeDurationCandles   *int                   `json:"max_trade_duration_candles,omitempty"`
	AddStateInfo              *bool                  `json:"add_state_info,omitempty"`
	MaxTrainingDrawdownPct    *float64               `json:"max_training_drawdown_pct,omitempty"`
	CPUCount                  *int                   `json:"cpu_count,omitempty"`
	ModelType                 string                 `json:"model_type,omitempty"`
	PolicyType                string                 `json:"policy_type,omitempty"`
	NetArch                   []int                  `json:"net_arch,omitempty"`
	RandomizeStartingPosition *bool                  `json:"randomize_starting_position,omitempty"`
	ProgressBar               *bool                  `json:"progress_bar,omitempty"`
	ModelRewardParameters     *ModelRewardParameters `json:"model_reward_parameters,omitempty"`
}

type ModelRewardParameters struct {
	RR        *float64 `json:"rr,omitempty"`
	ProfitAim *float64 `json:"profit_aim,omitempty"`
}

type DataConfig struct {
	DataformatOHLCV             string   `json:"dataformat_ohlcv,omitempty"`
	DataformatTrades            string   `json:"dataformat_trades,omitempty"`
	PositionAdjustment          string   `json:"position_adjustment,omitempty"`
	NewPairsDaysAgo             *int     `json:"new_pairs_days_ago,omitempty"`
	DownloadTrades              *bool    `json:"download_trades,omitempty"`
	MaxEntryPositionAdjustment  *float64 `json:"max_entry_position_adjustment,omitempty"`
	AvailableCapital            *float64 `json:"available_capital,omitempty"`
	AmendLastStakeAmount        *bool    `json:"amend_last_stake_amount,omitempty"`
	LastStakeAmountMinRatio     *float64 `json:"last_stake_amount_min_ratio,omitempty"`
	ProcessOnlyNewCandles       *bool    `json:"process_only_new_candles,omitempty"`
	AmountReservePercent        *float64 `json:"amount_reserve_percent,omitempty"`
	ReduceDfFootprint           *bool    `json:"reduce_df_footprint,omitempty"`
	CustomPriceMaxDistanceRatio *float64 `json:"custom_price_max_distance_ratio,omitempty"`
}

type AdvancedConfig struct {
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
	Interval            *int  `json:"interval,omitempty"`
	SdNotify            *bool `json:"sd_notify,omitempty"`
}

// PairListSpec defines the desired state of PairList
type PairListSpec struct {
	Pairs []string `json:"pairs,omitempty"`
}

// PairlistMethod represents the method used for pair selection
type PairlistMethod string

const (
	StaticPairList        PairlistMethod = "StaticPairList"
	VolumePairList        PairlistMethod = "VolumePairList"
	PercentChangePairList PairlistMethod = "PercentChangePairList"
	ProducerPairList      PairlistMethod = "ProducerPairList"
	RemotePairList        PairlistMethod = "RemotePairList"
	MarketCapPairList     PairlistMethod = "MarketCapPairList"
	AgeFilter             PairlistMethod = "AgeFilter"
	FullTradesFilter      PairlistMethod = "FullTradesFilter"
	OffsetFilter          PairlistMethod = "OffsetFilter"
	PerformanceFilter     PairlistMethod = "PerformanceFilter"
	PrecisionFilter       PairlistMethod = "PrecisionFilter"
	PriceFilter           PairlistMethod = "PriceFilter"
	ShuffleFilter         PairlistMethod = "ShuffleFilter"
	SpreadFilter          PairlistMethod = "SpreadFilter"
	RangeStabilityFilter  PairlistMethod = "RangeStabilityFilter"
	VolatilityFilter      PairlistMethod = "VolatilityFilter"
)

// PairlistConfig represents the configuration for a pairlist method
type PairlistConfig struct {
	Method PairlistMethod `json:"method"`

	NumberAssets  *int   `json:"number_assets,omitempty"`
	RefreshPeriod *int64 `json:"refresh_period,omitempty"`

	AllowInactive bool `json:"allow_inactive,omitempty"`

	SortKey string `json:"sort_key,omitempty"`

	LookbackTimeframe string   `json:"lookback_timeframe,omitempty"`
	LookbackPeriod    *int     `json:"lookback_period,omitempty"`
	LookbackDays      *int     `json:"lookback_days,omitempty"`
	MinChangeRate     *float64 `json:"min_change_rate,omitempty"`

	ProducerName string `json:"producer_name,omitempty"`

	Mode                  string `json:"mode,omitempty"`
	ProcessingMode        string `json:"processing_mode,omitempty"`
	PairlistURL           string `json:"pairlist_url,omitempty"`
	KeepPairlistOnFailure *bool  `json:"keep_pairlist_on_failure,omitempty"`
	ReadTimeout           *int   `json:"read_timeout,omitempty"`
	BearerToken           string `json:"bearer_token,omitempty"`
	SaveToFile            string `json:"save_to_file,omitempty"`

	MaxRank    *int     `json:"max_rank,omitempty"`
	Categories []string `json:"categories,omitempty"`

	MinDaysListed *int `json:"min_days_listed,omitempty"`
	MaxDaysListed *int `json:"max_days_listed,omitempty"`

	MinPrice *float64 `json:"min_price,omitempty"`
	MaxPrice *float64 `json:"max_price,omitempty"`

	MaxSpreadRatio *float64 `json:"max_spread_ratio,omitempty"`

	LookbackDaysRange *int     `json:"lookback_days_range,omitempty"`
	MinRateOfChange   *float64 `json:"min_rate_of_change,omitempty"`
	MaxRateOfChange   *float64 `json:"max_rate_of_change,omitempty"`

	LookbackDaysVolatility *int     `json:"lookback_days_volatility,omitempty"`
	MinVolatility          *float64 `json:"min_volatility,omitempty"`
	MaxVolatility          *float64 `json:"max_volatility,omitempty"`

	Offset *int `json:"offset,omitempty"`
}

// PairlistMethodsSpec defines the desired state of PairlistMethods
type PairlistMethodsSpec struct {
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

// ExchangeSpec is B2's primary target: every plaintext credential field
// (Key/Secret/Password/UID/WalletAddress/PrivateKey) v1alpha1 carried
// behind the allow-plaintext-credentials gate is gone - secretRef is the
// only path. AccountID stays: it's an identifier, not a secret, and was
// never gated (see configbuilder/exchange.go).
type ExchangeSpec struct {
	Name      string `json:"name"`
	AccountID string `json:"account_id,omitempty"`
	// SecretRef references a Secret in this TradeBotConfig's own namespace
	// only (D3) - cross-namespace references are a deliberate, documented
	// constraint, not a TODO.
	SecretRef       corev1.LocalObjectReference `json:"secretRef,omitempty"`
	CcxtConfig      apiextensionsv1.JSON        `json:"ccxt_config,omitempty"`
	CcxtAsyncConfig apiextensionsv1.JSON        `json:"ccxt_async_config,omitempty"`
	CcxtSyncConfig  apiextensionsv1.JSON        `json:"ccxt_sync_config,omitempty"`
	Whitelist       *PairListSpec               `json:"whitelist,omitempty"`
	Blacklist       *PairListSpec               `json:"blacklist,omitempty"`
	LogResponses    *bool                       `json:"log_responses,omitempty"`
	EnableWS        *bool                       `json:"enable_ws,omitempty"`
	// UnknownFeeRate closes A1's bug at the API level: v1alpha1 keeps its
	// original spelling (a served field can't be renamed without a
	// breaking change - see B2), but this fresh v1beta1 field gets the
	// correct name and JSON tag from the start.
	UnknownFeeRate        *bool `json:"unknown_fee_rate,omitempty"`
	OutdatedOffset        *int  `json:"outdated_offset,omitempty"`
	MarketRefreshInterval *int  `json:"market_refresh_interval,omitempty"`
}

type OrderSpec struct {
	Types       *Types       `json:"types,omitempty"`
	TimeInForce *TimeInForce `json:"time_in_force,omitempty"`
	Flow        *Flow        `json:"flow,omitempty"`
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
	CacheSize             *int     `json:"cache_size,omitempty"`
	MaxCandles            *int     `json:"max_candles,omitempty"`
	Scale                 *float64 `json:"scale,omitempty"`
	StackedImbalanceRange *int     `json:"stacked_imbalance_range,omitempty"`
	ImbalanceVolume       *int     `json:"imbalance_volume,omitempty"`
	ImbalanceRatio        *float64 `json:"imbalance_ratio,omitempty"`
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

// APIServerConfig: Password/JWTSecretKey are gone - see ExchangeSpec's own
// doc comment, same B2 reasoning. SecretRef is the only path.
type APIServerConfig struct {
	Enabled  *bool  `json:"enabled,omitempty"`
	ListenIP string `json:"listen_ip_address,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ListenPort    *int   `json:"listen_port,omitempty"`
	Verbosity     string `json:"verbosity,omitempty"`
	EnableOpenAPI *bool  `json:"enable_openapi,omitempty"`
	Username      string `json:"username,omitempty"`
	// SecretRef references a Secret in this TradeBotConfig's own namespace
	// only (D3).
	SecretRef   corev1.LocalObjectReference `json:"secretRef,omitempty"`
	CORSOrigins []string                    `json:"cors_origins,omitempty"`
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

// NotificationTelegram: Token is gone - see ExchangeSpec's own doc comment,
// same B2 reasoning. SecretRef is the only path. ChatID/TopicID stay
// plain strings: identifiers, not secrets, same as ExchangeSpec.AccountID.
type NotificationTelegram struct {
	Enabled *bool `json:"enabled,omitempty"`
	// SecretRef references a Secret in this TradeBotConfig's own namespace
	// only (D3).
	SecretRef           corev1.LocalObjectReference `json:"secretRef,omitempty"`
	BalanceDustLevel    *float64                    `json:"balance_dust_level,omitempty"`
	Reload              *bool                       `json:"reload,omitempty"`
	AllowCustomMessages *bool                       `json:"allow_custom_messages,omitempty"`
	ChatID              string                      `json:"chat_id,omitempty"`
	TopicID             string                      `json:"topic_id,omitempty"`
	AuthorizedUsers     []string                    `json:"authorized_users,omitempty"`

	Settings *NotificationTelegramSettings `json:"settings,omitempty"`
}

type NotificationTelegramSettings struct {
	Status                  string `json:"status,omitempty"`
	Warning                 string `json:"warning,omitempty"`
	Startup                 string `json:"startup,omitempty"`
	Entry                   string `json:"entry,omitempty"`
	EntryFill               string `json:"entry_fill,omitempty"`
	EntryCancel             string `json:"entry_cancel,omitempty"`
	Exit                    string `json:"exit,omitempty"`
	ExitFill                string `json:"exit_fill,omitempty"`
	ExitCancel              string `json:"exit_cancel,omitempty"`
	ProtectionTrigger       string `json:"protection_trigger,omitempty"`
	ProtectionTriggerGlobal string `json:"protection_trigger_global,omitempty"`
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
	// Deprecated: not a real Freqtrade webhook option - see
	// v1alpha1.NotificationWebhook.AllowCustomMessages. Carried over
	// unchanged rather than dropped - B2's scope is credential fields and
	// typed references, not a general API cleanup.
	AllowCustomMessages *bool `json:"allow_custom_messages,omitempty"`
}

type NotificationDiscord struct {
	Enabled    *bool               `json:"enabled,omitempty"`
	WebhookURL string              `json:"webhook_url,omitempty"`
	ExitFill   []map[string]string `json:"exit_fill,omitempty"`
	EntryFill  []map[string]string `json:"entry_fill,omitempty"`
}

// TradeBotConfigStatus defines the observed state of TradeBotConfig -
// identical to v1alpha1's.
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
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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

// Hub marks TradeBotConfig as the conversion hub (controller-runtime's
// conversion.Hub interface) - v1beta1 is the storage version (above), so
// v1alpha1.TradeBotConfig converts to/from this directly; see
// api/v1alpha1/tradebotconfig_conversion.go.
func (*TradeBotConfig) Hub() {}

var _ conversion.Hub = &TradeBotConfig{}

func init() {
	SchemeBuilder.Register(&TradeBotConfig{}, &TradeBotConfigList{})
}
