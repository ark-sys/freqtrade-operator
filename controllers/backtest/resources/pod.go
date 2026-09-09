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
// every Backtest is a one-shot run, full stop - no probes, no ports.
// user-data is an emptyDir and dies with the pod; the results PVC (always
// mounted, RW, into the P6-2 sidecar only) is what actually outlives it -
// the sidecar copies the run's own raw result file there before writing
// its extracted summary ConfigMap. operatorImage is the operator's own
// image (not the freqtrade one) - it runs the results sidecar via the
// same binary, `/manager collect-results ...` (P6-2), so this never has
// to build, scan, or release a second image for it.
func BuildPod(
	backtest freqtradev1beta1.Backtest, image, operatorImage, strategyName, configSecretName, strategyConfigMapName string,
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
		{
			Name: "results",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: ResultsPVCName(backtest.Name)},
			},
		},
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
	volumes = append(volumes, sidecarTokenVolume())
	initContainers = append(initContainers, buildSidecarContainer(operatorImage, backtest.Name, strategyName))

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
		ServiceAccountName: SidecarServiceAccountName,
		// The pod-wide automount is off regardless: the freqtrade container
		// has no business holding any token, however narrow, and the sidecar
		// gets its own via sidecarTokenVolume's hand-built projection instead
		// (mounted only on that one container).
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
	dlArgs := []string{
		"download-data", "--config", "/config/config.json",
		"--userdir", "/freqtrade/user_data", "--datadir", "/cache",
	}
	// Mirrors buildArgs' own handling of these three fields: download-data has to fetch the
	// same (timerange, timeframe, pairs) the run itself will ask backtesting for, or the cache
	// ends up holding data that simply doesn't cover what the run needs. Verified directly: a
	// first pass leaving these out downloaded freqtrade's own default window (ending "now"),
	// which silently didn't overlap a historical --timerange at all - "No history ... found. No
	// data found. Terminating", despite download-data itself exiting 0 with data on disk.
	if spec.Timerange != "" {
		dlArgs = append(dlArgs, "--timerange", spec.Timerange)
	}
	if spec.Timeframe != "" {
		dlArgs = append(dlArgs, "-t", spec.Timeframe)
	}
	if len(spec.Pairs) > 0 {
		dlArgs = append(dlArgs, "--pairs")
		dlArgs = append(dlArgs, spec.Pairs...)
	}
	// DownloadArgs last (D8-style pressure valve, same as RunSpec.ExtraArgs) - anything here
	// can still override/extend the baseline above, e.g. --days or --exchange for cases the
	// typed fields don't cover.
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

// sidecarTokenServiceAccountMountPath is where kubelet's own automatic
// ServiceAccount token projection normally lands - mounting the sidecar's
// hand-built projection at the identical path means its own Go code can
// use client-go's ordinary in-cluster config (rest.InClusterConfig(),
// what ctrl.GetConfigOrDie() calls) unmodified, with no custom path
// plumbing of its own to get wrong.
const sidecarTokenServiceAccountMountPath = "/var/run/secrets/kubernetes.io/serviceaccount"

// sidecarTokenVolume hand-builds the same projected volume kubelet's own
// automatic ServiceAccount token admission would produce, so it can be
// mounted into only the sidecar container - not the whole pod via
// automountServiceAccountToken, which BuildPod deliberately leaves off
// (the freqtrade container itself has no business holding any token at
// all, however narrow). kube-root-ca.crt is the well-known ConfigMap every
// namespace carries with the cluster's own CA bundle - the same one
// automatic projection itself reads from.
//
// Caveat (P6-2, unverified): this exact mechanism has not been exercised
// against a real cluster. envtest can't cover it either - its control
// plane runs no root-ca-cert-publisher controller, so kube-root-ca.crt
// never exists there regardless of which mounting approach is used.
func sidecarTokenVolume() corev1.Volume {
	expirationSeconds := int64(3607)
	return corev1.Volume{
		Name: "sidecar-token",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources: []corev1.VolumeProjection{
					{
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Path: "token", ExpirationSeconds: &expirationSeconds,
						},
					},
					{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{Name: "kube-root-ca.crt"},
							Items:                []corev1.KeyToPath{{Key: "ca.crt", Path: "ca.crt"}},
						},
					},
					{
						DownwardAPI: &corev1.DownwardAPIProjection{
							Items: []corev1.DownwardAPIVolumeFile{{
								Path:     "namespace",
								FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
							}},
						},
					},
				},
			},
		},
	}
}

// ResultsPVCMountPath is where the sidecar mounts the results PVC (RW) to copy the run's own raw
// result file onto it - exported so cmd/main.go's runCollectResults can default --results-pvc-dir
// to this exact value instead of an independently hardcoded copy that could drift out of sync.
const ResultsPVCMountPath = "/results"

// buildSidecarContainer is the P6-2 results-collection sidecar: a native
// sidecar (RestartPolicy: Always on an init container entry, GA since
// Kubernetes 1.29 - see the README's note on the version floor this
// implies) that waits for the freqtrade container to exit, reads its
// result file off the shared user-data volume, copies it onto the results
// PVC (durable beyond the Job pod's own lifetime, unlike user-data's
// emptyDir), and writes a summary ConfigMap the operator's own reconcile
// loop later adopts and parses. Runs the operator's own image/binary
// rather than a second one to build and release.
func buildSidecarContainer(operatorImage, backtestName, strategyName string) corev1.Container {
	always := corev1.ContainerRestartPolicyAlways
	return corev1.Container{
		Name:            "collect-results",
		Image:           operatorImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		RestartPolicy:   &always,
		Args: []string{
			"collect-results",
			"--backtest-name", backtestName,
			"--strategy-name", strategyName,
			"--results-pvc-dir", ResultsPVCMountPath,
		},
		Env: []corev1.EnvVar{
			{Name: "POD_NAME", ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
			}},
			{Name: "POD_NAMESPACE", ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
			}},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "user-data", MountPath: "/freqtrade/user_data", ReadOnly: true},
			{Name: "sidecar-token", MountPath: sidecarTokenServiceAccountMountPath, ReadOnly: true},
			{Name: "results", MountPath: ResultsPVCMountPath},
		},
		SecurityContext: shared.RestrictedSecurityContext(),
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
