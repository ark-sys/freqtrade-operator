package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExitPricingCheckDepthOfMarket defines the check_depth_of_market settings
type ExitPricingCheckDepthOfMarket struct {
	Enabled        bool    `json:"enabled,omitempty"`
	BidsToAskDelta float64 `json:"bidsToAskDelta,omitempty"`
}

// ExitPricingSpec defines the desired state of ExitPricing
type ExitPricingSpec struct {
	PriceSide          string                        `json:"priceSide,omitempty"`
	PriceLastBalance   bool                          `json:"priceLastBalance,omitempty"`
	UseOrderBook       bool                          `json:"useOrderBook,omitempty"`
	OrderBookTop       int                           `json:"orderBookTop,omitempty"`
	CheckDepthOfMarket ExitPricingCheckDepthOfMarket `json:"checkDepthOfMarket,omitempty"`
}

// ExitPricingStatus defines the observed state of ExitPricing
type ExitPricingStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type ExitPricing struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExitPricingSpec   `json:"spec,omitempty"`
	Status ExitPricingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type ExitPricingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExitPricing `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ExitPricing{}, &ExitPricingList{})
}
