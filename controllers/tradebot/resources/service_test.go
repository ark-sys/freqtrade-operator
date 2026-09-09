package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildService_Defaults(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
	}

	svc := BuildService(tradeBot)

	if svc.Name != "my-bot" {
		t.Errorf("expected Name %q, got %q", "my-bot", svc.Name)
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected default type ClusterIP, got %q", svc.Spec.Type)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != 8080 {
		t.Errorf("expected a single port 8080, got %+v", svc.Spec.Ports)
	}
	if svc.Spec.Selector["name"] != "my-bot" {
		t.Errorf("expected selector name=my-bot, got %v", svc.Spec.Selector)
	}
}

func TestBuildService_Overrides(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			App: &freqtradev1alpha1.TBAppConfig{
				ServiceSpec: &freqtradev1alpha1.ServiceSpec{
					Type:        corev1.ServiceTypeNodePort,
					Ports:       []corev1.ServicePort{{Name: "api", Port: 9000}},
					Selector:    map[string]string{"custom": "selector"},
					Annotations: map[string]string{"lb.example.com/internal": "true"},
				},
			},
		},
	}

	svc := BuildService(tradeBot)

	if svc.Spec.Type != corev1.ServiceTypeNodePort {
		t.Errorf("expected overridden type NodePort, got %q", svc.Spec.Type)
	}
	if len(svc.Spec.Ports) != 1 || svc.Spec.Ports[0].Port != 9000 {
		t.Errorf("expected overridden port 9000, got %+v", svc.Spec.Ports)
	}
	if svc.Spec.Selector["custom"] != "selector" {
		t.Errorf("expected overridden selector, got %v", svc.Spec.Selector)
	}
	if svc.Annotations["lb.example.com/internal"] != "true" {
		t.Errorf("expected the override annotation to be set, got %v", svc.Annotations)
	}
}

func TestMergeServiceSpecOverrides_NilOverrideIsNoOp(t *testing.T) {
	defaultSpec := corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP}

	got := mergeServiceSpecOverrides(defaultSpec, nil)

	if got.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected the default spec unchanged, got %+v", got)
	}
}
