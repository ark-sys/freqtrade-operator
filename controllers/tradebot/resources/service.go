package resources

import (
	"context"
	"reflect"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// mergeServiceSpecOverrides merges user overrides from App.ServiceSpec into the default ServiceSpec.
func mergeServiceSpecOverrides(defaultSpec corev1.ServiceSpec, override *freqtradev1alpha1.ServiceSpec) corev1.ServiceSpec {
	if override == nil {
		return defaultSpec
	}

	if override.Type != "" {
		defaultSpec.Type = override.Type
	}
	if len(override.Ports) > 0 {
		defaultSpec.Ports = override.Ports
	}
	if len(override.Selector) > 0 {
		defaultSpec.Selector = override.Selector
	}

	return defaultSpec
}

// BuildService creates a Service for the bot
func BuildService(tradeBot freqtradev1alpha1.TradeBot) corev1.Service {
	// Build default service spec
	baseServiceSpec := corev1.ServiceSpec{
		Selector: map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
		Type:     corev1.ServiceTypeClusterIP, // Default service type
		Ports: []corev1.ServicePort{
			{
				Name:       "http",
				Port:       8080,
				TargetPort: intstr.FromInt32(8080),
				Protocol:   corev1.ProtocolTCP,
			},
		},
	}

	// Merge with user-provided service configuration
	finalSpec := baseServiceSpec
	if tradeBot.Spec.App != nil && tradeBot.Spec.App.ServiceSpec != nil {
		finalSpec = mergeServiceSpecOverrides(baseServiceSpec, tradeBot.Spec.App.ServiceSpec)
	}

	annotations := map[string]string{}
	if tradeBot.Spec.App != nil && tradeBot.Spec.App.ServiceSpec != nil && len(tradeBot.Spec.App.ServiceSpec.Annotations) > 0 {
		annotations = tradeBot.Spec.App.ServiceSpec.Annotations
	}

	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        tradeBot.Name,
			Namespace:   tradeBot.Namespace,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: finalSpec,
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

	// Check if update is needed by comparing relevant fields
	needsUpdate := false

	// Compare ports
	if !reflect.DeepEqual(existing.Spec.Ports, svc.Spec.Ports) {
		needsUpdate = true
	}

	// Compare selector
	if !reflect.DeepEqual(existing.Spec.Selector, svc.Spec.Selector) {
		needsUpdate = true
	}

	// Compare type
	if existing.Spec.Type != svc.Spec.Type {
		needsUpdate = true
	}

	// Compare session affinity
	if existing.Spec.SessionAffinity != svc.Spec.SessionAffinity {
		needsUpdate = true
	}

	if !reflect.DeepEqual(existing.Spec.LoadBalancerSourceRanges, svc.Spec.LoadBalancerSourceRanges) {
		needsUpdate = true
	}

	// Only update if there are actual changes
	if needsUpdate {
		svc.ResourceVersion = existing.ResourceVersion
		// Preserve the cluster IP that was assigned
		svc.Spec.ClusterIP = existing.Spec.ClusterIP
		// Preserve other system-assigned fields
		if existing.Spec.ClusterIPs != nil {
			svc.Spec.ClusterIPs = existing.Spec.ClusterIPs
		}
		if existing.Spec.IPFamilies != nil {
			svc.Spec.IPFamilies = existing.Spec.IPFamilies
		}
		if existing.Spec.IPFamilyPolicy != nil {
			svc.Spec.IPFamilyPolicy = existing.Spec.IPFamilyPolicy
		}
		return c.Update(ctx, svc)
	}

	return nil
}
