package resources

import (
	"context"
	"reflect"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/ark-sys/freqtrade-operator/controllers/shared"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildFreqUIDeployment creates a Deployment for FreqUI
func BuildFreqUIDeployment(frequi freqtradev1alpha1.FreqUI) appsv1.Deployment {
	replicas := int32(1)

	// Set default image if not specified
	image := "freqtradeorg/frequi:latest"
	if frequi.Spec.App != nil &&
		len(frequi.Spec.App.DeploymentSpec.Template.Spec.Containers) > 0 &&
		frequi.Spec.App.DeploymentSpec.Template.Spec.Containers[0].Image != "" {
		image = frequi.Spec.App.DeploymentSpec.Template.Spec.Containers[0].Image
	}

	// Build default deployment spec
	baseDeploymentSpec := appsv1.DeploymentSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": frequi.Name},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": frequi.Name},
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
	}

	// Merge with user-provided deployment configuration
	finalSpec := baseDeploymentSpec
	if frequi.Spec.App != nil && !reflect.DeepEqual(frequi.Spec.App.DeploymentSpec, appsv1.DeploymentSpec{}) {
		finalSpec = shared.MergeSpecsWithStrategicPatch(baseDeploymentSpec, frequi.Spec.App.DeploymentSpec, &appsv1.DeploymentSpec{})
	}

	return appsv1.Deployment{
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

// ApplyDeployment creates or updates the Deployment
func ApplyDeployment(ctx context.Context, c client.Client, deploy *appsv1.Deployment) error {
	var existing appsv1.Deployment
	err := c.Get(ctx, types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, deploy)
	} else if err != nil {
		return err
	}

	// Check if update is needed by comparing relevant fields
	needsUpdate := false

	// Compare spec fields that matter for deployment
	if !reflect.DeepEqual(existing.Spec.Template.Spec, deploy.Spec.Template.Spec) {
		needsUpdate = true
	}

	// Compare template labels
	if !reflect.DeepEqual(existing.Spec.Template.Labels, deploy.Spec.Template.Labels) {
		needsUpdate = true
	}

	// Compare annotations
	if !reflect.DeepEqual(existing.Spec.Template.Annotations, deploy.Spec.Template.Annotations) {
		needsUpdate = true
	}

	// Only update if there are actual changes
	if needsUpdate {
		return c.Update(ctx, deploy)
	}

	return nil
}
