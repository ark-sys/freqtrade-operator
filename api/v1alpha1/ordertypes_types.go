package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrderTypesSpec defines the desired state of OrderTypes
type OrderTypesSpec struct {
	Entry                        string   `json:"entry,omitempty"`
	Exit                         string   `json:"exit,omitempty"`
	EmergencyExit                string   `json:"emergencyExit,omitempty"`
	ForceEntry                   string   `json:"forceEntry,omitempty"`
	ForceExit                    string   `json:"forceExit,omitempty"`
	Stoploss                     string   `json:"stoploss,omitempty"`
	StoplossOnExchange           *bool    `json:"stoplossOnExchange,omitempty"`
	StoplossOnExchangeInterval   *int     `json:"stoplossOnExchangeInterval,omitempty"`
	StoplossOnExchangeLimitRatio *float64 `json:"stoplossOnExchangeLimitRatio,omitempty"`
}

// OrderTypesStatus defines the observed state of OrderTypes
type OrderTypesStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type OrderTypes struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrderTypesSpec   `json:"spec,omitempty"`
	Status OrderTypesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type OrderTypesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OrderTypes `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OrderTypes{}, &OrderTypesList{})
}
