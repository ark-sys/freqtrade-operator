package resources

import (
	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// BuildFreqUIDeployment creates a Deployment for FreqUI
func BuildFreqUIDeployment(frequi freqtradev1alpha1.FreqUI) appsv1.Deployment {
	replicas := int32(1)

	// Set default image if not specified
	image := "freqtradeorg/frequi:latest"
	if frequi.Spec.App != nil && frequi.Spec.App.PodSpec != nil && frequi.Spec.App.PodSpec.Image != "" {
		image = frequi.Spec.App.PodSpec.Image
	}

	// Override replicas if specified
	if frequi.Spec.App != nil && frequi.Spec.App.PodSpec != nil && frequi.Spec.App.PodSpec.Replicas != nil {
		replicas = *frequi.Spec.App.PodSpec.Replicas
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
									Port:   intstr.FromInt32(80),
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
									Port:   intstr.FromInt32(80),
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

	// Merge with user-provided pod configuration
	finalSpec := baseDeploymentSpec
	if frequi.Spec.App != nil && frequi.Spec.App.PodSpec != nil {
		applyPodSpecOverrides(&finalSpec.Template.Spec, frequi.Spec.App.PodSpec)
	}

	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      frequi.Name,
			Namespace: frequi.Namespace,
		},
		Spec: finalSpec,
	}
}

// applyPodSpecOverrides applies user-provided pod specification overrides to the base pod spec
func applyPodSpecOverrides(podSpec *corev1.PodSpec, userPodSpec *freqtradev1alpha1.FUPodSpec) {
	// Override container resources if specified
	if len(userPodSpec.Resources.Limits) > 0 || len(userPodSpec.Resources.Requests) > 0 {
		if len(podSpec.Containers) > 0 {
			podSpec.Containers[0].Resources = userPodSpec.Resources
		}
	}

	// Override environment variables if specified
	if len(userPodSpec.Env) > 0 {
		if len(podSpec.Containers) > 0 {
			podSpec.Containers[0].Env = append(podSpec.Containers[0].Env, userPodSpec.Env...)
		}
	}

	// Override volume mounts if specified
	if len(userPodSpec.VolumeMounts) > 0 {
		if len(podSpec.Containers) > 0 {
			podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, userPodSpec.VolumeMounts...)
		}
	}

	// Override volumes if specified
	if len(userPodSpec.Volumes) > 0 {
		podSpec.Volumes = append(podSpec.Volumes, userPodSpec.Volumes...)
	}

	// Override security context if specified
	if userPodSpec.SecurityContext != nil {
		podSpec.SecurityContext = userPodSpec.SecurityContext
	}

	// Override init containers if specified
	if len(userPodSpec.InitContainers) > 0 {
		podSpec.InitContainers = append(podSpec.InitContainers, userPodSpec.InitContainers...)
	}

	// Override image pull secrets if specified
	if len(userPodSpec.ImagePullSecrets) > 0 {
		podSpec.ImagePullSecrets = append(podSpec.ImagePullSecrets, userPodSpec.ImagePullSecrets...)
	}

	// Override liveness probe if specified
	if userPodSpec.LivenessProbe != nil {
		if len(podSpec.Containers) > 0 {
			podSpec.Containers[0].LivenessProbe = userPodSpec.LivenessProbe
		}
	}

	// Override readiness probe if specified
	if userPodSpec.ReadinessProbe != nil {
		if len(podSpec.Containers) > 0 {
			podSpec.Containers[0].ReadinessProbe = userPodSpec.ReadinessProbe
		}
	}

	// Override affinity if specified
	if userPodSpec.Affinity != nil {
		podSpec.Affinity = userPodSpec.Affinity
	}

	// Override anti-affinity if specified
	if userPodSpec.AntiAffinity != nil {
		if podSpec.Affinity == nil {
			podSpec.Affinity = &corev1.Affinity{}
		}
		podSpec.Affinity.PodAntiAffinity = userPodSpec.AntiAffinity
	}

	// Override node selector if specified
	if len(userPodSpec.NodeSelector) > 0 {
		podSpec.NodeSelector = userPodSpec.NodeSelector
	}

	// Override tolerations if specified
	if len(userPodSpec.Tolerations) > 0 {
		podSpec.Tolerations = append(podSpec.Tolerations, userPodSpec.Tolerations...)
	}

	// Override topology spread constraints if specified
	if len(userPodSpec.TopologySpreadConstraints) > 0 {
		podSpec.TopologySpreadConstraints = append(podSpec.TopologySpreadConstraints, userPodSpec.TopologySpreadConstraints...)
	}
}
