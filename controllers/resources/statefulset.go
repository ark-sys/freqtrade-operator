package resources

import (
	"context"
	"path/filepath"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildStatefulSet creates a StatefulSet for the bot
func BuildStatefulSet(tradeBot freqtradev1alpha1.TradeBot, configMapName, strategyConfigMapName, pvcName string) appsv1.StatefulSet {
	replicas := int32(1)
	strategyName := filepath.Base(tradeBot.Spec.StrategyRef)

	// Set default image if not specified
	image := "freqtradeorg/freqtrade:stable"
	if tradeBot.Spec.Image != "" {
		image = tradeBot.Spec.Image
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

	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name,
			Namespace: tradeBot.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(&tradeBot, freqtradev1alpha1.GroupVersion.WithKind("TradeBot")),
			},
		},
		Spec: appsv1.StatefulSetSpec{
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
					Containers: []corev1.Container{
						{
							Name:  "freqtrade",
							Image: image,
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
							Resources: tradeBot.Spec.Resources,
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
		},
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
	sts.ResourceVersion = existing.ResourceVersion
	return c.Update(ctx, sts)
}
