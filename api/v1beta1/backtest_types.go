package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunSpec is the set of fields a one-shot freqtrade run needs regardless of
// which command it runs - embedded with json:",inline" (invisible in the
// serialized JSON) so Hyperopt (D7, a later minor) can reuse it with zero
// API churn on Backtest. Retrofitting this split after v1beta1 is stored
// would be a breaking change for no benefit, so it's done now even though
// Hyperopt itself isn't built yet.
type RunSpec struct {
	// ConfigRef is the TradeBotConfig this run renders config.json from,
	// same namespace only (D3).
	ConfigRef corev1.LocalObjectReference `json:"configRef"`
	// StrategyRef is the Strategy this run executes, same namespace only (D3).
	StrategyRef corev1.LocalObjectReference `json:"strategyRef"`

	// Timerange is freqtrade's --timerange value, e.g. "20230101-20230201"
	// or "20230101-" for open-ended.
	// +kubebuilder:validation:Pattern=`^\d{8}-(\d{8})?$`
	Timerange string `json:"timerange,omitempty"`
	// Timeframe is freqtrade's --timeframe value, e.g. "5m", "1h", "1d".
	// +kubebuilder:validation:Pattern=`^\d+[mhdw]$`
	Timeframe string `json:"timeframe,omitempty"`

	// +kubebuilder:validation:MaxItems=256
	Pairs []string `json:"pairs,omitempty"`

	// Data optionally mounts a shared read-only cache PVC as --datadir,
	// with an init container downloading into it first per DownloadPolicy -
	// moved here from TradeBot.Spec.Data (v1alpha1), which only ever made
	// sense for a one-shot run in the first place (see pod.go's isTrade gate
	// on it in the old package).
	Data *DataSourceSpec `json:"data,omitempty"`

	// Results configures the per-run PVC this run's output is written to
	// (D1) - the CR is the run's identity, so unlike v1alpha1's Job mode
	// there is no orphaned-volume or spec-hash-in-the-name problem to work
	// around.
	Results *ResultsSpec `json:"results,omitempty"`

	// Pod carries resource requests/limits and scheduling overrides for the
	// run's pod.
	Pod *PodSpec `json:"pod,omitempty"`

	// TTLSecondsAfterFinished reaps the finished Job (not the results PVC -
	// see ResultsSpec.RetentionPolicy, and D10 on why nothing prunes results
	// automatically) this many seconds after it completes.
	// +kubebuilder:default=86400
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`

	// ExtraArgs appends raw freqtrade CLI arguments (D8/D5): typed fields
	// above remain the documented surface for anything they cover. This is
	// a pressure valve for flags this API hasn't caught up to yet, not an
	// alternative to them - guarded by the freqtrade.io/allow-extra-args:
	// "true" annotation (rejected by the admission webhook without it) and
	// checked against a denylist of flags the operator itself controls
	// (--config, --strategy, --strategy-path, --db-url, --logfile,
	// --userdir, --datadir); the controller emits a warning Event whenever
	// it's used so its usage stays visible rather than quietly load-bearing.
	// +kubebuilder:validation:MaxItems=64
	ExtraArgs []string `json:"extraArgs,omitempty"`
}

// DataSourceSpec optionally mounts a shared, read-only data cache for a run
// (renamed from v1alpha1's DataCacheSpec, which this replaces for one-shot
// runs - see RunSpec.Data's doc comment).
type DataSourceSpec struct {
	// PVCName is the name of a pre-existing, shared RWX PVC to mount
	// read-only at /cache and pass as --datadir.
	PVCName string `json:"pvcName,omitempty"`

	// DownloadArgs are optional extra args for "freqtrade download-data",
	// e.g. []string{"--exchange", "binance", "-t", "1m", "5m", "--days", "30"}.
	DownloadArgs []string `json:"downloadArgs,omitempty"`

	// DownloadPolicy controls when the cache is refreshed before the run.
	// +kubebuilder:validation:Enum=always;ifMissing;never
	// +kubebuilder:default=always
	DownloadPolicy string `json:"downloadPolicy,omitempty"`
}

// ResultsSpec configures the PVC a run's results are written to (D1).
type ResultsSpec struct {
	// +kubebuilder:default="1Gi"
	Size resource.Quantity `json:"size,omitempty"`
	// StorageClassName defaults to the cluster's default storage class when unset.
	StorageClassName *string `json:"storageClassName,omitempty"`
	// RetentionPolicy decided once, at creation, like every other field
	// (the whole spec is immutable) - Delete GCs the results PVC when this
	// CR is deleted (the working manual reaper D10 documents); Retain
	// strips the owner reference instead, the same pattern
	// TradeBot.finalizers.go uses for freqtrade.io/preserve-data.
	// +kubebuilder:validation:Enum=Delete;Retain
	// +kubebuilder:default=Delete
	RetentionPolicy string `json:"retentionPolicy,omitempty"`
}

// PodSpec carries resource/scheduling overrides for a run's pod - a trimmed
// version of v1alpha1's PodSpec: no ServiceName (StatefulSet-only) and no
// probes (nothing polls a one-shot Job's api_server, because it never has one).
type PodSpec struct {
	Image            string                        `json:"image,omitempty"`
	Resources        corev1.ResourceRequirements   `json:"resources,omitempty"`
	Env              []corev1.EnvVar               `json:"env,omitempty"`
	VolumeMounts     []corev1.VolumeMount          `json:"volumeMounts,omitempty"`
	Volumes          []corev1.Volume               `json:"volumes,omitempty"`
	SecurityContext  *corev1.PodSecurityContext    `json:"securityContext,omitempty"`
	InitContainers   []corev1.Container            `json:"initContainers,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	Affinity                  *corev1.Affinity                  `json:"affinity,omitempty"`
	NodeSelector              map[string]string                 `json:"nodeSelector,omitempty"`
	Tolerations               []corev1.Toleration               `json:"tolerations,omitempty"`
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// BacktestSpec defines the desired state of Backtest. Immutable after
// creation in full (CEL self == oldSelf, below) - a Backtest is an
// immutable fact about a (strategy, config, timerange, data) tuple, not a
// thing you edit in place. To change any parameter, create a new Backtest.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec is immutable after creation - create a new Backtest to change any parameter"
type BacktestSpec struct {
	RunSpec `json:",inline"`

	// TimeframeDetail is freqtrade's --timeframe-detail value, for
	// sub-timeframe order execution detail.
	TimeframeDetail string `json:"timeframeDetail,omitempty"`
	// MaxOpenTrades overrides the rendered config's max_open_trades for this run.
	MaxOpenTrades *int `json:"maxOpenTrades,omitempty"`
	// StakeAmount is freqtrade's --stake-amount value: "unlimited", or a
	// quantity in the exchange's stake currency.
	StakeAmount string `json:"stakeAmount,omitempty"`
	// DryRunWallet overrides the rendered config's dry_run_wallet starting balance.
	DryRunWallet *resource.Quantity `json:"dryRunWallet,omitempty"`
	// Fee overrides the rendered config's trading fee (a fraction, e.g. "0.001").
	// A pointer so an explicit "0" is distinguishable from unset.
	Fee *string `json:"fee,omitempty"`
	// EnableProtections turns on freqtrade's --enable-protections for this run.
	EnableProtections *bool `json:"enableProtections,omitempty"`
	// Breakdown requests freqtrade's --breakdown output at these granularities.
	// +kubebuilder:validation:Enum=day;week;month
	Breakdown []string `json:"breakdown,omitempty"`
	// Cache is freqtrade's --cache value, controlling its own internal
	// signal-calculation caching across repeated backtests - unrelated to
	// RunSpec.Data, the operator's own shared data-download PVC feature.
	// +kubebuilder:validation:Enum=none;day;week;month
	Cache string `json:"cache,omitempty"`
}

// BacktestStatus defines the observed state of Backtest.
type BacktestStatus struct {
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

	JobName        string `json:"jobName,omitempty"`
	ResultsPVCName string `json:"resultsPVCName,omitempty"`

	StartTime      *metav1.Time `json:"startTime,omitempty"`
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Results is filled in once the run's sidecar has extracted them (P6-2)
	// - nil until then, and left as the last successful run's data if a
	// later parse ever fails (see ConditionResultsAvailable).
	Results *BacktestResults `json:"results,omitempty"`
}

// BacktestResults summarizes one run's outcome. All numeric results are
// strings, never float64 - no stable JSON round-trip, and every other
// numeric field across this API follows the same rule (see e.g.
// v1alpha1.BotStatus.TotalProfitAbs).
type BacktestResults struct {
	TotalTrades    int    `json:"totalTrades,omitempty"`
	ProfitAbs      string `json:"profitAbs,omitempty"`
	ProfitPct      string `json:"profitPct,omitempty"`
	WinRatePct     string `json:"winRatePct,omitempty"`
	MaxDrawdownPct string `json:"maxDrawdownPct,omitempty"`
	SharpeRatio    string `json:"sharpeRatio,omitempty"`
	SortinoRatio   string `json:"sortinoRatio,omitempty"`
	CAGRPct        string `json:"cagrPct,omitempty"`
	BestPair       string `json:"bestPair,omitempty"`
	WorstPair      string `json:"worstPair,omitempty"`
	// ResultFile is the path on the results PVC the full result JSON was
	// read from.
	ResultFile string `json:"resultFile,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategyRef.name`
//+kubebuilder:printcolumn:name="Timerange",type=string,JSONPath=`.spec.timerange`
//+kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Trades",type=integer,JSONPath=`.status.results.totalTrades`
//+kubebuilder:printcolumn:name="Profit%",type=string,JSONPath=`.status.results.profitPct`
//+kubebuilder:printcolumn:name="Drawdown%",type=string,JSONPath=`.status.results.maxDrawdownPct`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
//+kubebuilder:resource:shortName=bt

type Backtest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BacktestSpec   `json:"spec,omitempty"`
	Status BacktestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type BacktestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Backtest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Backtest{}, &BacktestList{})
}
