package v1alpha1

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FreqUISpec defines the desired state of FreqUI
type FreqUISpec struct {
	// Image is the FreqUI Docker image to use
	Image string `json:"image,omitempty"`

	// Host is the hostname for the FreqUI ingress
	Host string `json:"host,omitempty"`

	// IngressAnnotations are annotations to add to the FreqUI ingress
	IngressAnnotations map[string]string `json:"ingressAnnotations,omitempty"`

	// TLS configuration for the FreqUI ingress
	TLS []networkingv1.IngressTLS `json:"tls,omitempty"`
}

// FreqUIStatus defines the observed state of FreqUI
type FreqUIStatus struct {
	// Phase is the current phase of the FreqUI deployment
	Phase string `json:"phase,omitempty"`

	// Message is a human-readable message indicating details about the current phase
	Message string `json:"message,omitempty"`

	// URL is the URL where FreqUI is accessible
	URL string `json:"url,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FreqUI is the Schema for the frequis API
type FreqUI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FreqUISpec   `json:"spec,omitempty"`
	Status FreqUIStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FreqUIList contains a list of FreqUI
type FreqUIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FreqUI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FreqUI{}, &FreqUIList{})
}
