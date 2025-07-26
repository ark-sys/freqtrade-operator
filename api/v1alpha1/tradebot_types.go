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
	StakeAmount            string   `json:"stake_amount,omitempty"`
	MaxOpenTrades          *int     `json:"max_open_trades,omitempty"`
	FiatDisplayCurrency    string   `json:"fiat_display_currency,omitempty"`
	DBUrl                  string   `json:"db_url,omitempty"`
	Export                 string   `json:"export,omitempty"`
	DisableParamExport     *bool    `json:"disable_param_export,omitempty"`
	DisableDataframeChecks *bool    `json:"disable_dataframe_checks,omitempty"`
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
	Enabled       *bool  `json:"enabled,omitempty"`
	ListenIP      string `json:"listen_ip_address,omitempty"`
	ListenPort    *int   `json:"listen_port,omitempty"`
	Verbosity     string `json:"verbosity,omitempty"`
	EnableOpenAPI *bool  `json:"enable_openapi,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	//jwtSecretKey string `json:"jwtSecretKey,omitempty"`
	SecretRef   string   `json:"secretRef,omitempty"`
	CORSOrigins []string `json:"cors_origins,omitempty"`
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
	Phase        string `json:"phase,omitempty"`
	Message      string `json:"message,omitempty"`
	JWTSecretKey string `json:"jwt_secret_key,omitempty"`
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
