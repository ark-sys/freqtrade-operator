// controllers/backtest/resources/secret.go
package resources

import (
	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainsCredentialsLabel marks a resource as holding live exchange/API
// credentials, mirroring controllers/tradebot/resources' label of the same
// name (P3-2) - a backup tool excluding one by this label should exclude
// both.
const ContainsCredentialsLabel = "freqtrade.io/contains-credentials"

// BuildConfigSecret creates the Secret holding this run's rendered config.json.
func BuildConfigSecret(backtest freqtradev1beta1.Backtest, secretData map[string]string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backtest.Name + "-config",
			Namespace: backtest.Namespace,
			Labels: map[string]string{
				"app":                    "freqtrade-backtest",
				"name":                   backtest.Name,
				ContainsCredentialsLabel: "true",
			},
		},
		StringData: secretData,
		Type:       corev1.SecretTypeOpaque,
	}
}
