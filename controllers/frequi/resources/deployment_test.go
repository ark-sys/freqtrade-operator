package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// testNamespace and testFreqUIName are shared across this package's *_test.go files.
const testNamespace = "trading"
const testFreqUIName = "my-frequi"

func TestBuildFreqUIDeployment_Defaults(t *testing.T) {
	frequi := freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
	}

	dep := BuildFreqUIDeployment(frequi)

	if dep.Name != testFreqUIName || dep.Namespace != testNamespace {
		t.Errorf("expected my-frequi/trading, got %s/%s", dep.Namespace, dep.Name)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 1 {
		t.Errorf("expected default replicas 1, got %v", dep.Spec.Replicas)
	}
	if dep.Spec.Selector.MatchLabels["app"] != testFreqUIName {
		t.Errorf("expected selector app=my-frequi, got %v", dep.Spec.Selector.MatchLabels)
	}
	if len(dep.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected exactly one container, got %d", len(dep.Spec.Template.Spec.Containers))
	}
	container := dep.Spec.Template.Spec.Containers[0]
	if container.Image != "freqtradeorg/frequi:latest" {
		t.Errorf("expected default image, got %q", container.Image)
	}
	if container.LivenessProbe == nil || container.ReadinessProbe == nil {
		t.Errorf("expected default liveness/readiness probes to be set")
	}
}

// Regression test: this Deployment previously set no container SecurityContext at all, unlike
// every other workload this operator builds (TradeBot, Backtest both get
// shared.RestrictedSecurityContext, P3-3) - it could never actually be scheduled into a
// restricted-PSA namespace. Found directly: a real e2e run's FreqUI reconcile logged "would
// violate PodSecurity restricted:latest" on every attempt. nginx (the image's base) needs
// somewhere writable for its temp buffers, cache, and pid file even under a read-only root
// filesystem - verified directly (docker run --read-only against the real image) that exactly
// these three paths are what it needs, nothing else. RunAsUser/FSGroup are additionally required
// beyond the shared RestrictedSecurityContext (unlike TradeBot/Backtest, whose image already
// defaults to non-root): this image's own default user is root ("container has runAsNonRoot and
// image will run as root" otherwise), and even with RunAsUser set, the emptyDir Volumes below are
// root-owned by default, so FSGroup is what actually makes them writable by that UID - verified
// end to end against a real pod on a real cluster (1/1 Running, a real HTTP 200 from its Service).
func TestBuildFreqUIDeployment_RestrictedSecurityContext(t *testing.T) {
	frequi := freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
	}

	dep := BuildFreqUIDeployment(frequi)
	podSpec := dep.Spec.Template.Spec
	container := podSpec.Containers[0]

	fsGroupOK := podSpec.SecurityContext != nil && podSpec.SecurityContext.FSGroup != nil &&
		*podSpec.SecurityContext.FSGroup == frequiUID
	if !fsGroupOK {
		t.Errorf("expected pod-level FSGroup=%d, got %v", frequiUID, podSpec.SecurityContext)
	}

	sc := container.SecurityContext
	if sc == nil {
		t.Fatal("expected a container SecurityContext to be set")
	}
	if sc.RunAsUser == nil || *sc.RunAsUser != frequiUID {
		t.Errorf("expected container RunAsUser=%d, got %v", frequiUID, sc.RunAsUser)
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Errorf("expected allowPrivilegeEscalation=false, got %v", sc.AllowPrivilegeEscalation)
	}
	if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Errorf("expected readOnlyRootFilesystem=true, got %v", sc.ReadOnlyRootFilesystem)
	}
	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Errorf("expected runAsNonRoot=true, got %v", sc.RunAsNonRoot)
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Errorf("expected capabilities.drop=[ALL], got %v", sc.Capabilities)
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Errorf("expected seccompProfile RuntimeDefault, got %v", sc.SeccompProfile)
	}

	wantMounts := map[string]string{"tmp": "/tmp", "nginx-cache": "/var/cache/nginx", "nginx-run": "/var/run"}
	if len(container.VolumeMounts) != len(wantMounts) {
		t.Fatalf("expected %d volume mounts for nginx's writable paths, got %+v", len(wantMounts), container.VolumeMounts)
	}
	for _, vm := range container.VolumeMounts {
		if wantMounts[vm.Name] != vm.MountPath {
			t.Errorf("unexpected mount %s -> %s", vm.Name, vm.MountPath)
		}
	}
	if len(podSpec.Volumes) != len(wantMounts) {
		t.Errorf("expected a pod-level Volume backing each of the %d mounts, got %+v", len(wantMounts), podSpec.Volumes)
	}
	for _, v := range podSpec.Volumes {
		if v.EmptyDir == nil {
			t.Errorf("expected volume %s to be an emptyDir, got %+v", v.Name, v)
		}
	}
}

