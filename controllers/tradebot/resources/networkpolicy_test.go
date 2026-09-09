package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildNetworkPolicy_SelectsThisBotsOwnPods(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

	np := BuildNetworkPolicy(tradeBot, []string{"ui"}, "operator-system")

	if np.Name != "my-bot" || np.Namespace != "trading" {
		t.Errorf("expected NetworkPolicy my-bot/trading, got %s/%s", np.Namespace, np.Name)
	}
	want := map[string]string{"name": "my-bot", "app": "freqtrade"}
	if got := np.Spec.PodSelector.MatchLabels; !mapsEqual(got, want) {
		t.Errorf("expected podSelector %v to match this TradeBot's own pods (see statefulset.go/job.go), got %v", want, got)
	}
	if len(np.Spec.PolicyTypes) != 1 || np.Spec.PolicyTypes[0] != networkingv1.PolicyTypeIngress {
		t.Errorf("expected only Ingress in PolicyTypes, got %v", np.Spec.PolicyTypes)
	}
}

func TestBuildNetworkPolicy_AllowsOperatorAndFreqUIOnPort8080(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

	np := BuildNetworkPolicy(tradeBot, []string{"ui-a", "ui-b"}, "operator-system")

	if len(np.Spec.Ingress) != 1 {
		t.Fatalf("expected exactly one ingress rule, got %d", len(np.Spec.Ingress))
	}
	rule := np.Spec.Ingress[0]
	if len(rule.Ports) != 1 || rule.Ports[0].Port.IntValue() != FreqtradeAPIPort {
		t.Errorf("expected the rule to cover only port %d, got %+v", FreqtradeAPIPort, rule.Ports)
	}
	if len(rule.From) != 3 {
		t.Fatalf("expected 3 peers (operator + 2 FreqUIs), got %d: %+v", len(rule.From), rule.From)
	}

	foundOperator := false
	foundUIA, foundUIB := false, false
	for _, peer := range rule.From {
		switch {
		case peer.NamespaceSelector != nil:
			if peer.NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] == "operator-system" {
				foundOperator = true
			}
			if peer.PodSelector != nil {
				t.Errorf("expected the operator peer to allow its whole namespace, not a specific pod label: %+v", peer)
			}
		case peer.PodSelector != nil:
			switch peer.PodSelector.MatchLabels["app"] {
			case "ui-a":
				foundUIA = true
			case "ui-b":
				foundUIB = true
			}
		}
	}
	if !foundOperator {
		t.Error("expected a peer allowing the operator's namespace")
	}
	if !foundUIA || !foundUIB {
		t.Errorf("expected peers for both referencing FreqUIs, got from=%+v", rule.From)
	}
}

func TestBuildNetworkPolicy_NoOperatorNamespaceDropsThatPeerOnly(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

	np := BuildNetworkPolicy(tradeBot, []string{"ui"}, "")

	if len(np.Spec.Ingress) != 1 || len(np.Spec.Ingress[0].From) != 1 {
		t.Fatalf("expected exactly one FreqUI peer and no operator peer, got %+v", np.Spec.Ingress)
	}
	if np.Spec.Ingress[0].From[0].PodSelector.MatchLabels["app"] != "ui" {
		t.Errorf("expected the remaining peer to be the FreqUI, got %+v", np.Spec.Ingress[0].From[0])
	}
}

// TestBuildNetworkPolicy_NothingToAllowStaysDenyAllNotAllowAll is the
// safety-critical case: a NetworkPolicyIngressRule with an empty (or nil)
// From means allow-from-everywhere, the exact opposite of this resource's
// purpose. With no operator namespace and no referencing FreqUI, there is
// nothing legitimate to allow, so BuildNetworkPolicy must drop the ingress
// rule entirely (deny-all) rather than emit one with an empty From.
func TestBuildNetworkPolicy_NothingToAllowStaysDenyAllNotAllowAll(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"}}

	np := BuildNetworkPolicy(tradeBot, nil, "")

	if len(np.Spec.Ingress) != 0 {
		t.Fatalf("expected no ingress rules (deny-all) when there is nothing to allow, got %+v", np.Spec.Ingress)
	}
	if len(np.Spec.PolicyTypes) != 1 || np.Spec.PolicyTypes[0] != networkingv1.PolicyTypeIngress {
		t.Errorf("expected PolicyTypes to still list Ingress so deny-all actually applies, got %v", np.Spec.PolicyTypes)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
