package resources

import (
	"context"
	"reflect"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildFreqUIService creates a Service for FreqUI
func BuildFreqUIService(frequi freqtradev1alpha1.FreqUI) corev1.Service {
	// Build default service spec
	baseServiceSpec := corev1.ServiceSpec{
		Selector: map[string]string{"app": frequi.Name},
		Type:     corev1.ServiceTypeClusterIP, // Default service type
		Ports: []corev1.ServicePort{
			{
				Name:       "http",
				Port:       80,
				TargetPort: intstr.FromInt32(80),
				Protocol:   corev1.ProtocolTCP,
			},
		},
	}

	// Merge with user-provided service configuration
	finalSpec := baseServiceSpec
	if frequi.Spec.App != nil && !reflect.DeepEqual(frequi.Spec.App.ServiceSpec, corev1.ServiceSpec{}) {
		finalSpec = shared.MergeSpecsWithStrategicPatch(baseServiceSpec, frequi.Spec.App.ServiceSpec, &corev1.ServiceSpec{})
	}

	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frequi.Name,
			Namespace: frequi.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&frequi, freqtradev1alpha1.GroupVersion.WithKind("FreqUI")),
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

	// Compare external traffic policy
	if existing.Spec.ExternalTrafficPolicy != svc.Spec.ExternalTrafficPolicy {
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
