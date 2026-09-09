package resources

import (
	corev1 "k8s.io/api/core/v1"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigHashAnnotation records, on the StatefulSet pod template, the hash
// (see TradeBotStatus.AppliedConfigHash) of the config that template
// reflects. Changing it is what a config change under
// spec.updateStrategy: Auto uses to trigger a StatefulSet rolling update -
// under Manual, the reconciler deliberately passes the template's own
// existing value back through BuildStatefulSet instead of the fresh one, so
// this annotation - and therefore the template - doesn't change (see
// reconcileResources).
const ConfigHashAnnotation = "freqtrade.io/config-hash"

// BuildStatefulSet creates a StatefulSet for the bot, merging App.PodSpec
// overrides. image is the resolved freqtrade image (see BuildPod).
// strategyName is the resolved Strategy.Spec.Name; the caller is
// responsible for having already fetched and validated the referenced
// Strategy exists. configHash is written onto the pod template as
// ConfigHashAnnotation when non-empty - see reconcileResources for how the
// caller picks which hash value that is.
func BuildStatefulSet(
	tradeBot freqtradev1alpha1.TradeBot,
	image, strategyName, configSecretName, strategyConfigMapName, pvcName, configHash string,
) appsv1.StatefulSet {
	replicas := int32(1)

	// Build the reusable PodSpec for trade mode.
	podSpec := BuildPod(
		tradeBot,
		image,
		strategyName,
		configSecretName,
		strategyConfigMapName,
		pvcName,
		"trade", // force long-running mode here
		tradeBot.Spec.FreqtradeArguments,
	)

	templateAnnotations := map[string]string{}
	if configHash != "" {
		templateAnnotations[ConfigHashAnnotation] = configHash
	}

	baseStatefulSetSpec := appsv1.StatefulSetSpec{
		Replicas: &replicas,
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
		},
		ServiceName: tradeBot.Name,
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      map[string]string{"name": tradeBot.Name, "app": "freqtrade"},
				Annotations: templateAnnotations,
			},
			Spec: podSpec,
		},
	}

	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name,
			Namespace: tradeBot.Namespace,
		},
		Spec: baseStatefulSetSpec,
	}
}
