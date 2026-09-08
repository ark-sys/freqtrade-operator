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
	if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].Name != "my-bot" {
		t.Errorf("expected a single owner reference to my-bot, got %+v", secret.OwnerReferences)
	}
	if secret.StringData["config.json"] != secretData["config.json"] {
		t.Errorf("expected StringData to round-trip, got %v", secret.StringData)
	}
}
