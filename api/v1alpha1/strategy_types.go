package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StrategySpec defines the desired state of Strategy
type StrategySpec struct {
	// Name of the strategy (e.g., "SampleStrategy")
	Name string `json:"name"`
	// Script content or reference to a ConfigMap/Secret
	Script string `json:"script,omitempty"`
}

// StrategyStatus defines the observed state of Strategy
type StrategyStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type Strategy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StrategySpec   `json:"spec,omitempty"`
	Status StrategyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type StrategyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Strategy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Strategy{}, &StrategyList{})
}
