package resources

import (
	"strings"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// testImage stands in for a resolved freqtrade image reference across this
// package's tests - what value it holds doesn't matter to any of them.
const testImage = "freqtradeorg/freqtrade@sha256:test"

func volumeNamed(spec corev1.PodSpec, name string) *corev1.Volume {
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == name {
			return &spec.Volumes[i]
		}
	}
	return nil
}

func initContainerNamed(spec corev1.PodSpec, name string) *corev1.Container {
	for i := range spec.InitContainers {
		if spec.InitContainers[i].Name == name {
			return &spec.InitContainers[i]
		}
	}
	return nil
}

func TestBuildPod_TradeMode(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

	spec := BuildPod(
		tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", "trade", nil,
	)

	container := spec.Containers[0]
	if container.Args[0] != "trade" {
		t.Errorf("expected first arg 'trade', got %v", container.Args)
	}
	if containsArg(container.Args, "--userdir") {
		t.Errorf("trade mode should not set --userdir explicitly, got %v", container.Args)
	}
	if container.LivenessProbe == nil || container.ReadinessProbe == nil {
		t.Error("expected trade mode to set liveness and readiness probes")
	}
	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 8080 {
		t.Errorf("expected port 8080 exposed in trade mode, got %+v", container.Ports)
	}
	v := volumeNamed(spec, "user-data")
	if v == nil || v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName != "my-bot-data" {
		t.Errorf("expected user-data backed by the given PVC, got %+v", v)
	}
	if initContainerNamed(spec, "init-download-data") != nil {
		t.Error("trade mode should never add a download-data init container")
	}
}

func TestBuildPod_JobModeNoCache(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

	freqArgs := []string{"--timerange", "20240101-"}
	spec := BuildPod(
		tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "", "backtesting", freqArgs,
	)

	container := spec.Containers[0]
	if container.Args[0] != "backtesting" {
		t.Errorf("expected first arg 'backtesting', got %v", container.Args)
	}
	if !containsArg(container.Args, "--userdir") {
		t.Errorf("job mode must set --userdir explicitly, got %v", container.Args)
	}
	if containsArg(container.Args, "--datadir") {
		t.Errorf("job mode without a cache PVC should not set --datadir, got %v", container.Args)
	}
	if !containsArg(container.Args, "--timerange") {
		t.Errorf("expected extra freqArgs to be appended, got %v", container.Args)
	}
	if container.LivenessProbe != nil || container.ReadinessProbe != nil {
		t.Error("job mode should not set probes - nothing serves the API")
	}
	if v := volumeNamed(spec, "user-data"); v == nil || v.EmptyDir == nil {
		t.Errorf("expected user-data to be an emptyDir with no PVC name, got %+v", v)
	}
	if v := volumeNamed(spec, "cache"); v != nil {
		t.Errorf("expected no cache volume without Spec.Data, got %+v", v)
	}
	if spec.RestartPolicy != corev1.RestartPolicyOnFailure {
		t.Errorf("expected RestartPolicy OnFailure by default in job mode, got %q", spec.RestartPolicy)
	}
}

func TestBuildPod_JobModeWithCache(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			Data: &freqtradev1alpha1.DataCacheSpec{
				PVCName:      "shared-cache",
				DownloadArgs: []string{"--exchange", "binance"},
			},
		},
	}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "", "backtesting", nil)

	container := spec.Containers[0]
	if !containsArg(container.Args, "--datadir") {
		t.Errorf("expected --datadir when a cache PVC is configured, got %v", container.Args)
	}

	cacheVol := volumeNamed(spec, "cache")
	if cacheVol == nil || cacheVol.PersistentVolumeClaim == nil || cacheVol.PersistentVolumeClaim.ClaimName != "shared-cache" {
		t.Fatalf("expected a cache volume backed by the shared-cache PVC, got %+v", cacheVol)
	}

	found := false
	for _, m := range container.VolumeMounts {
		if m.Name == "cache" {
			found = true
			if !m.ReadOnly {
				t.Error("expected the main container's cache mount to be read-only")
			}
		}
	}
	if !found {
		t.Error("expected the main container to mount the cache volume")
	}

	dl := initContainerNamed(spec, "init-download-data")
	if dl == nil {
		t.Fatal("expected a download-data init container when a cache PVC is configured with the default policy")
	}
	if !containsArg(dl.Args, "--exchange") {
		t.Errorf("expected DownloadArgs to be appended to the download-data init container, got %v", dl.Args)
	}
	for _, m := range dl.VolumeMounts {
		if m.Name == "cache" && m.ReadOnly {
			t.Error("expected the download-data init container's cache mount to be writable")
		}
	}
}

