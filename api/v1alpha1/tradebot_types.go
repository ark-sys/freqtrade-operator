package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TradeBotSpec defines the desired state of TradeBot
type TradeBotSpec struct {
	// Bot configuration
	BotName             string  `json:"botName"`
	TradingMode         string  `json:"tradingMode,omitempty"` // spot, futures, etc.
	DryRun              bool    `json:"dryRun,omitempty"`
	DryRunWallet        float64 `json:"dryRunWallet,omitempty"`
	StakeCurrency       string  `json:"stakeCurrency,omitempty"`
	StakeAmount         string  `json:"stakeAmount,omitempty"`
	MaxOpenTrades       int     `json:"maxOpenTrades,omitempty"`
	FiatDisplayCurrency string  `json:"fiatDisplayCurrency,omitempty"`

	// Advanced trading configuration
	TradableBalanceRatio   *float64 `json:"tradableBalanceRatio,omitempty"`   // Ratio of available balance to use for trading
	CancelOpenOrdersOnExit *bool    `json:"cancelOpenOrdersOnExit,omitempty"` // Cancel open orders when bot exits
	MarginMode             string   `json:"marginMode,omitempty"`             // isolated, cross (for futures)
	InitialState           string   `json:"initialState,omitempty"`           // running, stopped
	ForceEntryEnable       *bool    `json:"forceEntryEnable,omitempty"`       // Allow force entry via API

	// Timeout configuration
	UnfilledTimeout *UnfilledTimeoutConfig `json:"unfilledTimeout,omitempty"`

	// Edge configuration
	Edge *EdgeConfig `json:"edge,omitempty"`

	// Internal processing configuration
	Internals *InternalsConfig `json:"internals,omitempty"`

	// References to other resources
	PairlistMethodsRef string `json:"pairlistMethodsRef,omitempty"`
	ExchangeRef        string `json:"exchangeRef"`
	EntryPricingRef    string `json:"entryPricingRef,omitempty"`
	ExitPricingRef     string `json:"exitPricingRef,omitempty"`
	OrderTypesRef      string `json:"orderTypesRef,omitempty"`
	RiskManagementRef  string `json:"riskManagementRef,omitempty"`
	NotificationRef    string `json:"notificationRef,omitempty"`
	StrategyRef        string `json:"strategyRef"`

	// Deployment configuration
	Image string `json:"image,omitempty"` // Docker image for freqtrade

	// Resources for the freqtrade container
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage configuration
	StorageSize      string `json:"storageSize,omitempty"`      // Size of the PVC (e.g., "1Gi")
	StorageClassName string `json:"storageClassName,omitempty"` // StorageClass to use for the PVC

	// API configuration
	APIEnabled bool `json:"apiEnabled,omitempty"` // Whether to enable the REST API

	// API server configuration
	APIServer *APIServerConfig `json:"apiServer,omitempty"` // API server settings

	// CORS settings for API
	CORSOrigins []string `json:"corsOrigins,omitempty"` // List of allowed origins for CORS
}

// UnfilledTimeoutConfig defines timeout settings for unfilled orders
type UnfilledTimeoutConfig struct {
	Entry            int    `json:"entry,omitempty"`            // Entry order timeout in minutes
	Exit             int    `json:"exit,omitempty"`             // Exit order timeout in minutes
	ExitTimeoutCount int    `json:"exitTimeoutCount,omitempty"` // Number of exit timeouts before giving up
	Unit             string `json:"unit,omitempty"`             // Time unit (minutes, seconds)
}

// EdgeConfig defines edge position sizing configuration
type EdgeConfig struct {
	Enabled                    *bool   `json:"enabled,omitempty"`
	ProcessThrottleSecs        int     `json:"processThrottleSecs,omitempty"`
	CalculateSinceNumberOfDays int     `json:"calculateSinceNumberOfDays,omitempty"`
	AllowedRisk                float64 `json:"allowedRisk,omitempty"`
	StoplossRangeMin           float64 `json:"stoplossRangeMin,omitempty"`
	StoplossRangeMax           float64 `json:"stoplossRangeMax,omitempty"`
	StoplossRangeStep          float64 `json:"stoplossRangeStep,omitempty"`
	MinimumWinrate             float64 `json:"minimumWinrate,omitempty"`
	MinimumExpectancy          float64 `json:"minimumExpectancy,omitempty"`
	MinTradeNumber             int     `json:"minTradeNumber,omitempty"`
	MaxTradeDurationMinute     int     `json:"maxTradeDurationMinute,omitempty"`
	RemovePumps                *bool   `json:"removePumps,omitempty"`
}

// InternalsConfig defines internal processing configuration
type InternalsConfig struct {
	ProcessThrottleSecs int `json:"processThrottleSecs,omitempty"` // Throttle processing in seconds
}

// APIServerConfig defines API server configuration
type APIServerConfig struct {
	ListenIP      string `json:"listenIpAddress,omitempty"` // IP address to listen on
	ListenPort    int    `json:"listenPort,omitempty"`      // Port to listen on
	Verbosity     string `json:"verbosity,omitempty"`       // Log verbosity level
	EnableOpenAPI *bool  `json:"enableOpenapi,omitempty"`   // Enable OpenAPI documentation
	Username      string `json:"username,omitempty"`        // API username
	Password      string `json:"password,omitempty"`        // API password
}

// TradeBotStatus defines the observed state of TradeBot
type TradeBotStatus struct {
	Phase        string `json:"phase,omitempty"`
	Message      string `json:"message,omitempty"`
	JWTSecretKey string `json:"jwtSecretKey,omitempty"` // Generated JWT secret key for API authentication
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
