package resources

import (
	"strings"
	"testing"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	tradebotresources "github.com/ark-sys/freqtrade-operator/controllers/tradebot/resources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestBuildFreqUIHTTPRoutes_DefaultHostAndNoAPIRoutes(t *testing.T) {
	frequi := freqtradev1beta1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
	}

	routes, skipped := BuildFreqUIHTTPRoutes(frequi, nil)

	if len(skipped) != 0 {
		t.Errorf("expected no skipped routes, got %v", skipped)
	}
	if len(routes) != 1 {
		t.Fatalf("expected exactly the UI route with no API routes, got %d", len(routes))
	}
	ui := routes[0]
	if ui.Name != testFreqUIName || ui.Namespace != testNamespace {
		t.Errorf("expected my-frequi/trading, got %s/%s", ui.Namespace, ui.Name)
	}
	wantHost := "my-frequi.trading.svc.cluster.local"
	if len(ui.Spec.Hostnames) != 1 || string(ui.Spec.Hostnames[0]) != wantHost {
		t.Errorf("expected default host %q, got %v", wantHost, ui.Spec.Hostnames)
	}
	if len(ui.Spec.Rules) != 1 || len(ui.Spec.Rules[0].BackendRefs) != 1 {
		t.Fatalf("expected exactly one rule with one backendRef, got %+v", ui.Spec.Rules)
	}
	backend := ui.Spec.Rules[0].BackendRefs[0]
	if string(backend.Name) != testFreqUIName || backend.Port == nil || *backend.Port != 80 {
		t.Errorf("expected UI backend %s:80, got %s:%v", testFreqUIName, backend.Name, backend.Port)
	}
}

func TestBuildFreqUIHTTPRoutes_ExplicitHostAndAPIRoutes(t *testing.T) {
	frequi := freqtradev1beta1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
		Spec:       freqtradev1beta1.FreqUISpec{Host: "frequi.example.com"},
	}
	apiRoutes := []TradeBotAPIRoute{
		{Name: "bot-a", ServiceName: "bot-a-svc", PathPrefix: "/"},
		{Name: "bot-b", ServiceName: "bot-b-svc", PathPrefix: "/"},
	}

	routes, skipped := BuildFreqUIHTTPRoutes(frequi, apiRoutes)

	if len(skipped) != 0 {
		t.Errorf("expected no skipped routes, got %v", skipped)
	}
	if len(routes) != 3 {
		t.Fatalf("expected 3 routes (UI + 2 bots), got %d", len(routes))
	}

	ui := routes[0]
	if len(ui.Spec.Hostnames) != 1 || string(ui.Spec.Hostnames[0]) != "frequi.example.com" {
		t.Errorf("expected UI hostname frequi.example.com, got %v", ui.Spec.Hostnames)
	}

	wantNames := map[string]struct {
		host    string
		backend string
		port    int32
	}{
		testFreqUIName + "-bot-a": {"bot-a.frequi.example.com", "bot-a-svc", tradebotresources.FreqtradeAPIPort},
		testFreqUIName + "-bot-b": {"bot-b.frequi.example.com", "bot-b-svc", tradebotresources.FreqtradeAPIPort},
	}
	for _, r := range routes[1:] {
		want, ok := wantNames[r.Name]
		if !ok {
			t.Fatalf("unexpected route name %q", r.Name)
		}
		if len(r.Spec.Hostnames) != 1 || string(r.Spec.Hostnames[0]) != want.host {
			t.Errorf("route %s: expected hostname %q, got %v", r.Name, want.host, r.Spec.Hostnames)
		}
		backend := r.Spec.Rules[0].BackendRefs[0]
		if string(backend.Name) != want.backend || backend.Port == nil || int32(*backend.Port) != want.port {
			t.Errorf("route %s: expected backend %s:%d, got %s:%v", r.Name, want.backend, want.port, backend.Name, backend.Port)
		}
	}
}

