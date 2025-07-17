package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EntryPricingCheckDepthOfMarket defines the check_depth_of_market settings
type EntryPricingCheckDepthOfMarket struct {
	Enabled        bool    `json:"enabled,omitempty"`
	BidsToAskDelta float64 `json:"bidsToAskDelta,omitempty"`
}

// EntryPricingSpec defines the desired state of EntryPricing
type EntryPricingSpec struct {
	PriceSide          string                         `json:"priceSide,omitempty"`
	PriceLastBalance   bool                           `json:"priceLastBalance,omitempty"`
	UseOrderBook       bool                           `json:"useOrderBook,omitempty"`
	OrderBookTop       int                            `json:"orderBookTop,omitempty"`
	CheckDepthOfMarket EntryPricingCheckDepthOfMarket `json:"checkDepthOfMarket,omitempty"`
}

// EntryPricingStatus defines the observed state of EntryPricing
type EntryPricingStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type EntryPricing struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EntryPricingSpec   `json:"spec,omitempty"`
	Status EntryPricingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type EntryPricingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EntryPricing `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EntryPricing{}, &EntryPricingList{})
}