func TestBuildPod_DownloadPolicyNeverSkipsDownload(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			Data: &freqtradev1alpha1.DataCacheSpec{PVCName: "shared-cache", DownloadPolicy: "never"},
		},
	}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "", "backtesting", nil)

	if initContainerNamed(spec, "init-download-data") != nil {
		t.Error("expected DownloadPolicy=never to skip the download-data init container")
	}
	// The cache volume itself must still be mounted even when the download
	// step is skipped, so a pre-populated cache can still be used.
	if volumeNamed(spec, "cache") == nil {
		t.Error("expected the cache volume to still be present with DownloadPolicy=never")
	}
}

// TestBuildPod_DownloadPolicyIfMissingOnlyDownloadsWhenEmpty covers the real
// "ifMissing" semantics: previously this policy value existed in the API
// but behaved identically to "always" (P1-1 - "do not ship a lie"). The
// init container now wraps the freqtrade invocation in a shell conditional;
// dlArgs are passed as positional parameters after the script text ($@),
// never interpolated into it, so nothing here is a shell-injection risk.
func TestBuildPod_DownloadPolicyIfMissingOnlyDownloadsWhenEmpty(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			Data: &freqtradev1alpha1.DataCacheSpec{PVCName: "shared-cache", DownloadPolicy: "ifMissing"},
		},
	}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "", "backtesting", nil)

	dl := initContainerNamed(spec, "init-download-data")
	if dl == nil {
		t.Fatal("expected a download-data init container with DownloadPolicy=ifMissing")
	}
	if len(dl.Command) < 3 || dl.Command[0] != "sh" || dl.Command[1] != "-c" {
		t.Fatalf("expected a shell conditional, got Command=%v", dl.Command)
	}
	if !strings.Contains(dl.Command[2], "ls -A /cache") {
		t.Errorf("expected the script to check whether /cache is empty, got %q", dl.Command[2])
	}
	if !strings.Contains(dl.Command[2], `"$@"`) {
		t.Errorf("expected the script to forward args via \"$@\" rather than interpolating them, got %q", dl.Command[2])
	}
	if !containsArg(dl.Args, "download-data") {
		t.Errorf("expected the freqtrade subcommand to be passed as a positional arg, got %v", dl.Args)
	}
}

func TestBuildPod_AppliesPodSpecOverrides(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			App: &freqtradev1alpha1.TBAppConfig{
				PodSpec: &freqtradev1alpha1.PodSpec{
					Image: "custom/freqtrade:latest",
					Env:   []corev1.EnvVar{{Name: "TZ", Value: "UTC"}},
				},
			},
		},
	}

	spec := BuildPod(
		tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", "trade", nil,
	)

	if spec.Containers[0].Image != "custom/freqtrade:latest" {
		t.Errorf("expected the overridden image to apply, got %q", spec.Containers[0].Image)
	}
	if len(spec.Containers[0].Env) != 1 || spec.Containers[0].Env[0].Name != "TZ" {
		t.Errorf("expected the overridden env vars to apply, got %+v", spec.Containers[0].Env)
	}
}

func TestMergePodSpecOverrides_NilOverrideIsNoOp(t *testing.T) {
	defaultSpec := corev1.PodSpec{Containers: []corev1.Container{{Name: "freqtrade", Image: "default:image"}}}

	got := mergePodSpecOverrides(defaultSpec, nil)

	if got.Containers[0].Image != "default:image" {
		t.Errorf("expected the default spec unchanged, got %+v", got)
	}
}

