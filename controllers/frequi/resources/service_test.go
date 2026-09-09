package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildFreqUIService_Defaults(t *testing.T) {
	frequi := freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
	}

	svc := BuildFreqUIService(frequi)

	if svc.Name != testFreqUIName || svc.Namespace != testNamespace {
		t.Errorf("expected my-frequi/trading, got %s/%s", svc.Namespace, svc.Name)
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected default type ClusterIP, got %q", svc.Spec.Type)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != 80 {
		t.Errorf("expected a single port 80, got %+v", svc.Spec.Ports)
	}
	if svc.Spec.Selector["app"] != testFreqUIName {
		t.Errorf("expected selector app=my-frequi, got %v", svc.Spec.Selector)
	}
}

func TestBuildFreqUIService_Overrides(t *testing.T) {
	frequi := freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
		Spec: freqtradev1alpha1.FreqUISpec{
			App: &freqtradev1alpha1.FUAppConfig{
				ServiceSpec: &freqtradev1alpha1.FUServiceSpec{
					Type:                     corev1.ServiceTypeLoadBalancer,
					Ports:                    []corev1.ServicePort{{Name: "web", Port: 8443}},
					Selector:                 map[string]string{"custom": "selector"},
					LoadBalancerSourceRanges: []string{"10.0.0.0/8"},
					ExternalTrafficPolicy:    corev1.ServiceExternalTrafficPolicyLocal,
					SessionAffinity:          corev1.ServiceAffinityClientIP,
				},
			},
		},
	}

	svc := BuildFreqUIService(frequi)

	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("expected overridden type LoadBalancer, got %q", svc.Spec.Type)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != 8443 {
		t.Errorf("expected overridden port 8443, got %+v", svc.Spec.Ports)
	}
	if svc.Spec.Selector["custom"] != "selector" {
		t.Errorf("expected overridden selector, got %v", svc.Spec.Selector)
	}
	if len(svc.Spec.LoadBalancerSourceRanges) != 1 || svc.Spec.LoadBalancerSourceRanges[0] != "10.0.0.0/8" {
		t.Errorf("expected overridden LB source ranges, got %v", svc.Spec.LoadBalancerSourceRanges)
	}
	if svc.Spec.ExternalTrafficPolicy != corev1.ServiceExternalTrafficPolicyLocal {
		t.Errorf("expected overridden external traffic policy, got %q", svc.Spec.ExternalTrafficPolicy)
	}
	if svc.Spec.SessionAffinity != corev1.ServiceAffinityClientIP {
		t.Errorf("expected overridden session affinity, got %q", svc.Spec.SessionAffinity)
	}
}

func TestApplyServiceSpecOverrides_EmptyOverrideIsNoOp(t *testing.T) {
	defaultSpec := corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP}

	applyServiceSpecOverrides(&defaultSpec, &freqtradev1alpha1.FUServiceSpec{})

	if defaultSpec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected the default spec unchanged, got %+v", defaultSpec)
	}
}
