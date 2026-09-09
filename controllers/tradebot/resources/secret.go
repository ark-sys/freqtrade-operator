package resources

import (
	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildSecret creates a Secret for the bot's config.json
func BuildSecret(tradeBot freqtradev1alpha1.TradeBot, secretData map[string]string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name + "-config",
			Namespace: tradeBot.Namespace,
			Labels: map[string]string{
				"app":  "freqtrade",
				"name": tradeBot.Name,
			},
		},
		StringData: secretData,
		Type:       corev1.SecretTypeOpaque,
	}
}
