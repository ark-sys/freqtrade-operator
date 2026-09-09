package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildSecret(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
	}
	secretData := map[string]string{"config.json": `{"stake_currency":"USDT"}`}

	secret := BuildSecret(tradeBot, secretData)

	if secret.Name != "my-bot-config" {
		t.Errorf("expected Name %q, got %q", "my-bot-config", secret.Name)
	}
	if secret.Namespace != "trading" {
		t.Errorf("expected Namespace %q, got %q", "trading", secret.Namespace)
	}
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("expected type Opaque, got %q", secret.Type)
	}
	if secret.StringData["config.json"] != secretData["config.json"] {
		t.Errorf("expected StringData to round-trip, got %v", secret.StringData)
	}
	if secret.Labels[ContainsCredentialsLabel] != "true" {
		t.Errorf("expected %s=true so backup tooling can exclude it, got %v", ContainsCredentialsLabel, secret.Labels)
	}
}
