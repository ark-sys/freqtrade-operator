package v1alpha1

import (
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
	// PodSpec is the pod specification for FreqUI
	PodSpec *FUPodSpec `json:"pod,omitempty"`

	// ServiceSpec is the service specification for FreqUI
	ServiceSpec *FUServiceSpec `json:"service,omitempty"`

	// IngressSpec is the ingress specification for FreqUI
	IngressSpec *FUIngressSpec `json:"ingress,omitempty"`
}

// FUPodSpec defines the pod specification for FreqUI
type FUPodSpec struct {
	// Image is the container image for FreqUI
	Image string `json:"image,omitempty"`
	// Resources defines the resource requirements for the pod
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// Env defines environment variables for the pod
	Env []corev1.EnvVar `json:"env,omitempty"`
	// VolumeMounts defines volume mounts for the pod
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
	// Volumes defines volumes for the pod
	Volumes []corev1.Volume `json:"volumes,omitempty"`
	// SecurityContext defines the security context for the pod
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// InitContainers defines init containers for the pod
	InitContainers []corev1.Container `json:"initContainers,omitempty"`
	// ImagePullSecrets defines image pull secrets for the pod
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// LivenessProbe defines the liveness probe for the pod
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`
	// ReadinessProbe defines the readiness probe for the pod
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
	// Affinity defines pod affinity rules
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// AntiAffinity defines pod anti-affinity rules
	AntiAffinity *corev1.PodAntiAffinity `json:"antiAffinity,omitempty"`
	// NodeSelector defines node selector for the pod
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations defines tolerations for the pod
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// TopologySpreadConstraints defines topology spread constraints
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// Replicas defines the number of pod replicas
	Replicas *int32 `json:"replicas,omitempty"`
}

// FUServiceSpec defines the service specification for FreqUI
type FUServiceSpec struct {
	// Type defines the type of service (e.g., ClusterIP, NodePort, LoadBalancer)
	Type corev1.ServiceType `json:"type,omitempty"`
	// Ports defines the ports for the service
	Ports []corev1.ServicePort `json:"ports,omitempty"`
	// Selector defines the labels to select the pods for this service
	Selector map[string]string `json:"selector,omitempty"`
	// Annotations defines additional annotations for the service
	Annotations map[string]string `json:"annotations,omitempty"`
	// LoadBalancerSourceRanges defines source ranges for load balancer
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges,omitempty"`
	// ExternalTrafficPolicy defines the external traffic policy
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicyType `json:"externalTrafficPolicy,omitempty"`
	// SessionAffinity defines session affinity
	SessionAffinity corev1.ServiceAffinity `json:"sessionAffinity,omitempty"`
}

// FUIngressSpec defines the ingress specification for FreqUI
type FUIngressSpec struct {
	// IngressClassName defines the ingress class name
	IngressClassName *string `json:"ingressClassName,omitempty"`
	// Rules defines ingress rules
	Rules []networkingv1.IngressRule `json:"rules,omitempty"`
	// TLS defines TLS configuration
	TLS []networkingv1.IngressTLS `json:"tls,omitempty"`
	// DefaultBackend defines the default backend
	DefaultBackend *networkingv1.IngressBackend `json:"defaultBackend,omitempty"`
	// Annotations defines additional annotations for the ingress
	Annotations map[string]string `json:"annotations,omitempty"`
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
//+kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
//+kubebuilder:resource:shortName=fui

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
