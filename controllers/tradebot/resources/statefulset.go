package resources

import (
	"context"
	"reflect"
	"time"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// mergePodSpecOverrides merges user overrides from App.PodSpec into the default PodSpec.
func mergePodSpecOverrides(defaultPodSpec corev1.PodSpec, override *freqtradev1alpha1.PodSpec) corev1.PodSpec {
	if override == nil {
		return defaultPodSpec
	}

	if override.Image != "" {
		defaultPodSpec.Containers[0].Image = override.Image
	}

	// Only override fields that are non-zero in the override struct
	if !reflect.DeepEqual(override.Resources, corev1.ResourceRequirements{}) {
		defaultPodSpec.Containers[0].Resources = override.Resources
	}
	if len(override.Env) > 0 {
		defaultPodSpec.Containers[0].Env = override.Env
	}
	if len(override.VolumeMounts) > 0 {
		defaultPodSpec.Containers[0].VolumeMounts = override.VolumeMounts
	}
	if len(override.Volumes) > 0 {
		defaultPodSpec.Volumes = override.Volumes
	}
	if override.SecurityContext != nil {
		defaultPodSpec.SecurityContext = override.SecurityContext
	}
	if len(override.InitContainers) > 0 {
		defaultPodSpec.InitContainers = override.InitContainers
	}

	if len(override.ImagePullSecrets) > 0 {
		defaultPodSpec.ImagePullSecrets = override.ImagePullSecrets
	}
	if override.LivenessProbe != nil {
		defaultPodSpec.Containers[0].LivenessProbe = override.LivenessProbe
	}
	if override.ReadinessProbe != nil {
		defaultPodSpec.Containers[0].ReadinessProbe = override.ReadinessProbe
	}

	if override.Affinity != nil {
		defaultPodSpec.Affinity = override.Affinity
	}
	if len(override.NodeSelector) > 0 {
		defaultPodSpec.NodeSelector = override.NodeSelector
	}
	if len(override.Tolerations) > 0 {
		defaultPodSpec.Tolerations = override.Tolerations
	}
	if len(override.TopologySpreadConstraints) > 0 {
		defaultPodSpec.TopologySpreadConstraints = override.TopologySpreadConstraints
	}

	return defaultPodSpec
}

// BuildStatefulSet creates a StatefulSet for the bot, merging App.PodSpec overrides.
func BuildStatefulSet(ctx context.Context, c client.Client, tradeBot freqtradev1alpha1.TradeBot, configSecretName, strategyConfigMapName, pvcName string) appsv1.StatefulSet {
	replicas := int32(1)

	// Fetch the referenced Strategy resource
	var strategy freqtradev1alpha1.Strategy
	err := c.Get(ctx, types.NamespacedName{Namespace: tradeBot.Namespace, Name: tradeBot.Spec.Strategy}, &strategy)
	if err != nil {
		if errors.IsNotFound(err) {
			return appsv1.StatefulSet{}
		}
		panic(err)
	}
	strategyName := strategy.Spec.Name

	image := "freqtradeorg/freqtrade:stable"
	if tradeBot.Spec.App != nil &&
		tradeBot.Spec.App.PodSpec != nil &&
		len(tradeBot.Spec.App.PodSpec.VolumeMounts) > 0 {
	}

	command := tradeBot.Spec.FreqtradeCommand
	arguments := tradeBot.Spec.FreqtradeArguments

	// TODO: Validate command and arguments (duplicates, etc.)
	if command == "" {
		command = "trade"
	}

	args := []string{
		command,
		"--config", "/config/config.json",
		"--strategy-path", "/strategy",
		"--strategy", strategyName,
		"--db-url", "sqlite:////freqtrade/user_data/tradesv3.sqlite",
		"--logfile", "/freqtrade/user_data/logs/freqtrade.log",
	}

	if len(arguments) > 0 {
		args = append(args, arguments...)
	}

	runAsUser := int64(1000)
	runAsGroup := int64(1000)
	fsGroup := int64(1000)
	runAsRoot := int64(0)
	runAsRootGroup := int64(0)

	defaultPodSpec := corev1.PodSpec{
		SecurityContext: &corev1.PodSecurityContext{
			RunAsUser:  &runAsUser,
			RunAsGroup: &runAsGroup,
			FSGroup:    &fsGroup,
		},
		InitContainers: []corev1.Container{
			{
				Name:            "init-user-data",
				Image:           "busybox:latest",
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:  &runAsRoot,
					RunAsGroup: &runAsRootGroup,
				},
				Command: []string{
					"sh",
					"-c",
					"mkdir -p /freqtrade/user_data/logs /freqtrade/user_data/data && chmod -R 775 /freqtrade/user_data && chown -R 1000:1000 /freqtrade/user_data",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "user-data",
						MountPath: "/freqtrade/user_data",
					},
				},
			},
		},
		Containers: []corev1.Container{
			{
				Name:            "freqtrade",
				Image:           image,
				ImagePullPolicy: corev1.PullAlways,
				Command: []string{
					"freqtrade",
				},
				Args: args,
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "config",
						MountPath: "/config",
						ReadOnly:  true,
					},
					{
						Name:      "strategy",
						MountPath: "/strategy",
						ReadOnly:  true,
					},
					{
						Name:      "user-data",
						MountPath: "/freqtrade/user_data",
					},
				},
				Ports: []corev1.ContainerPort{
					{
						ContainerPort: 8080,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/api/v1/ping",
							Port:   intstr.FromInt32(8080),
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds: 60,
					PeriodSeconds:       60,
					TimeoutSeconds:      5,
					FailureThreshold:    3,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/api/v1/ping",
							Port:   intstr.FromInt32(8080),
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds: 30,
					PeriodSeconds:       30,
					TimeoutSeconds:      5,
					FailureThreshold:    3,
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: configSecretName,
					},
				},
			},
			{
				Name: "strategy",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: strategyConfigMapName,
						},
					},
				},
			},
			{
				Name: "user-data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			},
		},
	}

	// Merge PodSpec overrides from App.PodSpec
	if tradeBot.Spec.App != nil && tradeBot.Spec.App.PodSpec != nil {
		defaultPodSpec = mergePodSpecOverrides(defaultPodSpec, tradeBot.Spec.App.PodSpec)
	}

	baseStatefulSetSpec := appsv1.StatefulSetSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
		},
		ServiceName: tradeBot.Name,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
			},
			Spec: defaultPodSpec,
		},
	}

	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name,
			Namespace: tradeBot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: baseStatefulSetSpec,
	}
}

