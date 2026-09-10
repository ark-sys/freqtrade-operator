// controllers/tradebot/resources/pod.go
package resources

import (
	"reflect"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// freqCommandTrade is the freqtrade subcommand BuildPod always runs -
// TradeBot has been trade-only since v1beta1 (P6-4); Backtest's own,
// unrelated pod builder lives in controllers/backtest/resources.
const freqCommandTrade = "trade"

// userDataVolumeName is the shared volume the main container and
// init-user-data both mount at /freqtrade/user_data.
const userDataVolumeName = "user-data"

// BuildPod constructs the PodSpec for a TradeBot's long-running trading
// StatefulSet pod.
// - tradeBot: source CR used for overrides (App.PodSpec) and namespacing
// - image: the freqtrade image reference to run (normally digest-pinned;
// see Reconciler.DefaultImage) - the caller resolves this once so
// TradeBotStatus.ResolvedImage can record exactly what was used (P3-3)
// - strategyName: the Python strategy name (from Strategy.Spec.Name)
// - configSecretName: Secret name for config.json
// - strategyConfigMapName: ConfigMap name for strategy script
// - pvcName: user-data PVC name, always set - a TradeBot's PVC is created
// unconditionally (see resources.BuildPersistentVolumeClaim)
// - freqArgs: additional CLI arguments appended after the subcommand
func BuildPod(
	tradeBot freqtradev1alpha1.TradeBot,
	image string,
	strategyName string,
	configSecretName string,
	strategyConfigMapName string,
	pvcName string,
	freqArgs []string,
) corev1.PodSpec {
	args := []string{
		freqCommandTrade,
		"--config", "/config/config.json",
		"--strategy-path", "/strategy",
		"--strategy", strategyName,
		"--db-url", "sqlite:////freqtrade/user_data/tradesv3.sqlite",
		"--logfile", "/freqtrade/user_data/logs/freqtrade.log",
	}
	if len(freqArgs) > 0 {
		args = append(args, freqArgs...)
	}

	// Security contexts
	runAsUser := int64(1000)
	runAsGroup := int64(1000)
	fsGroup := int64(1000)

	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: configSecretName},
			},
		},
		{
			Name: "strategy",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: strategyConfigMapName},
				},
			},
		},
		{
			Name: userDataVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
			},
		},
	}

	mainMounts := []corev1.VolumeMount{
		{Name: "config", MountPath: "/config", ReadOnly: true},
		{Name: "strategy", MountPath: "/strategy", ReadOnly: true},
		{Name: userDataVolumeName, MountPath: "/freqtrade/user_data"},
	}

	initUserDataMounts := []corev1.VolumeMount{
		{Name: userDataVolumeName, MountPath: "/freqtrade/user_data"},
	}

	fixVolumePermissions := tradeBot.Spec.App != nil && tradeBot.Spec.App.PVCSpec != nil &&
		tradeBot.Spec.App.PVCSpec.FixVolumePermissions != nil && *tradeBot.Spec.App.PVCSpec.FixVolumePermissions

	// init-user-data only creates directories: fsGroup (set on the pod's own
	// SecurityContext below) already makes the volume group-writable on most
	// CSI drivers, so - unlike before P3-3 - it runs as the same non-root
	// user as everything else and never chmod/chowns anything.
	// FixVolumePermissions (an opt-in, not the default) restores the old
	// root-chown behavior for drivers where that's not true.
	var initUserData corev1.Container
	if fixVolumePermissions {
		runAsRoot := int64(0)
		initUserData = corev1.Container{
			Name:            "init-user-data",
			Image:           "busybox:latest",
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &corev1.SecurityContext{RunAsUser: &runAsRoot, RunAsGroup: &runAsRoot},
			Command:         []string{"sh", "-c"},
			Args: []string{
				"mkdir -p /freqtrade/user_data/logs /freqtrade/user_data/data && " +
					"chmod -R 775 /freqtrade/user_data && chown -R 1000:1000 /freqtrade/user_data",
			},
			VolumeMounts: initUserDataMounts,
		}
	} else {
		initUserData = corev1.Container{
			Name:            "init-user-data",
			Image:           "busybox:latest",
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: shared.RestrictedSecurityContext(),
			Command:         []string{"sh", "-c"},
			Args:            []string{"mkdir -p /freqtrade/user_data/logs /freqtrade/user_data/data"},
			VolumeMounts:    initUserDataMounts,
		}
	}

	// Main container
	mainContainer := corev1.Container{
		Name: freqtradeAppName,
		// A floating tag on a fixed policy (the old ImagePullPolicy: PullAlways)
		// meant a routine pod restart could silently pick up a new freqtrade
		// version mid-trading. image is normally digest-pinned (see
		// Reconciler.DefaultImage), so PullIfNotPresent is both correct
		// (a digest never changes what it points to) and avoids a pointless
		// re-pull on every restart.
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{freqtradeAppName},
		Args:            args,
		VolumeMounts:    mainMounts,
		SecurityContext: shared.RestrictedSecurityContext(),
		Resources:       shared.DefaultContainerResources(),
		Ports: []corev1.ContainerPort{
			{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/api/v1/ping",
					Port:   intstr.FromInt32(8080),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 60,
			PeriodSeconds:       60,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/api/v1/ping",
					Port:   intstr.FromInt32(8080),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       30,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
		},
	}

	podSpec := corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:  &runAsUser,
			RunAsGroup: &runAsGroup,
			FSGroup:    &fsGroup,
		},
		InitContainers: []corev1.Container{initUserData},
		Containers:     []corev1.Container{mainContainer},
		Volumes:        volumes,
	}

	// Apply optional App.PodSpec overrides
	if tradeBot.Spec.App != nil && tradeBot.Spec.App.PodSpec != nil {
		podSpec = mergePodSpecOverrides(podSpec, tradeBot.Spec.App.PodSpec)
	}

	return podSpec
}

// mergePodSpecOverrides merges user overrides from App.PodSpec into the default PodSpec.
func mergePodSpecOverrides(defaultPodSpec corev1.PodSpec, override *freqtradev1alpha1.PodSpec) corev1.PodSpec {
	if override == nil {
		return defaultPodSpec
	}

	if override.Image != "" {
		defaultPodSpec.Containers[0].Image = override.Image
	}

	// Only override fields that are non-zero in the override struct
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
	if override.LivenessProbe != nil {
		defaultPodSpec.Containers[0].LivenessProbe = override.LivenessProbe
	}
	if override.ReadinessProbe != nil {
		defaultPodSpec.Containers[0].ReadinessProbe = override.ReadinessProbe
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
