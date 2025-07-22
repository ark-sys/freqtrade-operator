package resources

import (
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildFreqUIIngress creates an Ingress for FreqUI with subdomain-based API routing
func BuildFreqUIIngress(options FreqUIOptions) networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix

	// Default host if not specified
	host := options.Name + "." + options.Namespace + ".svc.cluster.local"
	if options.Host != "" {
		host = options.Host
	}

	// Prepare annotations (no regex needed for subdomain approach)
	annotations := make(map[string]string)
	for k, v := range options.IngressAnnotations {
		annotations[k] = v
	}

	// Build ingress rules - UI host + API subdomains
	var rules []networkingv1.IngressRule

	// Add main UI rule (clean, no complex routing)
	rules = append(rules, networkingv1.IngressRule{
		Host: host,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path:     "/",
						PathType: &pathType,
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: options.Name,
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

	// Add API subdomain rules for each TradeBot
	// This simplifies the configuration for multiple bots under the same UI
	for _, apiRoute := range options.TradeBotAPIRoutes {
		apiHost := apiRoute.Name + "." + host

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

	// Build TLS configuration for all hosts
	var tlsHosts []string
	tlsHosts = append(tlsHosts, host) // Main UI host

	for _, apiRoute := range options.TradeBotAPIRoutes {
		apiHost := apiRoute.Name + "." + host
		tlsHosts = append(tlsHosts, apiHost)
	}

	// Update TLS configuration to include all hosts
	var tls []networkingv1.IngressTLS
	if len(options.TLS) > 0 {
		// Use the existing TLS config but extend hosts
		for _, tlsConfig := range options.TLS {
			newTLSConfig := tlsConfig
			newTLSConfig.Hosts = tlsHosts
			tls = append(tls, newTLSConfig)
		}
	}

	return networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        options.Name,
			Namespace:   options.Namespace,
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			Rules: rules,
			TLS:   tls,
		},
	}
}
