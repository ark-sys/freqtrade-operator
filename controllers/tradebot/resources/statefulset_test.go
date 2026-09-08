package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildStatefulSet(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			// StatefulSet must always run in trade mode, regardless of this field.
			FreqtradeCommand: "backtesting",
			Strategy:         "my-strategy",
			Config:           "my-config",
		},
	}

	sts := BuildStatefulSet(tradeBot, "MyStrategy", "my-bot-config", "my-bot-strategy", "my-bot-data")

	if sts.Name != "my-bot" {
		t.Errorf("expected Name %q, got %q", "my-bot", sts.Name)
	}
	if sts.Namespace != "trading" {
		t.Errorf("expected Namespace %q, got %q", "trading", sts.Namespace)
	}
	if len(sts.OwnerReferences) != 1 || sts.OwnerReferences[0].Name != "my-bot" {
		t.Errorf("expected a single owner reference to my-bot, got %+v", sts.OwnerReferences)
	}
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %+v", sts.Spec.Replicas)
	}

	container := sts.Spec.Template.Spec.Containers[0]
	if got := container.Args[0]; got != "trade" {
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

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
