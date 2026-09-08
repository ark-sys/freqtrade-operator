package resources

import (
	"context"
	"reflect"
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
			if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 1 {
				t.Errorf("expected BackoffLimit 1, got %v", job.Spec.BackoffLimit)
			}
			if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 86400 {
				t.Errorf("expected TTLSecondsAfterFinished 86400, got %v", job.Spec.TTLSecondsAfterFinished)
			}
		})
	}
}

// TestApplyJob_CreateIfAbsentNeverUpdates covers the P0-4 fix: batchv1.Job's
// Spec.Template is immutable after creation, so a second ApplyJob call with a
// different desired spec (e.g. changed freqtradeArguments) must not attempt
// an Update - it must leave the running Job untouched and report the
// existing object back to the caller instead.
func TestApplyJob_CreateIfAbsentNeverUpdates(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := batchv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add batchv1 to scheme: %v", err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	tradeBot := freqtradev1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "my-bot", Namespace: "trading"},
		Spec: freqtradev1alpha1.TradeBotSpec{
			FreqtradeCommand:   "backtesting",
			FreqtradeArguments: []string{"--timerange", "20240101-20240201"},
		},
	}
	firstDesired := BuildJob(tradeBot, "MyStrategy", "my-bot-config", "my-bot-strategy", "")
	firstApplied := firstDesired.DeepCopy()
	if err := ApplyJob(context.Background(), c, firstApplied); err != nil {
		t.Fatalf("unexpected error creating Job: %v", err)
	}

	// Simulate the user changing freqtradeArguments after the Job was created.
	tradeBot.Spec.FreqtradeArguments = []string{"--timerange", "20240201-20240301"}
	secondDesired := BuildJob(tradeBot, "MyStrategy", "my-bot-config", "my-bot-strategy", "")
	secondApplied := secondDesired.DeepCopy()
	if err := ApplyJob(context.Background(), c, secondApplied); err != nil {
		t.Fatalf("unexpected error on second ApplyJob call: %v", err)
	}

	// The caller-visible object must be what's actually running (the first
	// spec), not the newly-desired one.
	if !reflect.DeepEqual(secondApplied.Spec.Template.Spec, firstDesired.Spec.Template.Spec) {
		t.Error("expected ApplyJob to report the existing (first) Job back to the caller")
	}
	if reflect.DeepEqual(secondApplied.Spec.Template.Spec, secondDesired.Spec.Template.Spec) {
		t.Error("expected ApplyJob to NOT silently apply the newly-desired spec")
	}

	var stored batchv1.Job
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(&firstDesired), &stored); err != nil {
		t.Fatalf("failed to get stored Job: %v", err)
	}
	if !reflect.DeepEqual(stored.Spec.Template.Spec, firstDesired.Spec.Template.Spec) {
		t.Error("expected the stored Job's template to remain the first-created one, unmodified")
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
