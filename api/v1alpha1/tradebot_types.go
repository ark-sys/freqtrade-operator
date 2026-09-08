package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TradeBotSpec defines the desired state of TradeBot
type TradeBotSpec struct {
	// Default is "trade". Can be "backtesting" or "hyperopt"
	FreqtradeCommand   string   `json:"freqtrade_command,omitempty"`
	FreqtradeArguments []string `json:"freqtrade_arguments,omitempty"`

	Config string `json:"config"`

	// Reference to the Strategy resource
	Strategy string `json:"strategy"`

	App *TBAppConfig `json:"app,omitempty"`

	Data *DataCacheSpec `json:"data,omitempty"`
}

type DataCacheSpec struct {
	// PVCName is the name of a shared RWX PVC used as data cache for jobs.
	// If set, jobs will mount this PVC at /cache and use it as --datadir.
	PVCName string `json:"pvcName,omitempty"`

	// DownloadArgs are optional extra args for "freqtrade download-data".
	// Example: []string{"--exchange","binance","-t","1m","5m","--days","30"}
	DownloadArgs []string `json:"downloadArgs,omitempty"`

	// DownloadPolicy controls when the cache is refreshed by jobs.
	// Accepted: "always" (default), "ifMissing", "never".
	DownloadPolicy string `json:"downloadPolicy,omitempty"`
}

type TBAppConfig struct {
	PodSpec     *PodSpec     `json:"pod,omitempty"`     // Pod specification for the application
	ServiceSpec *ServiceSpec `json:"service,omitempty"` // Service specification for the application
	PVCSpec     *PVCSpec     `json:"pvc,omitempty"`     // Persistent Volume Claim specification for the application
}

type PodSpec struct {
	Image        string                  `json:"image,omitempty"`
	Resources    v1.ResourceRequirements `json:"resources,omitempty"`    // Resource requirements for the pod
	Env          []v1.EnvVar             `json:"env,omitempty"`          // Environment variables for the pod
	VolumeMounts []v1.VolumeMount        `json:"volumeMounts,omitempty"` // Volume mounts for the pod
	Volumes      []v1.Volume             `json:"volumes,omitempty"`      // Volumes for the pod
	// SecurityContext defines the security context for the pod
	SecurityContext *v1.PodSecurityContext `json:"securityContext,omitempty"` // Security context for the pod
	// InitContainers defines the init containers for the pod
	InitContainers []v1.Container `json:"initContainers,omitempty"` // Init containers for the pod
	ServiceName    string         `json:"serviceName,omitempty"`
	// ImagePullSecrets defines the image pull secrets for the pod
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"` // Image pull secrets for the pod
	LivenessProbe    *v1.Probe                 `json:"livenessProbe,omitempty"`
	ReadinessProbe   *v1.Probe                 `json:"readinessProbe,omitempty"` // Readiness probe for the pod
	// Scheduling parameters
	Affinity                  *v1.Affinity                  `json:"affinity,omitempty"`
	AntiAffinity              *v1.PodAntiAffinity           `json:"antiAffinity,omitempty"`
	NodeSelector              map[string]string             `json:"nodeSelector,omitempty"`
	Tolerations               []v1.Toleration               `json:"tolerations,omitempty"` // Tolerations for the pod
	TopologySpreadConstraints []v1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

type ServiceSpec struct {
	// Type defines the type of service (e.g., ClusterIP, NodePort, LoadBalancer)
	Type v1.ServiceType `json:"type,omitempty"`
	// Ports defines the ports for the service
	Ports []v1.ServicePort `json:"ports,omitempty"`
	// Selector defines the labels to select the pods for this service
	Selector map[string]string `json:"selector,omitempty"`
	// Annotations defines additional annotations for the service
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PVCSpec defines the Persistent Volume Claim specification
type PVCSpec struct {
	// AccessModes defines the access modes for the PVC (e.g., ReadWriteOnce, ReadOnlyMany)
	AccessModes []v1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	// StorageSize defines the size of the storage for the PVC
	StorageSize string `json:"storageSize,omitempty"` // Size of the storage for the PVC
	// StorageClassName defines the storage class for the PVC
	StorageClassName string `json:"storageClassName,omitempty"`
	// VolumeName defines the name of the volume to bind to
	VolumeName string `json:"volumeName,omitempty"`
	// Annotations defines additional annotations for the PVC
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels defines additional labels for the PVC
	Labels map[string]string `json:"labels,omitempty"`
}

// TradeBotStatus defines the observed state of TradeBot
type TradeBotStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type TradeBot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TradeBotSpec   `json:"spec,omitempty"`
	Status TradeBotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type TradeBotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TradeBot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TradeBot{}, &TradeBotList{})
}