func TestBuildFreqUIDeployment_ImageAndReplicaOverrides(t *testing.T) {
	replicas := int32(3)
	frequi := freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
		Spec: freqtradev1alpha1.FreqUISpec{
			App: &freqtradev1alpha1.FUAppConfig{
				PodSpec: &freqtradev1alpha1.FUPodSpec{
					Image:    "custom/frequi:v2",
					Replicas: &replicas,
				},
			},
		},
	}

	dep := BuildFreqUIDeployment(frequi)

	if dep.Spec.Template.Spec.Containers[0].Image != "custom/frequi:v2" {
		t.Errorf("expected overridden image, got %q", dep.Spec.Template.Spec.Containers[0].Image)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("expected overridden replicas 3, got %v", dep.Spec.Replicas)
	}
}

func TestApplyPodSpecOverrides(t *testing.T) {
	podSpec := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "frequi"}},
	}
	userSpec := &freqtradev1alpha1.FUPodSpec{
		Resources:        corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: {}}},
		Env:              []corev1.EnvVar{{Name: "FOO", Value: "bar"}},
		VolumeMounts:     []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
		Volumes:          []corev1.Volume{{Name: "data"}},
		SecurityContext:  &corev1.PodSecurityContext{RunAsNonRoot: ptrBool(true)},
		InitContainers:   []corev1.Container{{Name: "init"}},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "regcred"}},
		NodeSelector:     map[string]string{"disktype": "ssd"},
		Tolerations:      []corev1.Toleration{{Key: "dedicated"}},
	}

	applyPodSpecOverrides(podSpec, userSpec)

	if len(podSpec.Containers[0].Resources.Requests) != 1 {
		t.Errorf("expected resource requests to be applied")
	}
	if len(podSpec.Containers[0].Env) != 1 || podSpec.Containers[0].Env[0].Name != "FOO" {
		t.Errorf("expected env vars to be appended, got %+v", podSpec.Containers[0].Env)
	}
	if len(podSpec.Containers[0].VolumeMounts) != 1 {
		t.Errorf("expected volume mounts to be appended")
	}
	if len(podSpec.Volumes) != 1 {
		t.Errorf("expected volumes to be appended")
	}
	scOverridden := podSpec.SecurityContext != nil && podSpec.SecurityContext.RunAsNonRoot != nil &&
		*podSpec.SecurityContext.RunAsNonRoot
	if !scOverridden {
		t.Errorf("expected security context to be overridden")
	}
	if len(podSpec.InitContainers) != 1 {
		t.Errorf("expected init containers to be appended")
	}
	if len(podSpec.ImagePullSecrets) != 1 {
		t.Errorf("expected image pull secrets to be appended")
	}
	if podSpec.NodeSelector["disktype"] != "ssd" {
		t.Errorf("expected node selector to be overridden")
	}
	if len(podSpec.Tolerations) != 1 {
		t.Errorf("expected tolerations to be appended")
	}
}

func TestApplyPodSpecOverrides_AntiAffinityCreatesAffinityIfNil(t *testing.T) {
	podSpec := &corev1.PodSpec{Containers: []corev1.Container{{Name: "frequi"}}}
	userSpec := &freqtradev1alpha1.FUPodSpec{
		AntiAffinity: &corev1.PodAntiAffinity{},
	}

	applyPodSpecOverrides(podSpec, userSpec)

	if podSpec.Affinity == nil || podSpec.Affinity.PodAntiAffinity == nil {
		t.Errorf("expected anti-affinity to allocate an Affinity if none existed")
	}
}

func ptrBool(b bool) *bool { return &b }
