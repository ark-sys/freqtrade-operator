package resources

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IngressOptions defines options for creating an Ingress
type IngressOptions struct {
	Name               string
	Namespace          string
	ServiceName        string
	Host               string
	IngressAnnotations map[string]string
	TLS                []networkingv1.IngressTLS
	OwnerRef           *metav1.OwnerReference
}

// BuildIngress creates an Ingress for the service
func BuildIngress(options IngressOptions) networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix

	// Default host if not specified
	host := options.Name + "." + options.Namespace + ".svc.cluster.local"
	if options.Host != "" {
		host = options.Host
	}

	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        options.Name,
			Namespace:   options.Namespace,
			Annotations: options.IngressAnnotations,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: options.ServiceName,
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: options.TLS,
		},
	}

	// Add owner reference if provided
	if options.OwnerRef != nil {
		ingress.OwnerReferences = []metav1.OwnerReference{*options.OwnerRef}
	}

	return ingress
}

// ApplyIngress creates or updates the Ingress
func ApplyIngress(ctx context.Context, c client.Client, ing *networkingv1.Ingress) error {
	var existing networkingv1.Ingress
	err := c.Get(ctx, types.NamespacedName{Name: ing.Name, Namespace: ing.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, ing)
	} else if err != nil {
		return err
	}
	ing.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, ing)
}
