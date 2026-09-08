package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildJob(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		wantCommand string
	}{
		{name: "explicit backtesting command", command: "backtesting", wantCommand: "backtesting"},
		{name: "empty command defaults to trade", command: "", wantCommand: "trade"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tradeBot := freqtradev1alpha1.TradeBot{
				ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
				Spec: freqtradev1alpha1.TradeBotSpec{
					FreqtradeCommand: tt.command,
					Strategy:         "my-strategy",
					Config:           "my-config",
				},
			}

			job := BuildJob(tradeBot, "MyStrategy", "my-bot-config", "my-bot-strategy", "")

			if job.Name != "my-bot" {
				t.Errorf("expected Name %q, got %q", "my-bot", job.Name)
			}
			if job.Namespace != "trading" {
				t.Errorf("expected Namespace %q, got %q", "trading", job.Namespace)
			}
			if len(job.OwnerReferences) != 1 || job.OwnerReferences[0].Name != "my-bot" {
				t.Errorf("expected a single owner reference to my-bot, got %+v", job.OwnerReferences)
			}

			container := job.Spec.Template.Spec.Containers[0]
			if got := container.Args[0]; got != tt.wantCommand {
				t.Errorf("expected freqtrade subcommand %q, got %q", tt.wantCommand, got)
			}
			if job.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyOnFailure {
				t.Errorf("expected RestartPolicy OnFailure, got %q", job.Spec.Template.Spec.RestartPolicy)
			}
		})
	}
}

func TestBuildJob_NoPVCUsesEmptyDir(t *testing.T) {
	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			FreqtradeCommand: "backtesting",
			Strategy:         "my-strategy",
			Config:           "my-config",
		},
	}

	job := BuildJob(tradeBot, "MyStrategy", "my-bot-config", "my-bot-strategy", "")

	found := false
	for _, v := range job.Spec.Template.Spec.Volumes {
		if v.Name == "user-data" {
			found = true
			if v.EmptyDir == nil {
				t.Errorf("expected user-data to be an emptyDir when no PVC name is given, got %+v", v.VolumeSource)
			}
		}
	}
	if !found {
		t.Error("expected a user-data volume")
	}
}
