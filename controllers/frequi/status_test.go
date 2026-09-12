package frequi

import (
	"strings"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
)

func acceptedParent() gatewayv1.RouteParentStatus {
	return conditionParent(metav1.ConditionTrue, string(gatewayv1.RouteReasonAccepted), metav1.ConditionTrue)
}

func rejectedParent(reason string) gatewayv1.RouteParentStatus {
	return conditionParent(metav1.ConditionFalse, reason, metav1.ConditionTrue)
}

func conditionParent(
	acceptedStatus metav1.ConditionStatus, acceptedReason string, resolvedRefsStatus metav1.ConditionStatus,
) gatewayv1.RouteParentStatus {
	return gatewayv1.RouteParentStatus{
		Conditions: []metav1.Condition{
			{Type: string(gatewayv1.RouteConditionAccepted), Status: acceptedStatus, Reason: acceptedReason},
			{Type: string(gatewayv1.RouteConditionResolvedRefs), Status: resolvedRefsStatus, Reason: "ResolvedRefs"},
		},
	}
}

// routeWithParents builds an HTTPRoute carrying the given parent statuses -
// a one-liner for the table-driven-ish tests below instead of repeating the
// same nested Status/RouteStatus/Parents literal at every call site.
func routeWithParents(name string, parents ...gatewayv1.RouteParentStatus) gatewayv1.HTTPRoute {
	return gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status:     gatewayv1.HTTPRouteStatus{RouteStatus: gatewayv1.RouteStatus{Parents: parents}},
	}
}

func TestDeriveGatewayExposureCondition_NoParents(t *testing.T) {
	routes := []gatewayv1.HTTPRoute{
		{ObjectMeta: metav1.ObjectMeta{Name: "ui"}},
	}
	cond := deriveGatewayExposureCondition(routes)
	if cond.Status != metav1.ConditionFalse || cond.Reason != freqtradev1alpha1.ReasonRoutePending {
		t.Errorf("expected False/RoutePending for an unclaimed route, got %s/%s", cond.Status, cond.Reason)
	}
}

func TestDeriveGatewayExposureCondition_AcceptedFalse(t *testing.T) {
	routes := []gatewayv1.HTTPRoute{routeWithParents("ui", rejectedParent("NoMatchingParent"))}
	cond := deriveGatewayExposureCondition(routes)
	if cond.Status != metav1.ConditionFalse || cond.Reason != freqtradev1alpha1.ReasonRouteNotAccepted {
		t.Errorf("expected False/RouteNotAccepted, got %s/%s", cond.Status, cond.Reason)
	}
	if !strings.Contains(cond.Message, "ui") || !strings.Contains(cond.Message, "NoMatchingParent") {
		t.Errorf("expected message to name the route and reason, got %q", cond.Message)
	}
}

func TestDeriveGatewayExposureCondition_ResolvedRefsFalse(t *testing.T) {
	parent := conditionParent(metav1.ConditionTrue, string(gatewayv1.RouteReasonAccepted), metav1.ConditionFalse)
	routes := []gatewayv1.HTTPRoute{routeWithParents("bot-a", parent)}
	cond := deriveGatewayExposureCondition(routes)
	if cond.Status != metav1.ConditionFalse || cond.Reason != freqtradev1alpha1.ReasonRouteNotAccepted {
		t.Errorf("expected False/RouteNotAccepted for ResolvedRefs=False, got %s/%s", cond.Status, cond.Reason)
	}
}

func TestDeriveGatewayExposureCondition_AllGood(t *testing.T) {
	routes := []gatewayv1.HTTPRoute{
		routeWithParents("ui", acceptedParent()),
		routeWithParents("bot-a", acceptedParent()),
	}
	cond := deriveGatewayExposureCondition(routes)
	if cond.Status != metav1.ConditionTrue {
		t.Errorf("expected True when every route has an accepted parent, got %s/%s", cond.Status, cond.Reason)
	}
}

func TestDeriveGatewayExposureCondition_MixedNamesFailingRoute(t *testing.T) {
	routes := []gatewayv1.HTTPRoute{
		routeWithParents("ui", acceptedParent()),
		routeWithParents("bot-a", rejectedParent("NoMatchingListenerHostname")),
	}
	cond := deriveGatewayExposureCondition(routes)
	if cond.Status != metav1.ConditionFalse {
		t.Fatalf("expected False when one of two routes fails, got %s", cond.Status)
	}
	if strings.Contains(cond.Message, "\"ui\"") || !strings.Contains(cond.Message, "bot-a") {
		t.Errorf("expected the message to name only the failing route (bot-a), got %q", cond.Message)
	}
}

func TestDeriveFreqUIURL_HostWithTLS(t *testing.T) {
	frequi := &freqtradev1beta1.FreqUI{
		Spec: freqtradev1beta1.FreqUISpec{
			Host: "frequi.example.com",
			TLS:  []networkingv1.IngressTLS{{Hosts: []string{"frequi.example.com"}}},
		},
	}
	got := deriveFreqUIURL(frequi)
	want := "https://frequi.example.com"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestDeriveFreqUIURL_HostWithoutTLS(t *testing.T) {
	frequi := &freqtradev1beta1.FreqUI{
		Spec: freqtradev1beta1.FreqUISpec{Host: "frequi.example.com"},
	}
	got := deriveFreqUIURL(frequi)
	want := "http://frequi.example.com"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestDeriveFreqUIURL_NoneModeIsAlwaysClusterLocal(t *testing.T) {
	frequi := &freqtradev1beta1.FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: "my-frequi", Namespace: "trading"},
		Spec: freqtradev1beta1.FreqUISpec{
			Host:     "frequi.example.com",
			Exposure: freqtradev1beta1.FUExposureNone,
		},
	}
	got := deriveFreqUIURL(frequi)
	want := "http://my-frequi.trading.svc.cluster.local"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
