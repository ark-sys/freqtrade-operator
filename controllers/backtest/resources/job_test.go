package resources

import (
	"testing"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildJob_NameMatchesBacktestNameExactly(t *testing.T) {
	backtest := freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run", Namespace: "trading"}}
	job := BuildJob(backtest, "img", "SampleStrategy", "my-run-config", "my-run-strategy")

	if job.Name != "my-run" {
		t.Errorf("expected Job name to exactly match the Backtest name (no hash suffix - "+
			"spec is immutable), got %q", job.Name)
	}
	if job.Namespace != "trading" {
		t.Errorf("expected namespace trading, got %s", job.Namespace)
	}
}

func TestBuildJob_DefaultTTLWhenUnset(t *testing.T) {
	backtest := freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run"}}
	job := BuildJob(backtest, "img", "SampleStrategy", "cfg", "strategy-cm")

	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != defaultTTLSecondsAfterFinished {
		t.Errorf("expected default TTL %d, got %v", defaultTTLSecondsAfterFinished, job.Spec.TTLSecondsAfterFinished)
	}
}

func TestBuildJob_ExplicitTTLOverridesDefault(t *testing.T) {
	ttl := int32(3600)
	backtest := freqtradev1beta1.Backtest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-run"},
		Spec: freqtradev1beta1.BacktestSpec{
			RunSpec: freqtradev1beta1.RunSpec{TTLSecondsAfterFinished: &ttl},
		},
	}
	job := BuildJob(backtest, "img", "SampleStrategy", "cfg", "strategy-cm")

	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != ttl {
		t.Errorf("expected explicit TTL %d, got %v", ttl, job.Spec.TTLSecondsAfterFinished)
	}
}

func TestBuildJob_BackoffLimitAllowsOneRetry(t *testing.T) {
	backtest := freqtradev1beta1.Backtest{ObjectMeta: metav1.ObjectMeta{Name: "my-run"}}
	job := BuildJob(backtest, "img", "SampleStrategy", "cfg", "strategy-cm")

	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 1 {
		t.Errorf("expected BackoffLimit 1, got %v", job.Spec.BackoffLimit)
	}
}
