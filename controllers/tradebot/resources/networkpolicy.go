package resources

import (
	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// FreqtradeAPIPort is the port freqtrade's REST API listens on inside the
// pod (see pod.go's trade-mode Ports/probes) - the only port
// BuildNetworkPolicy ever opens.
const FreqtradeAPIPort = 8080

// BuildNetworkPolicy locks a trade-mode TradeBot's freqtrade REST API down
// to default-deny (P3-4): only the FreqUI instance(s) that actually
// reference this bot, and the operator itself, may reach port 8080 -
// everything else in the namespace (other bots included) is refused. The
// operator peer is here for D4/P4-3's future bot-polling, built now per the
// plan's own instruction to land both rules together rather than needing a
// second pass later; nothing polls yet, but nothing is blocked by having
// the rule exist early either.
//
// Job-mode TradeBots never get one at all (see reconcileResources) - a Job
// pod never opens 8080 in the first place (pod.go only sets it up for
// trade), so there'd be nothing to protect.
//
// operatorNamespace empty (Reconciler.OperatorNamespace unset) drops that
// peer entirely rather than emitting a selector that matches nothing by
// accident. If freqUINames is also empty, the ingress rule itself is
// dropped too - PolicyTypes still lists Ingress, so the net effect is
// deny-all on this pod rather than a rule with an empty From, which would
// mean the opposite: allow-all.
func BuildNetworkPolicy(
	tradeBot freqtradev1alpha1.TradeBot, freqUINames []string, operatorNamespace string,
) networkingv1.NetworkPolicy {
	peers := make([]networkingv1.NetworkPolicyPeer, 0, len(freqUINames)+1)
	if operatorNamespace != "" {
		peers = append(peers, networkingv1.NetworkPolicyPeer{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"kubernetes.io/metadata.name": operatorNamespace},
			},
		})
	}
	for _, name := range freqUINames {
		// No NamespaceSelector: a podSelector-only peer matches only within
		// the NetworkPolicy's own namespace, which is exactly right here -
		// FreqUI only ever references TradeBots in its own namespace (see
		// main.go's Reconcile, which lists FreqUI with client.InNamespace).
		peers = append(peers, networkingv1.NetworkPolicyPeer{
			PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{appLabelKey: name}},
		})
	}

	var ingress []networkingv1.NetworkPolicyIngressRule
	if len(peers) > 0 {
		port := intstr.FromInt32(FreqtradeAPIPort)
		protocol := corev1.ProtocolTCP
		ingress = []networkingv1.NetworkPolicyIngressRule{
			{
				Ports: []networkingv1.NetworkPolicyPort{{Protocol: &protocol, Port: &port}},
				From:  peers,
			},
		}
	}

	return networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name,
			Namespace: tradeBot.Namespace,
			Labels:    map[string]string{nameLabelKey: tradeBot.Name, appLabelKey: freqtradeAppName},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{nameLabelKey: tradeBot.Name, appLabelKey: freqtradeAppName},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress:     ingress,
		},
	}
}
