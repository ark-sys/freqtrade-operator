package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExchangeSpec defines the desired state of Exchange
type ExchangeSpec struct {
	Name                  string               `json:"name"`
	Key                   string               `json:"key,omitempty"`      // API Key for the exchange
	Secret                string               `json:"secret,omitempty"`   // API Secret for the exchange
	Password              string               `json:"password,omitempty"` // Password for the exchange
	UID                   string               `json:"uid,omitempty"`      // User identifier for the exchange
	AccountID             string               `json:"account_id,omitempty"`
	WalletAddress         string               `json:"wallet_address,omitempty"` // Wallet address for the exchange
	PrivateKey            string               `json:"private_key,omitempty"`    // Private key for the exchange
	SecretRef             string               `json:"secretRef"`                // Reference to Secret containing API credentials. TODO In replacement of previous lds?
	CcxtConfig            apiextensionsv1.JSON `json:"ccxt_config,omitempty"`
	CcxtAsyncConfig       apiextensionsv1.JSON `json:"ccxt_async_config,omitempty"`
	CcxtSyncConfig        apiextensionsv1.JSON `json:"ccxt_sync_config,omitempty"` // CCXT sync config for the exchange
	WhitelistRef          string               `json:"whitelistRef,omitempty"`     // Reference to PairList
	BlacklistRef          string               `json:"blacklistRef,omitempty"`     // Reference to PairList
	LogResponses          bool                 `json:"log_responses,omitempty"`
	EnableWS              bool                 `json:"enable_ws,omitempty"` // Enable WebSocket support
	UnkownFeeRate         bool                 `json:"unkown_fee_rate,omitempty"`
	OutdatedOffset        *int                 `json:"outdated_offset,omitempty"`         // Offset in minutes for outdated data
	MarketRefreshInterval *int                 `json:"market_refresh_interval,omitempty"` // Interval in seconds to refresh market data

}

// ExchangeStatus defines the observed state of Exchange
type ExchangeStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type Exchange struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExchangeSpec   `json:"spec,omitempty"`
	Status ExchangeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type ExchangeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Exchange `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Exchange{}, &ExchangeList{})
}
