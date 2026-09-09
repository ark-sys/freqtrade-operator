package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TradeBotSpec defines the desired state of TradeBot
// +kubebuilder:validation:XValidation:rule="!has(self.data) || self.freqtrade_command != 'trade'",message="spec.data is only meaningful when freqtrade_command is not 'trade'"
type TradeBotSpec struct {
	// Default is "trade". Can be "backtesting" or "hyperopt"
	// +kubebuilder:validation:Enum=trade;backtesting;hyperopt;download-data;lookahead-analysis
	// +kubebuilder:default=trade
	FreqtradeCommand string `json:"freqtrade_command,omitempty"`
	// +kubebuilder:validation:MaxItems=64
	FreqtradeArguments []string `json:"freqtrade_arguments,omitempty"`

	// Reference to the TradeBotConfig resource, same namespace only (D3)
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Config string `json:"config"`

	// Reference to the Strategy resource, same namespace only (D3)
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Strategy string `json:"strategy"`

	App *TBAppConfig `json:"app,omitempty"`

	Data *DataCacheSpec `json:"data,omitempty"`

	// UpdateStrategy controls what happens when a config change can't take
	// effect without a restart (config.json is mounted from a Secret and
	// read once at freqtrade startup, so rewriting the Secret alone doesn't
	// change what a running bot is doing). "Manual" (the default, per D2)
	// leaves the running StatefulSet pod template untouched and reports the
	// pending restart via the ConfigDrift condition instead - a bot may be
	// holding open positions, so an unrequested restart is not this
	// controller's call to make. "Auto" writes the new config hash onto the
	// pod template, letting Kubernetes' own StatefulSet rolling update
	// carry out the restart. Only meaningful for freqtrade_command: trade;
	// a Job's pod template is already immutable after creation regardless
	// (see the WorkloadImmutable condition).
	// +kubebuilder:validation:Enum=Manual;Auto
	// +kubebuilder:default=Manual
	UpdateStrategy string `json:"updateStrategy,omitempty"`

	// Introspection controls the operator's own polling of this bot's
	// freqtrade REST API for live trading state (P4-3, implements D4).
	// Read-only: nothing here can start, stop, or otherwise act on the bot
	// (see P4-4 for that, a deliberately separate task).
	Introspection *IntrospectionSpec `json:"introspection,omitempty"`
}

// IntrospectionSpec controls whether and how often the operator polls this
// bot's own freqtrade REST API (P4-3). Polling only works at all when
// spec.app... has an api_server enabled with Basic Auth credentials the
// operator can read back from apiServer.secretRef - see BotStatus.State
// "unknown" and the BotReachable condition for what happens otherwise.
type IntrospectionSpec struct {
	// Enabled turns polling on or off for this bot. Defaults to on: a
	// TradeBot with no api_server configured at all just polls, fails to
	// reach anything, and reports BotReachable=False - turn this off
	// explicitly to silence that for a bot that deliberately has no API
	// server (e.g. backtesting/hyperopt Jobs, which never expose one).
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// Interval between polls. The admission webhook rejects anything under
	// 10s - freqtrade's REST API is not built for tight polling loops, and
	// this operator is not a market-data source.
	// +kubebuilder:default="60s"
	Interval metav1.Duration `json:"interval,omitempty"`
}

