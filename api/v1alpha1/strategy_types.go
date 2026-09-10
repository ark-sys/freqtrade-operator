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
	// Phase is a derived, human-facing summary for the printer column only -
	// it is never the source of truth. Conditions are.
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`

	// +optional
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the most recent metadata.generation this status
	// was computed from.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:deprecatedversion:warning="freqtrade.io/v1alpha1 Strategy is deprecated; use freqtrade.io/v1beta1. See https://github.com/ark-sys/freqtrade-operator#upgrading-to-v1beta1"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Name",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=strat

type Strategy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StrategySpec   `json:"spec,omitempty"`
	Status StrategyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type StrategyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Strategy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Strategy{}, &StrategyList{})
}
