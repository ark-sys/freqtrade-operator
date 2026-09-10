package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// StrategySpec defines the desired state of Strategy - field-for-field
// identical to v1alpha1.StrategySpec (B3, REMAINING-WORK.md): Strategy
// carries neither credentials nor references, so graduating it is a
// mechanical copy, not a breaking change in substance. Done anyway rather
// than leaving it on v1alpha1 alone, to avoid a papercut every user would
// otherwise hit (every other kind on v1beta1, this one not).
type StrategySpec struct {
	// Name of the strategy (e.g., "SampleStrategy")
	Name string `json:"name"`
	// Script content or reference to a ConfigMap/Secret
	Script string `json:"script,omitempty"`
}

// StrategyStatus defines the observed state of Strategy - identical to
// v1alpha1's.
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
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
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

// Hub marks Strategy as the conversion hub (controller-runtime's
// conversion.Hub interface) - v1beta1 is the storage version (above), so
// v1alpha1.Strategy converts to/from this directly; see
// api/v1alpha1/strategy_conversion.go.
func (*Strategy) Hub() {}

var _ conversion.Hub = &Strategy{}

func init() {
	SchemeBuilder.Register(&Strategy{}, &StrategyList{})
}
