package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PairListSpec defines the desired state of PairList
type PairListSpec struct {
	Pairs []string `json:"pairs,omitempty"`
}

// PairListStatus defines the observed state of PairList
type PairListStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type PairList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PairListSpec   `json:"spec,omitempty"`
	Status PairListStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type PairListList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PairList `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PairList{}, &PairListList{})
}