func TestBuildFreqUIHTTPRoutes_GatewayHostnamesOverrideAppliesOnlyToUIRoute(t *testing.T) {
	frequi := freqtradev1beta1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
		Spec: freqtradev1beta1.FreqUISpec{
			Host: "frequi.example.com",
			Gateway: &freqtradev1beta1.FUGatewaySpec{
				ParentRefs: []gatewayv1.ParentReference{{Name: "my-gateway"}},
				Hostnames:  []gatewayv1.Hostname{"alt.example.com", "alt2.example.com"},
			},
		},
	}
	apiRoutes := []TradeBotAPIRoute{{Name: "bot-a", ServiceName: "bot-a-svc", PathPrefix: "/"}}

	routes, _ := BuildFreqUIHTTPRoutes(frequi, apiRoutes)

	ui := routes[0]
	if len(ui.Spec.Hostnames) != 2 || string(ui.Spec.Hostnames[0]) != "alt.example.com" {
		t.Errorf("expected the UI route to use the override hostnames, got %v", ui.Spec.Hostnames)
	}

	botRoute := routes[1]
	if len(botRoute.Spec.Hostnames) != 1 || string(botRoute.Spec.Hostnames[0]) != "bot-a.frequi.example.com" {
		t.Errorf("expected the bot route to still derive its hostname from spec.host, got %v", botRoute.Spec.Hostnames)
	}
}

func TestBuildFreqUIHTTPRoutes_ParentRefsCopiedToEveryRoute(t *testing.T) {
	parentRefs := []gatewayv1.ParentReference{{Name: "gw-a"}, {Name: "gw-b"}}
	frequi := freqtradev1beta1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
		Spec: freqtradev1beta1.FreqUISpec{
			Gateway: &freqtradev1beta1.FUGatewaySpec{ParentRefs: parentRefs},
		},
	}
	apiRoutes := []TradeBotAPIRoute{{Name: "bot-a", ServiceName: "bot-a-svc", PathPrefix: "/"}}

	routes, _ := BuildFreqUIHTTPRoutes(frequi, apiRoutes)

	for _, r := range routes {
		if len(r.Spec.ParentRefs) != 2 {
			t.Fatalf("route %s: expected 2 parentRefs, got %d", r.Name, len(r.Spec.ParentRefs))
		}
		if string(r.Spec.ParentRefs[0].Name) != "gw-a" || string(r.Spec.ParentRefs[1].Name) != "gw-b" {
			t.Errorf("route %s: parentRefs not copied verbatim, got %+v", r.Name, r.Spec.ParentRefs)
		}
	}
}

func TestBuildFreqUIHTTPRoutes_LabelsAndAnnotations(t *testing.T) {
	frequi := freqtradev1beta1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
		Spec: freqtradev1beta1.FreqUISpec{
			Gateway: &freqtradev1beta1.FUGatewaySpec{
				ParentRefs:  []gatewayv1.ParentReference{{Name: "gw"}},
				Annotations: map[string]string{"custom/annotation": "yes"},
				// Attempting to override the operator's own ownership label must lose.
				Labels: map[string]string{"team": "trading", frequiRouteLabelKey: "someone-else"},
			},
		},
	}

	routes, _ := BuildFreqUIHTTPRoutes(frequi, nil)
	route := routes[0]

	if route.Labels[frequiRouteLabelKey] != testFreqUIName {
		t.Errorf("expected operator's ownership label to win, got %q", route.Labels[frequiRouteLabelKey])
	}
	if route.Labels[managedByLabelKey] != managedByLabelValue {
		t.Errorf("expected managed-by label %q, got %q", managedByLabelValue, route.Labels[managedByLabelKey])
	}
	if route.Labels["team"] != "trading" {
		t.Errorf("expected user-supplied label to survive, got %q", route.Labels["team"])
	}
	if route.Annotations["custom/annotation"] != "yes" {
		t.Errorf("expected spec.gateway.annotations to be copied, got %+v", route.Annotations)
	}
}

func TestBuildFreqUIHTTPRoutes_OverLongNameIsSkippedNotEmitted(t *testing.T) {
	frequi := freqtradev1beta1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: testFreqUIName, Namespace: testNamespace},
	}
	longBotName := strings.Repeat("a", 250)
	apiRoutes := []TradeBotAPIRoute{{Name: longBotName, ServiceName: "bot-svc", PathPrefix: "/"}}

	routes, skipped := BuildFreqUIHTTPRoutes(frequi, apiRoutes)

	if len(routes) != 1 {
		t.Fatalf("expected only the UI route, the over-long bot route should be skipped, got %d routes", len(routes))
	}
	if len(skipped) != 1 || skipped[0] != longBotName {
		t.Errorf("expected the over-long bot route to be reported as skipped, got %v", skipped)
	}
}
