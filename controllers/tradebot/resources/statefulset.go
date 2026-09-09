package resources

import (
	corev1 "k8s.io/api/core/v1"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildStatefulSet creates a StatefulSet for the bot, merging App.PodSpec overrides.
// strategyName is the resolved Strategy.Spec.Name; the caller is responsible for
// having already fetched and validated the referenced Strategy exists.
func BuildStatefulSet(tradeBot freqtradev1alpha1.TradeBot, strategyName, configSecretName, strategyConfigMapName, pvcName string) appsv1.StatefulSet {
	replicas := int32(1)

	// Build the reusable PodSpec for trade mode.
	podSpec := BuildPod(
		tradeBot,
		strategyName,
		configSecretName,
		strategyConfigMapName,
		pvcName,
		"trade", // force long-running mode here
		tradeBot.Spec.FreqtradeArguments,
	)

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
