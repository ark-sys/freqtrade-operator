package resources

import (
	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainsCredentialsLabel marks a resource as holding live exchange/API
// credentials (P3-2), so a backup tool can be configured to exclude it -
// see the README's "Config Secret backups" section.
const ContainsCredentialsLabel = "freqtrade.io/contains-credentials"

// labelValueTrue is the canonical string value k8s labels/annotations use
// for a boolean flag - labels and annotations are always strings.
const labelValueTrue = "true"

// appLabelKey and freqtradeAppName are the "app" label every TradeBot-owned
// resource in this package carries, so selectors (Service, NetworkPolicy)
// and the resources they select always agree on the same key/value pair.
const (
	appLabelKey      = "app"
	freqtradeAppName = "freqtrade"
	nameLabelKey     = "name"
)

// BuildSecret creates a Secret for the bot's config.json
func BuildSecret(tradeBot freqtradev1alpha1.TradeBot, secretData map[string]string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tradeBot.Name + "-config",
			Namespace: tradeBot.Namespace,
			Labels: map[string]string{
				appLabelKey:              freqtradeAppName,
				nameLabelKey:             tradeBot.Name,
				ContainsCredentialsLabel: labelValueTrue,
			},
		},
		StringData: secretData,
		Type:       corev1.SecretTypeOpaque,
	}
}
