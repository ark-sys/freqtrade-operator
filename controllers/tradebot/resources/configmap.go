package resources

import (
	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildStrategyConfigMap creates a ConfigMap for the bot's strategy script
func BuildStrategyConfigMap(tradeBot freqtradev1alpha1.TradeBot, strategy freqtradev1alpha1.Strategy) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name + "-strategy",
			Namespace: tradeBot.Namespace,
		},
		Data: map[string]string{
			strategy.Spec.Name + ".py": strategy.Spec.Script,
		},
	}
}
