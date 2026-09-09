package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildFreqUIIngress_DefaultHostAndNoAPIRoutes(t *testing.T) {
	frequi := freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
	}

	ing := BuildFreqUIIngress(frequi, nil)

	if ing.Name != testFreqUIName || ing.Namespace != testNamespace {
		t.Errorf("expected my-frequi/trading, got %s/%s", ing.Namespace, ing.Name)
	}
	if len(ing.Spec.Rules) != 1 {
		t.Fatalf("expected exactly the main UI rule with no API routes, got %d rules", len(ing.Spec.Rules))
	}
	wantHost := "my-frequi.trading.svc.cluster.local"
	if ing.Spec.Rules[0].Host != wantHost {
		t.Errorf("expected default host %q, got %q", wantHost, ing.Spec.Rules[0].Host)
	}
	if len(ing.Spec.TLS) != 1 || len(ing.Spec.TLS[0].Hosts) != 1 || ing.Spec.TLS[0].Hosts[0] != wantHost {
		t.Errorf("expected TLS auto-built for the default host, got %+v", ing.Spec.TLS)
	}
}

func TestBuildFreqUIIngress_ExplicitHostAndAPIRoutes(t *testing.T) {
	frequi := freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
		Spec:       freqtradev1alpha1.FreqUISpec{Host: "frequi.example.com"},
	}
	routes := []TradeBotAPIRoute{
		{Name: "bot-a", ServiceName: "bot-a-svc", PathPrefix: "/"},
		{Name: "bot-b", ServiceName: "bot-b-svc", PathPrefix: "/"},
	}

	ing := BuildFreqUIIngress(frequi, routes)

	if len(ing.Spec.Rules) != 3 {
		t.Fatalf("expected 1 main rule + 2 API routes = 3 rules, got %d", len(ing.Spec.Rules))
	}
	if ing.Spec.Rules[0].Host != "frequi.example.com" {
		t.Errorf("expected explicit host on the main rule, got %q", ing.Spec.Rules[0].Host)
	}
	wantAPIHost := "bot-a.frequi.example.com"
	if ing.Spec.Rules[1].Host != wantAPIHost {
		t.Errorf("expected API subdomain host %q, got %q", wantAPIHost, ing.Spec.Rules[1].Host)
	}
	backend := ing.Spec.Rules[1].HTTP.Paths[0].Backend.Service
	if backend.Name != "bot-a-svc" || backend.Port.Number != 8080 {
		t.Errorf("expected API rule to route to bot-a-svc:8080, got %+v", backend)
	}
	if len(ing.Spec.TLS) != 1 || len(ing.Spec.TLS[0].Hosts) != 3 {
		t.Errorf("expected TLS auto-built covering the main host and both API hosts, got %+v", ing.Spec.TLS)
	}
}

func TestBuildFreqUIIngress_ExplicitTLSIsNotOverwritten(t *testing.T) {
	frequi := freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
		Spec: freqtradev1alpha1.FreqUISpec{
			Host: "frequi.example.com",
			TLS:  []networkingv1.IngressTLS{{Hosts: []string{"frequi.example.com"}, SecretName: "my-cert"}},
		},
	}

	ing := BuildFreqUIIngress(frequi, nil)

	if len(ing.Spec.TLS) != 1 || ing.Spec.TLS[0].SecretName != "my-cert" {
		t.Errorf("expected the user-supplied TLS block to be used as-is, got %+v", ing.Spec.TLS)
	}
}

func TestBuildFreqUIIngress_AnnotationsMergeWithAppOverrideTakingPrecedence(t *testing.T) {
	ingressClass := "nginx"
	frequi := freqtradev1alpha1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
		Spec: freqtradev1alpha1.FreqUISpec{
			IngressAnnotations: map[string]string{
				"a": "from-spec",
				"b": "from-spec-only",
			},
			App: &freqtradev1alpha1.FUAppConfig{
				IngressSpec: &freqtradev1alpha1.FUIngressSpec{
					IngressClassName: &ingressClass,
					Annotations:      map[string]string{"a": "from-app-override"},
				},
			},
		},
	}

	ing := BuildFreqUIIngress(frequi, nil)

	if ing.Annotations["a"] != "from-app-override" {
		t.Errorf("expected App.IngressSpec.Annotations to win over spec.IngressAnnotations for key %q, got %q",
			"a", ing.Annotations["a"])
	}
	if ing.Annotations["b"] != "from-spec-only" {
		t.Errorf("expected spec.IngressAnnotations-only keys to survive the merge, got %q", ing.Annotations["b"])
	}
	if ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName != "nginx" {
		t.Errorf("expected overridden ingress class name, got %v", ing.Spec.IngressClassName)
	}
}

func TestApplyIngressSpecOverrides_DefaultBackend(t *testing.T) {
	spec := &networkingv1.IngressSpec{}
	userSpec := &freqtradev1alpha1.FUIngressSpec{
		DefaultBackend: &networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{Name: "fallback"},
		},
	}

	applyIngressSpecOverrides(spec, userSpec)

	if spec.DefaultBackend == nil || spec.DefaultBackend.Service.Name != "fallback" {
		t.Errorf("expected default backend to be applied, got %+v", spec.DefaultBackend)
	}
}