func TestMergePodSpecOverrides_EveryFieldOverridden(t *testing.T) {
	defaultSpec := corev1.PodSpec{
		Containers: []corev1.Container{{Name: "freqtrade", Image: "default:image"}},
	}
	override := &freqtradev1alpha1.PodSpec{
		Image:            "override:image",
		Resources:        corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}},
		Env:              []corev1.EnvVar{{Name: "TZ", Value: "UTC"}},
		VolumeMounts:     []corev1.VolumeMount{{Name: "extra", MountPath: "/extra"}},
		Volumes:          []corev1.Volume{{Name: "extra"}},
		SecurityContext:  &corev1.PodSecurityContext{RunAsNonRoot: ptrBoolPod(true)},
		InitContainers:   []corev1.Container{{Name: "custom-init"}},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "regcred"}},
		LivenessProbe:    &corev1.Probe{ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"true"}}}},
		ReadinessProbe:   &corev1.Probe{ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"true"}}}},
		Affinity:         &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{}},
		NodeSelector:     map[string]string{"disktype": "ssd"},
		Tolerations:      []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists}},
		TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "zone"},
		},
	}

	got := mergePodSpecOverrides(defaultSpec, override)

	c := got.Containers[0]
	if c.Image != "override:image" {
		t.Errorf("Image not overridden: %q", c.Image)
	}
	if c.Resources.Limits.Cpu().String() != "1" {
		t.Errorf("Resources not overridden: %+v", c.Resources)
	}
	if len(c.Env) != 1 || c.Env[0].Name != "TZ" {
		t.Errorf("Env not overridden: %+v", c.Env)
	}
	if len(c.VolumeMounts) != 1 || c.VolumeMounts[0].Name != "extra" {
		t.Errorf("VolumeMounts not overridden: %+v", c.VolumeMounts)
	}
	if len(got.Volumes) != 1 || got.Volumes[0].Name != "extra" {
		t.Errorf("Volumes not overridden: %+v", got.Volumes)
	}
	if got.SecurityContext == nil || got.SecurityContext.RunAsNonRoot == nil || !*got.SecurityContext.RunAsNonRoot {
		t.Errorf("SecurityContext not overridden: %+v", got.SecurityContext)
	}
	if len(got.InitContainers) != 1 || got.InitContainers[0].Name != "custom-init" {
		t.Errorf("InitContainers not overridden: %+v", got.InitContainers)
	}
	if len(got.ImagePullSecrets) != 1 || got.ImagePullSecrets[0].Name != "regcred" {
		t.Errorf("ImagePullSecrets not overridden: %+v", got.ImagePullSecrets)
	}
	if c.LivenessProbe == nil || c.ReadinessProbe == nil {
		t.Error("LivenessProbe/ReadinessProbe not overridden")
	}
	if got.Affinity == nil || got.Affinity.NodeAffinity == nil {
		t.Error("Affinity not overridden")
	}
	if got.NodeSelector["disktype"] != "ssd" {
		t.Errorf("NodeSelector not overridden: %v", got.NodeSelector)
	}
	if len(got.Tolerations) != 1 || got.Tolerations[0].Key != "dedicated" {
		t.Errorf("Tolerations not overridden: %+v", got.Tolerations)
	}
	if len(got.TopologySpreadConstraints) != 1 {
		t.Errorf("TopologySpreadConstraints not overridden: %+v", got.TopologySpreadConstraints)
	}
}

func ptrBoolPod(b bool) *bool { return &b }

func TestMergePodSpecOverrides_OnlyOverridesSetFields(t *testing.T) {
	defaultSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:      "freqtrade",
			Image:     "default:image",
			Resources: corev1.ResourceRequirements{},
		}},
		NodeSelector: map[string]string{"default": "true"},
	}

	got := mergePodSpecOverrides(defaultSpec, &freqtradev1alpha1.PodSpec{
		Image: "override:image",
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m")},
		},
	})

	if got.Containers[0].Image != "override:image" {
		t.Errorf("expected image overridden, got %q", got.Containers[0].Image)
	}
	if got.NodeSelector["default"] != "true" {
		t.Errorf("expected NodeSelector left untouched since no override was given, got %v", got.NodeSelector)
	}
}

