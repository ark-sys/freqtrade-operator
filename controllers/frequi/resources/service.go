package resources

import (
	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	if frequi.Spec.App != nil && frequi.Spec.App.ServiceSpec != nil {
		applyServiceSpecOverrides(&finalSpec, frequi.Spec.App.ServiceSpec)
	}

	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frequi.Name,
			Namespace: frequi.Namespace,
		},
		Spec: finalSpec,
	}
}

// applyServiceSpecOverrides applies user-provided service specification overrides to the base service spec
func applyServiceSpecOverrides(serviceSpec *corev1.ServiceSpec, userServiceSpec *freqtradev1alpha1.FUServiceSpec) {
	// Override service type if specified
	if userServiceSpec.Type != "" {
		serviceSpec.Type = userServiceSpec.Type
	}

	// Override ports if specified
	if len(userServiceSpec.Ports) > 0 {
		serviceSpec.Ports = userServiceSpec.Ports
	}

	// Override selector if specified
	if len(userServiceSpec.Selector) > 0 {
		serviceSpec.Selector = userServiceSpec.Selector
	}

	// Override load balancer source ranges if specified
	if len(userServiceSpec.LoadBalancerSourceRanges) > 0 {
		serviceSpec.LoadBalancerSourceRanges = userServiceSpec.LoadBalancerSourceRanges
	}

	// Override external traffic policy if specified
	if userServiceSpec.ExternalTrafficPolicy != "" {
		serviceSpec.ExternalTrafficPolicy = userServiceSpec.ExternalTrafficPolicy
	}

	// Override session affinity if specified
	if userServiceSpec.SessionAffinity != "" {
		serviceSpec.SessionAffinity = userServiceSpec.SessionAffinity
	}
}
