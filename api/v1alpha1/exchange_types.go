package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExchangeSpec defines the desired state of Exchange
type ExchangeSpec struct {
	Name            string               `json:"name"`
	ApiKey          string               `json:"apiKey"`
	Secret          string               `json:"secret"`
	CcxtConfig      apiextensionsv1.JSON `json:"ccxtConfig,omitempty"`
	CcxtAsyncConfig apiextensionsv1.JSON `json:"ccxtAsyncConfig,omitempty"`
	WhitelistRef    string               `json:"whitelistRef,omitempty"` // Reference to PairList
	BlacklistRef    string               `json:"blacklistRef,omitempty"` // Reference to PairList
}

// ExchangeStatus defines the observed state of Exchange
type ExchangeStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type Exchange struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExchangeSpec   `json:"spec,omitempty"`
	Status ExchangeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type ExchangeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Exchange `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Exchange{}, &ExchangeList{})
}