// assertRestrictedSecurityContext fails t unless sc satisfies
// pod-security.kubernetes.io/enforce=restricted (P3-3) - verified
// empirically that freqtrade itself starts cleanly under exactly this.
func assertRestrictedSecurityContext(t *testing.T, containerName string, sc *corev1.SecurityContext) {
	t.Helper()
	if sc == nil {
		t.Fatalf("%s: expected a SecurityContext, got nil", containerName)
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Errorf("%s: expected allowPrivilegeEscalation=false, got %v", containerName, sc.AllowPrivilegeEscalation)
	}
	if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Errorf("%s: expected readOnlyRootFilesystem=true, got %v", containerName, sc.ReadOnlyRootFilesystem)
	}
	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Errorf("%s: expected runAsNonRoot=true, got %v", containerName, sc.RunAsNonRoot)
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("%s: expected capabilities.drop=[ALL], got %v", containerName, sc.Capabilities)
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("%s: expected seccompProfile RuntimeDefault, got %v", containerName, sc.SeccompProfile)
	}
}

func TestBuildPod_RestrictedSecurityContextByDefault(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

	spec := BuildPod(
		tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", "trade", nil,
	)

	assertRestrictedSecurityContext(t, "freqtrade", spec.Containers[0].SecurityContext)
	initUserData := initContainerNamed(spec, "init-user-data")
	if initUserData == nil {
		t.Fatal("expected an init-user-data container")
	}
	assertRestrictedSecurityContext(t, "init-user-data", initUserData.SecurityContext)
	if initUserData.SecurityContext.RunAsUser != nil {
		t.Errorf("expected init-user-data to run as the pod's own non-root user by default, got RunAsUser=%v",
			*initUserData.SecurityContext.RunAsUser)
	}
	for _, arg := range initUserData.Args {
		if strings.Contains(arg, "chown") || strings.Contains(arg, "chmod") {
			t.Errorf("expected no chown/chmod by default (fsGroup already makes the volume writable), got %q", arg)
		}
	}
}

func TestBuildPod_FixVolumePermissionsOptIn(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			App: &freqtradev1alpha1.TBAppConfig{
				PVCSpec: &freqtradev1alpha1.PVCSpec{FixVolumePermissions: ptrBoolPod(true)},
			},
		},
	}

	spec := BuildPod(
		tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", "trade", nil,
	)

	initUserData := initContainerNamed(spec, "init-user-data")
	if initUserData == nil {
		t.Fatal("expected an init-user-data container")
	}
	if initUserData.SecurityContext == nil || initUserData.SecurityContext.RunAsUser == nil ||
		*initUserData.SecurityContext.RunAsUser != 0 {
		t.Errorf("expected FixVolumePermissions to run init-user-data as root, got %+v", initUserData.SecurityContext)
	}
	found := false
	for _, arg := range initUserData.Args {
		if strings.Contains(arg, "chown") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected FixVolumePermissions to restore the chown step, got args %v", initUserData.Args)
	}
}

func TestBuildPod_DefaultResourcesAvoidBestEffortQoS(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

	spec := BuildPod(
		tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", "trade", nil,
	)

	resources := spec.Containers[0].Resources
	if len(resources.Requests) == 0 || len(resources.Limits) == 0 {
		t.Errorf("expected non-empty default requests and limits (BestEffort pods are OOM-killed first), got %+v", resources)
	}
}

func TestBuildPod_ImageAppliesToMainAndDownloadInitContainerNotInitUserData(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			Data: &freqtradev1alpha1.DataCacheSpec{PVCName: "cache-pvc"},
		},
	}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "", "backtesting", nil)

	if spec.Containers[0].Image != testImage {
		t.Errorf("expected the main container to use the resolved image %q, got %q", testImage, spec.Containers[0].Image)
	}
	downloadInit := initContainerNamed(spec, "init-download-data")
	if downloadInit == nil || downloadInit.Image != testImage {
		t.Errorf("expected init-download-data to use the resolved image %q, got %+v", testImage, downloadInit)
	}
	initUserData := initContainerNamed(spec, "init-user-data")
	if initUserData == nil || initUserData.Image != "busybox:latest" {
		t.Errorf("expected init-user-data to keep using busybox regardless of the resolved freqtrade image, got %+v",
			initUserData)
	}
}
