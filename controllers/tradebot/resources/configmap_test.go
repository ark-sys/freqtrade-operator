package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildStrategyConfigMap(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace},
	}
	strategy := freqtradev1alpha1.Strategy{
		ObjectMeta: metav1.ObjectMeta{Name: "my-strategy", Namespace: testNamespace},
		Spec:       freqtradev1alpha1.StrategySpec{Name: "SampleStrategy", Script: "class SampleStrategy:\n    pass"},
	}

	cm := BuildStrategyConfigMap(tradeBot, strategy)

	if cm.Name != "my-bot-strategy" {
		t.Errorf("expected Name %q, got %q", "my-bot-strategy", cm.Name)
	}
	if cm.Namespace != testNamespace {
		t.Errorf("expected Namespace %q, got %q", testNamespace, cm.Namespace)
	}
	script, ok := cm.Data["SampleStrategy.py"]
	if !ok {
		t.Fatalf("expected a SampleStrategy.py key, got %v", cm.Data)
	}
	if script != strategy.Spec.Script {
		t.Errorf("expected the script content to round-trip, got %q", script)
	}
}
