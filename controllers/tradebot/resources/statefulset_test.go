package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildStatefulSet(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace},
		Spec: freqtradev1alpha1.TradeBotSpec{
			// StatefulSet must always run in trade mode, regardless of this field.
			FreqtradeCommand: "backtesting",
			Strategy:         "my-strategy",
			Config:           "my-config",
		},
	}

	sts := BuildStatefulSet(tradeBot, testImage, "MyStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", "abc123")

	if got := sts.Spec.Template.Annotations[ConfigHashAnnotation]; got != "abc123" {
		t.Errorf("expected %s annotation %q, got %q", ConfigHashAnnotation, "abc123", got)
	}
	if sts.Name != testBotName {
		t.Errorf("expected Name %q, got %q", testBotName, sts.Name)
	}
	if sts.Namespace != testNamespace {
		t.Errorf("expected Namespace %q, got %q", testNamespace, sts.Namespace)
	}
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %+v", sts.Spec.Replicas)
	}

	container := sts.Spec.Template.Spec.Containers[0]
	if got := container.Args[0]; got != freqCommandTrade {
		t.Errorf("expected StatefulSet to always run in trade mode regardless of spec.freqtrade_command, got args[0]=%q", got)
	}
	if !containsArg(container.Args, "MyStrategy") {
		t.Errorf("expected resolved strategy name in args, got %v", container.Args)
	}

	found := false
	for _, v := range sts.Spec.Template.Spec.Volumes {
		if v.Name == "user-data" {
			found = true
			if v.PersistentVolumeClaim == nil || v.PersistentVolumeClaim.ClaimName != "my-bot-data" {
				t.Errorf("expected user-data volume backed by PVC %q, got %+v", "my-bot-data", v.VolumeSource)
			}
		}
	}
	if !found {
		t.Error("expected a user-data volume")
	}
}

func TestBuildStatefulSet_EmptyConfigHashSetsNoAnnotation(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{ObjectMeta: metav1.ObjectMeta{Name: testBotName, Namespace: testNamespace}}

	sts := BuildStatefulSet(tradeBot, testImage, "MyStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data", "")

	if _, ok := sts.Spec.Template.Annotations[ConfigHashAnnotation]; ok {
		t.Errorf(
			"expected no %s annotation when configHash is empty, got %+v", ConfigHashAnnotation, sts.Spec.Template.Annotations,
		)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
