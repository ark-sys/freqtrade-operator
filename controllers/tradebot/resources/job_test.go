package resources

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

// TestIsJobTemplateImmutableError covers the P2-1 replacement for the old
// ApplyJob create-if-absent-never-update dance: since server-side apply
// enforces batchv1.Job.spec.template immutability the same way a typed
// Update does (verified empirically against envtest - see
// resources.go/reconcileResources), a rejected apply with this exact error
// shape is how reconcileResources learns the Job's spec drifted, not a
// hand-rolled spec comparison.
func TestIsJobTemplateImmutableError(t *testing.T) {
	immutableErr := apierrors.NewInvalid(
		schema.GroupKind{Group: "batch", Kind: "Job"},
		"my-bot",
		field.ErrorList{field.Invalid(field.NewPath("spec").Child("template"), nil, "field is immutable")},
	)
	otherInvalidErr := apierrors.NewInvalid(
		schema.GroupKind{Group: "batch", Kind: "Job"},
		"my-bot",
		field.ErrorList{field.Invalid(field.NewPath("spec").Child("backoffLimit"), nil, "must be non-negative")},
	)

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "template immutable rejection", err: immutableErr, want: true},
		{name: "a different Invalid error", err: otherInvalidErr, want: false},
		{
			name: "a NotFound error",
			err:  apierrors.NewNotFound(schema.GroupResource{Group: "batch", Resource: "jobs"}, "my-bot"),
			want: false,
		},
		{name: "nil error", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsJobTemplateImmutableError(tt.err); got != tt.want {
				t.Errorf("IsJobTemplateImmutableError() = %v, want %v", got, tt.want)
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
