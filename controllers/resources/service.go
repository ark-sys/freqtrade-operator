package resources

import (
	"context"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildService creates a Service for the bot
func BuildService(tradeBot freqtradev1alpha1.TradeBot) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name,
			Namespace: tradeBot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": tradeBot.Name},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// ApplyService creates or updates the Service
func ApplyService(ctx context.Context, c client.Client, svc *corev1.Service) error {
	var existing corev1.Service
	err := c.Get(ctx, types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, svc)
	} else if err != nil {
		return err
	}
	svc.ResourceVersion = existing.ResourceVersion
	// Preserve the cluster IP that was assigned
	svc.Spec.ClusterIP = existing.Spec.ClusterIP
	return c.Update(ctx, svc)
}