// ApplyStatefulSet creates or updates the StatefulSet with retry logic for resource version conflicts
func ApplyStatefulSet(ctx context.Context, c client.Client, sts *appsv1.StatefulSet) error {
	logger := log.FromContext(ctx)

	// Retry logic for resource version conflicts
	return wait.PollImmediate(100*time.Millisecond, 2*time.Second, func() (bool, error) {
		var existing appsv1.StatefulSet
		err := c.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, &existing)
		if errors.IsNotFound(err) {
			err = c.Create(ctx, sts)
			if errors.IsAlreadyExists(err) {
				// Resource was created by another reconciliation, retry
				logger.V(1).Info("StatefulSet already exists, retrying", "name", sts.Name)
				return false, nil
			}
			return true, err
		} else if err != nil {
			return true, err
		}

		needsUpdate := false

		if !reflect.DeepEqual(existing.Spec.Template.Spec, sts.Spec.Template.Spec) {
			needsUpdate = true
		}
		if !reflect.DeepEqual(existing.Spec.Selector, sts.Spec.Selector) {
			needsUpdate = true
		}
		if existing.Spec.Replicas == nil || sts.Spec.Replicas == nil || *existing.Spec.Replicas != *sts.Spec.Replicas {
			needsUpdate = true
		}

		if needsUpdate {
			sts.ResourceVersion = existing.ResourceVersion
			err = c.Update(ctx, sts)
			if errors.IsConflict(err) {
				// Resource version conflict, retry
				logger.V(1).Info("StatefulSet resource version conflict, retrying", "name", sts.Name)
				return false, nil
			}
			return true, err
		}

		return true, nil
	})
}
