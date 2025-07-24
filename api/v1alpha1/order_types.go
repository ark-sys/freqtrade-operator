package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrderSpec defines the desired state of Order configuration
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
	StoplossOnExchange           bool     `json:"stoploss_on_exchange,omitempty"`
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

// OrderStatus defines the observed state of Order
type OrderStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Order is the Schema for the orders API
type Order struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrderSpec   `json:"spec,omitempty"`
	Status OrderStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OrderList contains a list of Order
type OrderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Order `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Order{}, &OrderList{})
}
