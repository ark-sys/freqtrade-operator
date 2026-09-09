// controllers/backtest/resources/pod.go
package resources

import (
	"reflect"
	"strconv"
	"strings"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// buildArgs translates BacktestSpec's typed fields into freqtrade's own CLI
// flags (D5: typed fields replace free-form arguments) - the one place this
// mapping lives, so BuildPod itself stays a pure assembly function.
// extraArgs is passed separately (rather than read from spec directly) so
// tests can exercise the mapping without also exercising webhook validation.
func buildArgs(spec freqtradev1beta1.BacktestSpec, strategyName string, hasCache bool, extraArgs []string) []string {
	args := []string{
		"backtesting",
		"--config", "/config/config.json",
		"--strategy-path", "/strategy",
		"--strategy", strategyName,
		"--userdir", "/freqtrade/user_data",
		"--logfile", "/freqtrade/user_data/logs/freqtrade.log",
	}
	if hasCache {
		args = append(args, "--datadir", "/cache")
	}
	if spec.Timerange != "" {
		args = append(args, "--timerange", spec.Timerange)
	}
	if spec.Timeframe != "" {
		args = append(args, "--timeframe", spec.Timeframe)
	}
	if spec.TimeframeDetail != "" {
		args = append(args, "--timeframe-detail", spec.TimeframeDetail)
	}
	if len(spec.Pairs) > 0 {
		args = append(args, "--pairs")
		args = append(args, spec.Pairs...)
	}
	if spec.MaxOpenTrades != nil {
		args = append(args, "--max-open-trades", strconv.Itoa(*spec.MaxOpenTrades))
	}
	if spec.StakeAmount != "" {
		args = append(args, "--stake-amount", spec.StakeAmount)
	}
	if spec.DryRunWallet != nil {
		// AsDec().String(), not String(): Quantity.String() prefers SI
		// suffixes (1000 -> "1k"), which freqtrade's own CLI parser doesn't
		// understand for a wallet balance - AsDec gives the exact plain
		// decimal the value was written as.
		args = append(args, "--dry-run-wallet", spec.DryRunWallet.AsDec().String())
	}
	if spec.Fee != nil {
		args = append(args, "--fee", *spec.Fee)
	}
	if spec.EnableProtections != nil && *spec.EnableProtections {
		args = append(args, "--enable-protections")
	}
	if len(spec.Breakdown) > 0 {
		args = append(args, "--breakdown")
		args = append(args, spec.Breakdown...)
	}
	if spec.Cache != "" {
		args = append(args, "--cache", spec.Cache)
	}
	// extraArgs last (D8) - already validated against a denylist of every
	// flag set above, by the admission webhook, before this ever runs.
	args = append(args, extraArgs...)
	return args
}

// BuildPod constructs the PodSpec for a Backtest run. Unlike
// controllers/tradebot's BuildPod, there is no trade-vs-job branch here:
// every Backtest is a one-shot run, full stop - no probes, no ports, no
// long-lived user-data PVC (user_data is an emptyDir; only the results PVC
// this run's own sidecar/output writes to, mounted separately by BuildJob,
// outlives the pod).
func BuildPod(
	backtest freqtradev1beta1.Backtest, image, strategyName, configSecretName, strategyConfigMapName string,
) corev1.PodSpec {
	spec := backtest.Spec
	hasCache := spec.Data != nil && strings.TrimSpace(spec.Data.PVCName) != ""

	volumes := []corev1.Volume{
		{
			Name:         "config",
			VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: configSecretName}},
		},
		{
			Name: "strategy",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: strategyConfigMapName},
				},
			},
		},
		{Name: "user-data", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
	}
	if hasCache {
		volumes = append(volumes, corev1.Volume{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: spec.Data.PVCName},
			},
		})
	}

	mainMounts := []corev1.VolumeMount{
		{Name: "config", MountPath: "/config", ReadOnly: true},
		{Name: "strategy", MountPath: "/strategy", ReadOnly: true},
		{Name: "user-data", MountPath: "/freqtrade/user_data"},
	}
	initMounts := append([]corev1.VolumeMount{}, mainMounts...)
	if hasCache {
		mainMounts = append(mainMounts, corev1.VolumeMount{Name: "cache", MountPath: "/cache", ReadOnly: true})
		initMounts = append(initMounts, corev1.VolumeMount{Name: "cache", MountPath: "/cache", ReadOnly: false})
	}

	initContainers := []corev1.Container{{
		Name:            "init-user-data",
		Image:           "busybox:latest",
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: shared.RestrictedSecurityContext(),
		Command:         []string{"sh", "-c"},
		Args:            []string{"mkdir -p /freqtrade/user_data/logs /freqtrade/user_data/data"},
		VolumeMounts:    []corev1.VolumeMount{{Name: "user-data", MountPath: "/freqtrade/user_data"}},
	}}
	if hasCache {
		initContainers = append(initContainers, buildDownloadDataInitContainer(spec, image, initMounts))
	}

	extraArgs := spec.ExtraArgs

	mainContainer := corev1.Container{
		Name:            "freqtrade",
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"freqtrade"},
		Args:            buildArgs(spec, strategyName, hasCache, extraArgs),
		VolumeMounts:    mainMounts,
		SecurityContext: shared.RestrictedSecurityContext(),
		Resources:       shared.DefaultContainerResources(),
	}

	runAsUser, runAsGroup, fsGroup := int64(1000), int64(1000), int64(1000)
	podSpec := corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser: &runAsUser, RunAsGroup: &runAsGroup, FSGroup: &fsGroup,
		},
		// A Backtest's pod never needs to talk to the Kubernetes API itself
		// (P6-1 has no sidecar yet; P6-2's does, and mounts its own token via
		// a dedicated narrow-RBAC ServiceAccount instead of this default).
		AutomountServiceAccountToken: ptr.To(false),
		RestartPolicy:                corev1.RestartPolicyNever,
		InitContainers:               initContainers,
		Containers:                   []corev1.Container{mainContainer},
		Volumes:                      volumes,
	}

	if spec.Pod != nil {
		podSpec = mergePodSpecOverrides(podSpec, spec.Pod)
	}
	return podSpec
}

