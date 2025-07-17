package resources

import (
	"context"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildIngress creates an Ingress for the bot's UI
func BuildIngress(tradeBot freqtradev1alpha1.TradeBot, serviceName string) networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix

	// Default host if not specified
	host := tradeBot.Name + "." + tradeBot.Namespace + ".svc.cluster.local"
	if tradeBot.Spec.Host != "" {
		host = tradeBot.Spec.Host
	}

	return networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name,
			Namespace: tradeBot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
			Annotations: tradeBot.Spec.IngressAnnotations,
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
											Name: serviceName,
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
			TLS: tradeBot.Spec.TLS,
		},
	}
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
