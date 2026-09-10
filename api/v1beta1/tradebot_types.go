package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// TradeBotSpec defines the desired state of TradeBot. Trade-only (P6-4,
// D5): FreqtradeCommand/FreqtradeArguments/Data - the whole one-shot-run
// branch - are gone, moved to Backtest (D1, P6-1). A v1alpha1 TradeBot
// using them has no v1beta1 equivalent at all; the conversion webhook
// (api/v1alpha1/tradebot_conversion.go) rejects converting one rather
// than silently dropping what it was actually configured to do.
type TradeBotSpec struct {
	// ConfigRef references the TradeBotConfig this bot renders config.json
	// from, same namespace only (D3/P6-5).
	ConfigRef corev1.LocalObjectReference `json:"configRef"`
	// StrategyRef references the Strategy this bot runs, same namespace
	// only (D3/P6-5).
	StrategyRef corev1.LocalObjectReference `json:"strategyRef"`

	App *TBAppConfig `json:"app,omitempty"`

	// UpdateStrategy controls what happens when a config change can't take
	// effect without a restart - see v1alpha1.TradeBotSpec's identical
	// field for the full explanation (P2-4).
	// +kubebuilder:validation:Enum=Manual;Auto
	// +kubebuilder:default=Manual
	UpdateStrategy string `json:"updateStrategy,omitempty"`

	// Introspection controls the operator's own polling of this bot's
	// freqtrade REST API for live trading state (P4-3, D4).
	Introspection *IntrospectionSpec `json:"introspection,omitempty"`

	// State is a deliberately later addition (P4-4, D9) - not part of this
	// phase. Bot introspection (above) stays read-only until then.
}

// TBAppConfig, PodSpec/ServiceSpec/PVCSpec, IntrospectionSpec and
// BotStatus below are field-for-field identical to their v1alpha1
// namesakes - P6-4/P6-5 only touches reference types and Job-mode fields,
// neither of which any of these have. Redeclared here rather than
// imported: v1alpha1's own conversion functions (ConvertTo/ConvertFrom)
// must live in package v1alpha1 (Go methods have to live alongside their
// receiver type), which already means api/v1alpha1 imports api/v1beta1 -
// the reverse import this package aliasing them from v1alpha1 would need
// is therefore not available; that would be a cycle. The conversion
// functions use a JSON-roundtrip helper for exactly these types
// (api/v1alpha1/tradebot_conversion.go), which is what the round-trip
// fuzz tests are for: a tag mismatch between the two copies would fail a
// fuzz round-trip immediately, not sit undetected.
//
// PodSpec here is TradeBotPodSpec, not PodSpec - api/v1beta1/backtest_types.go
// (P6-1) already uses the bare name for Backtest's own, differently-shaped
// (no probes, no ServiceName) one-shot-run pod spec.
type TBAppConfig struct {
	PodSpec     *TradeBotPodSpec `json:"pod,omitempty"`
	ServiceSpec *ServiceSpec     `json:"service,omitempty"`
	PVCSpec     *PVCSpec         `json:"pvc,omitempty"`
}

type TradeBotPodSpec struct {
	Image                     string                            `json:"image,omitempty"`
	Resources                 corev1.ResourceRequirements       `json:"resources,omitempty"`
	Env                       []corev1.EnvVar                   `json:"env,omitempty"`
	VolumeMounts              []corev1.VolumeMount              `json:"volumeMounts,omitempty"`
	Volumes                   []corev1.Volume                   `json:"volumes,omitempty"`
	SecurityContext           *corev1.PodSecurityContext        `json:"securityContext,omitempty"`
	InitContainers            []corev1.Container                `json:"initContainers,omitempty"`
	ServiceName               string                            `json:"serviceName,omitempty"`
	ImagePullSecrets          []corev1.LocalObjectReference     `json:"imagePullSecrets,omitempty"`
	LivenessProbe             *corev1.Probe                     `json:"livenessProbe,omitempty"`
	ReadinessProbe            *corev1.Probe                     `json:"readinessProbe,omitempty"`
	Affinity                  *corev1.Affinity                  `json:"affinity,omitempty"`
	AntiAffinity              *corev1.PodAntiAffinity           `json:"antiAffinity,omitempty"`
	NodeSelector              map[string]string                 `json:"nodeSelector,omitempty"`
	Tolerations               []corev1.Toleration               `json:"tolerations,omitempty"`
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

type ServiceSpec struct {
	Type        corev1.ServiceType   `json:"type,omitempty"`
	Ports       []corev1.ServicePort `json:"ports,omitempty"`
	Selector    map[string]string    `json:"selector,omitempty"`
	Annotations map[string]string    `json:"annotations,omitempty"`
}

type PVCSpec struct {
	AccessModes          []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
	StorageSize          *resource.Quantity                  `json:"storageSize,omitempty"`
	StorageClassName     string                              `json:"storageClassName,omitempty"`
	VolumeName           string                              `json:"volumeName,omitempty"`
	Annotations          map[string]string                   `json:"annotations,omitempty"`
	Labels               map[string]string                   `json:"labels,omitempty"`
	FixVolumePermissions *bool                               `json:"fixVolumePermissions,omitempty"`
}

// IntrospectionSpec controls whether and how often the operator polls
// this bot's own freqtrade REST API (P4-3) - identical to v1alpha1's.
type IntrospectionSpec struct {
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`
	// +kubebuilder:default="60s"
	Interval metav1.Duration `json:"interval,omitempty"`
}

// BotStatus is a snapshot of a bot's own freqtrade REST API responses
// (P4-3) - identical to v1alpha1's.
type BotStatus struct {
	// +kubebuilder:validation:Enum=running;stopped;unknown
	State          string       `json:"state,omitempty"`
	Version        string       `json:"version,omitempty"`
	DryRun         *bool        `json:"dryRun,omitempty"`
	OpenTrades     *int         `json:"openTrades,omitempty"`
	MaxOpenTrades  *int         `json:"maxOpenTrades,omitempty"`
	TotalProfitAbs string       `json:"totalProfitAbs,omitempty"`
	TotalProfitPct string       `json:"totalProfitPct,omitempty"`
	LastPollTime   *metav1.Time `json:"lastPollTime,omitempty"`
	LastPollError  string       `json:"lastPollError,omitempty"`
}

// TradeBotStatus defines the observed state of TradeBot.
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
	// was computed from.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// AppliedConfigHash is sha256(config.json bytes + strategy script
	// bytes), truncated to 16 hex characters - see v1alpha1.TradeBotStatus's
	// identical field.
	AppliedConfigHash string `json:"appliedConfigHash,omitempty"`

	// ResolvedImage is the exact freqtrade image reference the running
	// workload was built with (P3-3).
	ResolvedImage string `json:"resolvedImage,omitempty"`

	// Bot is this bot's own live trading state, as last observed by the
	// operator's poller (P4-3).
	// +optional
	Bot *BotStatus `json:"bot,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategyRef.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=tb;tbot

type TradeBot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TradeBotSpec   `json:"spec,omitempty"`
	Status TradeBotStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type TradeBotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TradeBot `json:"items"`
}

// Hub marks TradeBot as the conversion hub (controller-runtime's
// conversion.Hub interface) - v1beta1 is the storage version (above), so
// v1alpha1.TradeBot converts to/from this directly rather than through an
// intermediate; see api/v1alpha1/tradebot_conversion.go.
func (*TradeBot) Hub() {}

var _ conversion.Hub = &TradeBot{}

func init() {
	SchemeBuilder.Register(&TradeBot{}, &TradeBotList{})
}
