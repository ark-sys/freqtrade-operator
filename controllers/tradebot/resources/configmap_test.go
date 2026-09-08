package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildStrategyConfigMap(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
	}
	strategy := freqtradev1alpha1.Strategy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-strategy", Namespace: "trading"},
		Spec:       freqtradev1alpha1.StrategySpec{Name: "SampleStrategy", Script: "class SampleStrategy:\n    pass"},
	}

	cm := BuildStrategyConfigMap(tradeBot, strategy)

	if cm.Name != "my-bot-strategy" {
		t.Errorf("expected Name %q, got %q", "my-bot-strategy", cm.Name)
	}
	if cm.Namespace != "trading" {
		t.Errorf("expected Namespace %q, got %q", "trading", cm.Namespace)
	}
	if len(cm.OwnerReferences) != 1 || cm.OwnerReferences[0].Name != "my-bot" {
		t.Errorf("expected a single owner reference to my-bot, got %+v", cm.OwnerReferences)
	}
	script, ok := cm.Data["SampleStrategy.py"]
	if !ok {
		t.Fatalf("expected a SampleStrategy.py key, got %v", cm.Data)
	}
	if script != strategy.Spec.Script {
		t.Errorf("expected the script content to round-trip, got %q", script)
	}
}
