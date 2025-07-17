package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MinimalROI maps duration (in minutes) to ROI ratio
type MinimalROI map[string]float64

// RiskManagementSpec defines the desired state of RiskManagement
type RiskManagementSpec struct {
	MinimalROI                  MinimalROI `json:"minimalRoi,omitempty"`
	Stoploss                    float64    `json:"stoploss,omitempty"`
	TrailingStop                bool       `json:"trailingStop,omitempty"`
	TrailingStopPositive        float64    `json:"trailingStopPositive,omitempty"`
	TrailingStopPositiveOffset  float64    `json:"trailingStopPositiveOffset,omitempty"`
	TrailingOnlyOffsetIsReached bool       `json:"trailingOnlyOffsetIsReached,omitempty"`
}

// RiskManagementStatus defines the observed state of RiskManagement
type RiskManagementStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type RiskManagement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RiskManagementSpec   `json:"spec,omitempty"`
	Status RiskManagementStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type RiskManagementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RiskManagement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RiskManagement{}, &RiskManagementList{})
}