func buildDownloadDataInitContainer(
	spec freqtradev1beta1.BacktestSpec, image string, mounts []corev1.VolumeMount,
) corev1.Container {
	dlArgs := []string{"download-data", "--userdir", "/freqtrade/user_data", "--datadir", "/cache"}
	if spec.Data != nil && len(spec.Data.DownloadArgs) > 0 {
		dlArgs = append(dlArgs, spec.Data.DownloadArgs...)
	}
	policy := strings.ToLower(strings.TrimSpace(spec.Data.DownloadPolicy))
	if policy == "" {
		policy = "always"
	}

	base := corev1.Container{
		Name:            "init-download-data",
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: shared.RestrictedSecurityContext(),
		VolumeMounts:    mounts,
	}
	switch policy {
	case "never":
		// Use whatever is already on the cache PVC - main container still
		// runs; this init container becomes a no-op rather than being
		// omitted, so the pod's container list stays predictable regardless
		// of policy.
		base.Command = []string{"true"}
		return base
	case "ifmissing":
		// dlArgs are passed as positional parameters ("$@") after "--", not
		// interpolated into the script text, so nothing here can inject
		// shell commands.
		base.Command = []string{"sh", "-c",
			`if [ -z "$(ls -A /cache 2>/dev/null)" ]; then exec freqtrade "$@"; fi`, "init-download-data"}
		base.Args = dlArgs
		return base
	default: // "always"
		base.Command = []string{"freqtrade"}
		base.Args = dlArgs
		return base
	}
}

// mergePodSpecOverrides merges user overrides from spec.pod into the default PodSpec.
func mergePodSpecOverrides(defaultPodSpec corev1.PodSpec, override *freqtradev1beta1.PodSpec) corev1.PodSpec {
	if override.Image != "" {
		defaultPodSpec.Containers[0].Image = override.Image
	}
	if !reflect.DeepEqual(override.Resources, corev1.ResourceRequirements{}) {
		defaultPodSpec.Containers[0].Resources = override.Resources
	}
	if len(override.Env) > 0 {
		defaultPodSpec.Containers[0].Env = override.Env
	}
	if len(override.VolumeMounts) > 0 {
		defaultPodSpec.Containers[0].VolumeMounts = override.VolumeMounts
	}
	if len(override.Volumes) > 0 {
		defaultPodSpec.Volumes = override.Volumes
	}
	if override.SecurityContext != nil {
		defaultPodSpec.SecurityContext = override.SecurityContext
	}
	if len(override.InitContainers) > 0 {
		defaultPodSpec.InitContainers = override.InitContainers
	}
	if len(override.ImagePullSecrets) > 0 {
		defaultPodSpec.ImagePullSecrets = override.ImagePullSecrets
	}
	if override.Affinity != nil {
		defaultPodSpec.Affinity = override.Affinity
	}
	if len(override.NodeSelector) > 0 {
		defaultPodSpec.NodeSelector = override.NodeSelector
	}
	if len(override.Tolerations) > 0 {
		defaultPodSpec.Tolerations = override.Tolerations
	}
	if len(override.TopologySpreadConstraints) > 0 {
		defaultPodSpec.TopologySpreadConstraints = override.TopologySpreadConstraints
	}
	return defaultPodSpec
}
