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

// testBotName and testNamespace are the fixture TradeBot name/namespace
// shared across this package's tests.
const (
	testBotName   = "my-bot"
	testNamespace = "trading"
)

// dummyProbeCommand is a real no-op shell command, used as Exec probe
// command content in tests that only care whether an override propagates.
const dummyProbeCommand = "true"

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
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace}}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", nil)

	container := spec.Containers[0]
	if container.Args[0] != freqCommandTrade {
		t.Errorf("expected first arg 'trade', got %v", container.Args)
	}
	if container.LivenessProbe == nil || container.ReadinessProbe == nil {
		t.Error("expected liveness and readiness probes")
	}
	if len(container.Ports) != 1 || container.Ports[0].ContainerPort != 8080 {
		t.Errorf("expected port 8080 exposed, got %+v", container.Ports)
	}
	v := volumeNamed(spec, "user-data")
	if v == nil || v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName != "my-bot-data" {
		t.Errorf("expected user-data backed by the given PVC, got %+v", v)
	}
}

func TestBuildPod_ExtraFreqArgsAppended(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace}}

	freqArgs := []string{"--some-flag", "value"}
	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", freqArgs)

	if !containsArg(spec.Containers[0].Args, "--some-flag") {
		t.Errorf("expected extra freqArgs to be appended, got %v", spec.Containers[0].Args)
	}
}

func TestBuildPod_AppliesPodSpecOverrides(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace},
		Spec: freqtradev1alpha1.TradeBotSpec{
			App: &freqtradev1alpha1.TBAppConfig{
				PodSpec: &freqtradev1alpha1.PodSpec{
					Image: "custom/freqtrade:latest",
					Env:   []corev1.EnvVar{{Name: "TZ", Value: "UTC"}},
				},
			},
		},
	}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", nil)

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
		Image: "override:image",
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
		},
		Env:              []corev1.EnvVar{{Name: "TZ", Value: "UTC"}},
		VolumeMounts:     []corev1.VolumeMount{{Name: "extra", MountPath: "/extra"}},
		Volumes:          []corev1.Volume{{Name: "extra"}},
		SecurityContext:  &corev1.PodSecurityContext{RunAsNonRoot: ptrBoolPod(true)},
		InitContainers:   []corev1.Container{{Name: "custom-init"}},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "regcred"}},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{dummyProbeCommand}}},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{dummyProbeCommand}}},
		},
		Affinity:     &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{}},
		NodeSelector: map[string]string{"disktype": "ssd"},
		Tolerations:  []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists}},
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
		NodeSelector: map[string]string{"default": "enabled"},
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
	if got.NodeSelector["default"] != "enabled" {
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
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace}}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", nil)

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
		ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace},
		Spec: freqtradev1alpha1.TradeBotSpec{
			App: &freqtradev1alpha1.TBAppConfig{
				PVCSpec: &freqtradev1alpha1.PVCSpec{FixVolumePermissions: ptrBoolPod(true)},
			},
		},
	}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", nil)

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
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace}}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", nil)

	resources := spec.Containers[0].Resources
	if len(resources.Requests) == 0 || len(resources.Limits) == 0 {
		t.Errorf("expected non-empty default requests and limits (BestEffort pods are OOM-killed first), got %+v", resources)
	}
}

func TestBuildPod_ImageAppliesToMainAndInitUserData(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace}}

	spec := BuildPod(tradeBot, testImage, "SampleStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", nil)

	if spec.Containers[0].Image != testImage {
		t.Errorf("expected the main container to use the resolved image %q, got %q", testImage, spec.Containers[0].Image)
	}
	initUserData := initContainerNamed(spec, "init-user-data")
	if initUserData == nil || initUserData.Image != "busybox:latest" {
		t.Errorf("expected init-user-data to keep using busybox regardless of the resolved freqtrade image, got %+v",
			initUserData)
	}
}