type DataCacheSpec struct {
	// PVCName is the name of a shared RWX PVC used as data cache for jobs.
	// If set, jobs will mount this PVC at /cache and use it as --datadir.
	PVCName string `json:"pvcName,omitempty"`

	// DownloadArgs are optional extra args for "freqtrade download-data".
	// Example: []string{"--exchange","binance","-t","1m","5m","--days","30"}
	DownloadArgs []string `json:"downloadArgs,omitempty"`

	// DownloadPolicy controls when the cache is refreshed by jobs.
	// +kubebuilder:validation:Enum=always;ifMissing;never
	// +kubebuilder:default=always
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
	// StorageSize defines the size of the storage for the PVC. A quantity
	// value (e.g. "10Gi") is validated by the API server at admission time;
	// as a plain string this used to reach resource.MustParse in the
	// reconciler and panic the manager on anything malformed.
	StorageSize *resource.Quantity `json:"storageSize,omitempty"`
	// StorageClassName defines the storage class for the PVC
	StorageClassName string `json:"storageClassName,omitempty"`
	// VolumeName defines the name of the volume to bind to
	VolumeName string `json:"volumeName,omitempty"`
	// Annotations defines additional annotations for the PVC
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels defines additional labels for the PVC
	Labels map[string]string `json:"labels,omitempty"`
	// FixVolumePermissions restores the pre-P3-3 behavior of running the
	// init-user-data init container as root to chmod/chown this PVC before
	// the main container starts. Off by default: spec.securityContext.fsGroup
	// (already set on every pod this operator builds) already makes the
	// volume group-writable on most CSI drivers, and running as root here is
	// a real, if narrow, privilege escalation on an otherwise fully
	// non-root pod. Turn this on only if pods are actually crash-looping on
	// a permission-denied error under /freqtrade/user_data and changing
	// storage class isn't an option.
	// +optional
	FixVolumePermissions *bool `json:"fixVolumePermissions,omitempty"`
}

// TradeBotStatus defines the observed state of TradeBot
type TradeBotStatus struct {
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
	// was computed from, so a client can tell whether it reflects the spec
	// it just applied.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// AppliedConfigHash is sha256(config.json bytes + strategy script
	// bytes), truncated to 16 hex characters: the hash of the config
	// currently rendered into this TradeBot's Secret, regardless of
	// whether it has been rolled out to the running StatefulSet pods yet
	// (see the ConfigDrift condition).
	AppliedConfigHash string `json:"appliedConfigHash,omitempty"`

	// ResolvedImage is the exact freqtrade image reference (normally
	// digest-pinned) the running workload was built with - the manager's
	// --default-freqtrade-image flag unless spec.app.pod.image overrides it
	// (P3-3). A floating tag would let a routine pod restart silently pick
	// up a new freqtrade version mid-trading; this makes what's actually
	// running visible regardless of which source set it.
	ResolvedImage string `json:"resolvedImage,omitempty"`

	// Bot is this bot's own live trading state, as last observed by the
	// operator's poller (P4-3) - never written by the TradeBot reconciler
	// itself, and can lag behind spec.introspection.interval seconds.
	// LastPollTime/LastPollError say how current (or not) the rest of it
	// is; the BotReachable condition is the authoritative "can we trust
	// this at all right now" signal.
	// +optional
	Bot *BotStatus `json:"bot,omitempty"`
}

// BotStatus is a snapshot of a bot's own freqtrade REST API responses
// (P4-3) - polled, never pushed by the bot itself, so every field can be
// stale by up to spec.introspection.interval.
type BotStatus struct {
	// State is freqtrade's own run state: running, stopped, or unknown
	// (the operator either hasn't polled successfully yet, or the bot's
	// api_server isn't reachable/configured - see BotReachable for why).
	// +kubebuilder:validation:Enum=running;stopped;unknown
	State string `json:"state,omitempty"`

	Version string `json:"version,omitempty"`
	DryRun  *bool  `json:"dryRun,omitempty"`

	OpenTrades    *int `json:"openTrades,omitempty"`
	MaxOpenTrades *int `json:"maxOpenTrades,omitempty"`

	// TotalProfitAbs/TotalProfitPct are strings, not floats - API types
	// don't carry floats (see api/v1alpha1's own conventions elsewhere),
	// and a value straight from freqtrade's own JSON response is passed
	// through as text rather than round-tripped through float64.
	TotalProfitAbs string `json:"totalProfitAbs,omitempty"`
	TotalProfitPct string `json:"totalProfitPct,omitempty"`

	// LastPollTime is when the poller last completed a poll attempt for
	// this bot, successful or not.
	LastPollTime *metav1.Time `json:"lastPollTime,omitempty"`
	// LastPollError is the most recent poll failure's message, or empty
	// after a successful poll.
	LastPollError string `json:"lastPollError,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Command",type=string,JSONPath=`.spec.freqtrade_command`
//+kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
//+kubebuilder:resource:shortName=tb;tbot

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
