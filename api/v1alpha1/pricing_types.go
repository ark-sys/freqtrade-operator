package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PricingCheckDepthOfMarket defines the check_depth_of_market settings
type PricingCheckDepthOfMarket struct {
	Enabled        bool    `json:"enabled,omitempty"`
	BidsToAskDelta float64 `json:"bids_to_ask_delta,omitempty"`
}

// PricingSpec defines the desired state of Pricing
type PricingSpec struct {
	PriceSide          string                    `json:"price_side,omitempty"`
	PriceLastBalance   bool                      `json:"price_last_balance,omitempty"`
	UseOrderBook       bool                      `json:"use_order_book,omitempty"`
	OrderBookTop       int                       `json:"order_book_top,omitempty"`
	CheckDepthOfMarket PricingCheckDepthOfMarket `json:"check_depth_of_market,omitempty"`
}

// PricingStatus defines the observed state of Pricing
type PricingStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type Pricing struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PricingSpec   `json:"spec,omitempty"`
	Status PricingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type PricingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pricing `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Pricing{}, &PricingList{})
}
