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
	StakeAmount         string  `json:"stakeAmount,omitempty"` // can be "unlimited"
	MaxOpenTrades       int     `json:"maxOpenTrades,omitempty"`
	FiatDisplayCurrency string  `json:"fiatDisplayCurrency,omitempty"`

	// References to other resources
	PairListRef       string `json:"pairListRef"`
	PairlistsRef      string `json:"pairlistsRef,omitempty"`
	ExchangeRef       string `json:"exchangeRef"`
	EntryPricingRef   string `json:"entryPricingRef,omitempty"`
	ExitPricingRef    string `json:"exitPricingRef,omitempty"`
	OrderTypesRef     string `json:"orderTypesRef,omitempty"`
	RiskManagementRef string `json:"riskManagementRef,omitempty"`
	NotificationRef   string `json:"notificationRef,omitempty"`
	StrategyRef       string `json:"strategyRef"`

	// Deployment configuration
	Image string `json:"image,omitempty"` // Docker image for freqtrade

	// Resources for the freqtrade container
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Storage configuration
	StorageSize      string `json:"storageSize,omitempty"`      // Size of the PVC (e.g., "1Gi")
	StorageClassName string `json:"storageClassName,omitempty"` // StorageClass to use for the PVC

	// API configuration
	APIEnabled bool `json:"apiEnabled,omitempty"` // Whether to enable the REST API

	// CORS settings for API
	CORSOrigins []string `json:"corsOrigins,omitempty"` // List of allowed origins for CORS
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
