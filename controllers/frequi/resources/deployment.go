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

// TradeBotAPIRoute represents a TradeBot API route configuration
type TradeBotAPIRoute struct {
	Name        string
	ServiceName string
	PathPrefix  string
}

// FreqUIOptions defines options for FreqUI deployment
type FreqUIOptions struct {
	Name               string
	Namespace          string
	Image              string
	Host               string
	IngressAnnotations map[string]string
	TLS                []networkingv1.IngressTLS
	TradeBotAPIRoutes  []TradeBotAPIRoute
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
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: &[]int64{30}[0],
					DNSPolicy:                     corev1.DNSClusterFirst,
					SecurityContext:               &corev1.PodSecurityContext{},
					Containers: []corev1.Container{
						{
							Name:            "frequi",
							Image:           image,
							ImagePullPolicy: corev1.PullAlways,
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
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						},
					},
				},
			},
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
