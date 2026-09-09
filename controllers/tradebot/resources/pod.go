// controllers/tradebot/resources/pod.go
package resources

import (
	"reflect"
	"strings"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BuildPod constructs a reusable PodSpec for running Freqtrade.
// - tradeBot: source CR used for overrides (App.PodSpec) and namespacing
// - image: the freqtrade image reference to run (normally digest-pinned;
// see Reconciler.DefaultImage) - the caller resolves this once so
// TradeBotStatus.ResolvedImage can record exactly what was used (P3-3)
// - strategyName: the Python strategy name (from Strategy.Spec.Name)
// - configSecretName: Secret name for config.json
// - strategyConfigMapName: ConfigMap name for strategy script
// - pvcName: user-data PVC name; if empty, user-data is an emptyDir (used by Jobs)
// - freqCommand: "trade" for StatefulSet, anything else for Job
// - freqArgs: additional CLI arguments appended after the subcommand
func BuildPod(
	tradeBot freqtradev1alpha1.TradeBot,
	image string,
	strategyName string,
	configSecretName string,
	strategyConfigMapName string,
	pvcName string,
	freqCommand string,
	freqArgs []string,
) corev1.PodSpec {
	if strings.TrimSpace(freqCommand) == "" {
		freqCommand = "trade"
	}
	isTrade := freqCommand == "trade"
	hasCache := !isTrade && tradeBot.Spec.Data != nil && strings.TrimSpace(tradeBot.Spec.Data.PVCName) != ""

	// Common args
	args := []string{
		freqCommand,
		"--config", "/config/config.json",
		"--strategy-path", "/strategy",
		"--strategy", strategyName,
		"--db-url", "sqlite:////freqtrade/user_data/tradesv3.sqlite",
		"--logfile", "/freqtrade/user_data/logs/freqtrade.log",
	}
	// For jobs, explicitly set userdir, and if cache is present, bind datadir
	if !isTrade {
		args = append(args, "--userdir", "/freqtrade/user_data")
		if hasCache {
			args = append(args, "--datadir", "/cache")
		}
	}
	if len(freqArgs) > 0 {
		args = append(args, freqArgs...)
	}

	// Security contexts
	runAsUser := int64(1000)
	runAsGroup := int64(1000)
	fsGroup := int64(1000)

	// Base volumes
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
	}
	// user-data: PVC in trade mode (or when provided), else emptyDir (for jobs)
	userDataVol := corev1.Volume{Name: "user-data"}
	if pvcName != "" {
		userDataVol.VolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName},
		}
	} else {
		userDataVol.VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
	}
	volumes = append(volumes, userDataVol)

	// Cache volume for jobs (shared RWX PVC)
	if hasCache {
		volumes = append(volumes, corev1.Volume{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: tradeBot.Spec.Data.PVCName,
				},
			},
		})
	}

	// Base mounts for main container
	mainMounts := []corev1.VolumeMount{
		{Name: "config", MountPath: "/config", ReadOnly: true},
		{Name: "strategy", MountPath: "/strategy", ReadOnly: true},
		{Name: "user-data", MountPath: "/freqtrade/user_data"},
	}
	if hasCache {
		// ReadOnly for main workload
		mainMounts = append(mainMounts, corev1.VolumeMount{Name: "cache", MountPath: "/cache", ReadOnly: true})
	}

	// Init mounts for init containers
	initUserDataMounts := []corev1.VolumeMount{
		{Name: "user-data", MountPath: "/freqtrade/user_data"},
	}
	// For download-data, we need full set plus write access to /cache
	initDownloadMounts := []corev1.VolumeMount{
		{Name: "config", MountPath: "/config", ReadOnly: true},
		{Name: "strategy", MountPath: "/strategy", ReadOnly: true},
		{Name: "user-data", MountPath: "/freqtrade/user_data"},
	}
	if hasCache {
		initDownloadMounts = append(initDownloadMounts,
			corev1.VolumeMount{Name: "cache", MountPath: "/cache", ReadOnly: false})
	}

	fixVolumePermissions := tradeBot.Spec.App != nil && tradeBot.Spec.App.PVCSpec != nil &&
		tradeBot.Spec.App.PVCSpec.FixVolumePermissions != nil && *tradeBot.Spec.App.PVCSpec.FixVolumePermissions

	// Init containers. init-user-data only creates directories: fsGroup
	// (set on the pod's own SecurityContext below) already makes the volume
	// group-writable on most CSI drivers, so - unlike before P3-3 - it runs
	// as the same non-root user as everything else and never chmod/chowns
	// anything. FixVolumePermissions (an opt-in, not the default) restores
	// the old root-chown behavior for drivers where that's not true.
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
	initContainers := []corev1.Container{initUserData}

	// Append download-data init for jobs with cache
	if hasCache {
		dlArgs := []string{
			"download-data",
			"--userdir", "/freqtrade/user_data",
			"--datadir", "/cache",
		}
		if tradeBot.Spec.Data != nil && len(tradeBot.Spec.Data.DownloadArgs) > 0 {
			dlArgs = append(dlArgs, tradeBot.Spec.Data.DownloadArgs...)
		}
		policy := strings.ToLower(strings.TrimSpace(tradeBot.Spec.Data.DownloadPolicy))
		if policy == "" {
			policy = "always"
		}
		switch policy {
		case "never":
			// No init container: use whatever is already on the cache PVC.
		case "ifmissing":
			// Only download when the cache looks empty. dlArgs are passed as
			// positional parameters ("$@") after "--", not interpolated into
			// the script text, so nothing here can inject shell commands.
			script := `if [ -z "$(ls -A /cache 2>/dev/null)" ]; then exec freqtrade "$@"; fi`
			initContainers = append(initContainers, corev1.Container{
				Name:            "init-download-data",
				Image:           image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: shared.RestrictedSecurityContext(),
				Command:         []string{"sh", "-c", script, "init-download-data"},
				Args:            dlArgs,
				VolumeMounts:    initDownloadMounts,
			})
		default: // "always"
			initContainers = append(initContainers, corev1.Container{
				Name:            "init-download-data",
				Image:           image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: shared.RestrictedSecurityContext(),
				Command:         []string{"freqtrade"},
				Args:            dlArgs,
				VolumeMounts:    initDownloadMounts,
			})
		}
	}

	// Main container
	mainContainer := corev1.Container{
		Name: "freqtrade",
		// A floating tag on a fixed policy (the old ImagePullPolicy: PullAlways)
		// meant a routine pod restart could silently pick up a new freqtrade
		// version mid-trading. image is normally digest-pinned (see
		// Reconciler.DefaultImage), so PullIfNotPresent is both correct
		// (a digest never changes what it points to) and avoids a pointless
		// re-pull on every restart.
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"freqtrade"},
		Args:            args,
		VolumeMounts:    mainMounts,
		SecurityContext: shared.RestrictedSecurityContext(),
		Resources:       shared.DefaultContainerResources(),
	}

	// Probes and ports: only for trade
	if isTrade {
		mainContainer.Ports = []corev1.ContainerPort{
			{ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
		}
		mainContainer.LivenessProbe = &corev1.Probe{
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
		}
		mainContainer.ReadinessProbe = &corev1.Probe{
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
		}
	}

	podSpec := corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:  &runAsUser,
			RunAsGroup: &runAsGroup,
			FSGroup:    &fsGroup,
		},
		InitContainers: initContainers,
		Containers:     []corev1.Container{mainContainer},
		Volumes:        volumes,
	}

	// Jobs: set RestartPolicy to OnFailure if not provided
	if !isTrade && podSpec.RestartPolicy == "" {
		podSpec.RestartPolicy = corev1.RestartPolicyOnFailure
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
