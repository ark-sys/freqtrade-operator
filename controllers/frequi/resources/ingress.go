package resources

import (
	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TradeBotAPIRoute represents a TradeBot API route configuration
type TradeBotAPIRoute struct {
	Name        string
	ServiceName string
	PathPrefix  string
}

// BuildFreqUIIngress creates an Ingress for FreqUI with subdomain-based API routing
func BuildFreqUIIngress(frequi freqtradev1alpha1.FreqUI, tradeBotAPIRoutes []TradeBotAPIRoute) networkingv1.Ingress {
	spec := frequi.Spec
	pathType := networkingv1.PathTypePrefix

	// Use Host from spec if set, else default
	mainHost := spec.Host
	if mainHost == "" {
		mainHost = frequi.Name + "." + frequi.Namespace + ".svc.cluster.local"
	}

	var rules []networkingv1.IngressRule

	// Add main UI rule
	rules = append(rules, networkingv1.IngressRule{
		Host: mainHost,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path:     "/",
						PathType: &pathType,
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: frequi.Name,
								Port: networkingv1.ServiceBackendPort{
									Number: 80,
								},
							},
						},
					},
				},
			},
		},
	})

	// Add API subdomain rules
	for _, apiRoute := range tradeBotAPIRoutes {
		apiHost := apiRoute.Name + "." + mainHost
		rules = append(rules, networkingv1.IngressRule{
			Host: apiHost,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: apiRoute.ServiceName,
									Port: networkingv1.ServiceBackendPort{
										Number: 8080,
									},
								},
							},
						},
					},
				},
			},
		})
	}

	// Build TLS configuration
	tls := spec.TLS
	if len(tls) == 0 {
		// If not set, build TLS for all hosts (optional)
		var tlsHosts []string
		tlsHosts = append(tlsHosts, mainHost)
		for _, apiRoute := range tradeBotAPIRoutes {
			apiHost := apiRoute.Name + "." + mainHost
			tlsHosts = append(tlsHosts, apiHost)
		}
		tls = []networkingv1.IngressTLS{{Hosts: tlsHosts}}
	}

	baseIngressSpec := networkingv1.IngressSpec{
		Rules: rules,
		TLS:   tls,
	}

	finalSpec := baseIngressSpec
	if spec.App != nil && spec.App.IngressSpec != nil {
		applyIngressSpecOverrides(&finalSpec, spec.App.IngressSpec)
	}

	// Merge annotations from both spec.IngressAnnotations and spec.App.IngressSpec.Annotations
	annotations := make(map[string]string)

	// Add annotations from spec.IngressAnnotations first
	for k, v := range spec.IngressAnnotations {
		annotations[k] = v
	}

	// Add annotations from spec.App.IngressSpec.Annotations (these take precedence)
	if spec.App != nil && spec.App.IngressSpec != nil {
		for k, v := range spec.App.IngressSpec.Annotations {
			annotations[k] = v
		}
	}

	return networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        frequi.Name,
			Namespace:   frequi.Namespace,
			Annotations: annotations,
		},
		Spec: finalSpec,
	}
}

// applyIngressSpecOverrides applies user-provided ingress specification overrides to the base ingress spec
func applyIngressSpecOverrides(ingressSpec *networkingv1.IngressSpec, userIngressSpec *freqtradev1alpha1.FUIngressSpec) {
	// Override ingress class name if specified
	if userIngressSpec.IngressClassName != nil {
		ingressSpec.IngressClassName = userIngressSpec.IngressClassName
	}

	// Override rules if specified
	if len(userIngressSpec.Rules) > 0 {
		ingressSpec.Rules = userIngressSpec.Rules
	}

	// Override TLS if specified
	if len(userIngressSpec.TLS) > 0 {
		ingressSpec.TLS = userIngressSpec.TLS
	}

	// Override default backend if specified
	if userIngressSpec.DefaultBackend != nil {
		ingressSpec.DefaultBackend = userIngressSpec.DefaultBackend
	}
}
