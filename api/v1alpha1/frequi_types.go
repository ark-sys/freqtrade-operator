package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FreqUISpec defines the desired state of FreqUI
type FreqUISpec struct {
	//// Host is the hostname for the FreqUI ingress
	Host string `json:"host,omitempty"`

	//// TLS configuration for the FreqUI ingress
	TLS []networkingv1.IngressTLS `json:"tls,omitempty"`

	// IngressAnnotations are additional annotations for the FreqUI ingress
	IngressAnnotations map[string]string `json:"ingressAnnotations,omitempty"`

	// App is the configuration for the FreqUI application
	App *FUAppConfig `json:"app,omitempty"`

	// TradeBotRefs is a list of TradeBot names that this FreqUI should manage
	TradeBotRefs []string `json:"tradeBotRefs,omitempty"`
}

// FUAppConfig defines the configuration for the FreqUI application
type FUAppConfig struct {
	// DeploymentSpec is the deployment specification for FreqUI
	DeploymentSpec appsv1.DeploymentSpec `json:"deploymentSpec,omitempty"`

	// ServiceSpec is the service specification for FreqUI
	ServiceSpec corev1.ServiceSpec `json:"serviceSpec,omitempty"`

	// IngressSpec is the ingress specification for FreqUI
	IngressSpec networkingv1.IngressSpec `json:"ingressSpec,omitempty"`
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
