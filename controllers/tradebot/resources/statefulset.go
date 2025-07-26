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

// BuildStatefulSet creates a StatefulSet for the bot
func BuildStatefulSet(ctx context.Context, c client.Client, tradeBot freqtradev1alpha1.TradeBot, configMapName, strategyConfigMapName, pvcName string) appsv1.StatefulSet {
	replicas := int32(1)

	// Fetch the referenced Strategy resource
	var strategy freqtradev1alpha1.Strategy
	err := c.Get(ctx, types.NamespacedName{Namespace: tradeBot.Namespace, Name: tradeBot.Spec.References.StrategyRef}, &strategy)
	if err != nil {
		if errors.IsNotFound(err) {
			// Strategy not found, return an error
			return appsv1.StatefulSet{}
		}
		// Other error occurred, return it
		panic(err)
	}
	strategyName := strategy.Spec.Name

	// Set default image if not specified
	image := "freqtradeorg/freqtrade:stable"
	if tradeBot.Spec.App != nil &&
		len(tradeBot.Spec.App.StatefulSetSpec.Template.Spec.Containers) > 0 &&
		tradeBot.Spec.App.StatefulSetSpec.Template.Spec.Containers[0].Image != "" {
		image = tradeBot.Spec.App.StatefulSetSpec.Template.Spec.Containers[0].Image
	}

	// Prepare command arguments
	args := []string{
		"trade",
		"--config", "/config/config.json",
		"--strategy-path", "/strategy",
		"--strategy", strategyName,
		"--db-url", "sqlite:////freqtrade/user_data/tradesv3.sqlite",
		"--logfile", "/freqtrade/user_data/logs/freqtrade.log",
	}

	// Set security context for running as non-root
	runAsUser := int64(1000)
	runAsGroup := int64(1000)
	fsGroup := int64(1000)

	// Set security context for running init container as root
	runAsRoot := int64(0)
	runAsRootGroup := int64(0)

	baseStatefulSetSpec := appsv1.StatefulSetSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": tradeBot.Name},
		},
		ServiceName: tradeBot.Name,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": tradeBot.Name},
			},
			Spec: corev1.PodSpec{
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
									Port:   intstr.FromInt(8080),
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
									Port:   intstr.FromInt(8080),
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
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: configMapName,
								},
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
			},
		},
	}

	// Merge with user-provided StatefulSet configuration
	finalSpec := baseStatefulSetSpec
	if tradeBot.Spec.App != nil && !reflect.DeepEqual(tradeBot.Spec.App.StatefulSetSpec, appsv1.StatefulSetSpec{}) {
		finalSpec = shared.MergeSpecsWithStrategicPatch(baseStatefulSetSpec, tradeBot.Spec.App.StatefulSetSpec, &appsv1.StatefulSetSpec{})
	}

	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name,
			Namespace: tradeBot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: finalSpec,
	}
}

// ApplyStatefulSet creates or updates the StatefulSet
func ApplyStatefulSet(ctx context.Context, c client.Client, sts *appsv1.StatefulSet) error {
	var existing appsv1.StatefulSet
	err := c.Get(ctx, types.NamespacedName{Name: sts.Name, Namespace: sts.Namespace}, &existing)
	if errors.IsNotFound(err) {
		return c.Create(ctx, sts)
	} else if err != nil {
		return err
	}

	// Check if update is needed by comparing relevant fields
	needsUpdate := false

	// Compare spec fields that matter
	if !reflect.DeepEqual(existing.Spec.Template.Spec, sts.Spec.Template.Spec) {
		needsUpdate = true
	}

	if !reflect.DeepEqual(existing.Spec.Selector, sts.Spec.Selector) {
		needsUpdate = true
	}

	if existing.Spec.Replicas == nil || sts.Spec.Replicas == nil || *existing.Spec.Replicas != *sts.Spec.Replicas {
		needsUpdate = true
	}

	// Only update if there are actual changes
	if needsUpdate {
		sts.ResourceVersion = existing.ResourceVersion
		return c.Update(ctx, sts)
	}

	return nil
}
