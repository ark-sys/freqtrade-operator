package resources

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FreqUIOptions defines options for FreqUI deployment
type FreqUIOptions struct {
	Name               string
	Namespace          string
	Image              string
	Host               string
	IngressAnnotations map[string]string
	TLS                []networkingv1.IngressTLS
}

// BuildFreqUIDeployment creates a Deployment for FreqUI
func BuildFreqUIDeployment(options FreqUIOptions) appsv1.Deployment {
	replicas := int32(1)

	// Set default image if not specified
	image := "freqtradeorg/frequi:latest"
	if options.Image != "" {
		image = options.Image
	}

	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name,
			Namespace: options.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": options.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": options.Name},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "frequi",
							Image: image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/",
										Port:   intstr.FromInt(80),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       30,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/",
										Port:   intstr.FromInt(80),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
						},
					},
				},
			},
		},
	}
}

// BuildFreqUIService creates a Service for FreqUI
func BuildFreqUIService(options FreqUIOptions) corev1.Service {
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name,
			Namespace: options.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": options.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// BuildFreqUIIngress creates an Ingress for FreqUI
func BuildFreqUIIngress(options FreqUIOptions) networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix

	// Default host if not specified
	host := options.Name + "." + options.Namespace + ".svc.cluster.local"
	if options.Host != "" {
		host = options.Host
	}

	return networkingv1.Ingress{
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
				},
			},
			TLS: options.TLS,
		},
	}
}

// ApplyDeployment creates or updates the Deployment
func ApplyDeployment(ctx context.Context, c client.Client, deploy *appsv1.Deployment) error {
	var existing appsv1.Deployment
	err := c.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, deploy)
	} else if err != nil {
		return err
	}
	deploy.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, deploy)
}
