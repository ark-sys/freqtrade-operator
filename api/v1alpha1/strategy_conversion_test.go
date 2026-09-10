package v1alpha1

import (
	"testing"
	"time"

	"github.com/ark-sys/freqtrade-operator/api/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/randfill"
)

// strategyFuzzer fills a Strategy with random-but-valid data for the
// round-trip tests below - same Funcs as tradeBotFuzzer
// (tradebot_conversion_test.go) and for the same reasons.
func strategyFuzzer(seed int64) *randfill.Filler {
	return randfill.NewWithSeed(seed).NilChance(0.5).NumElements(1, 3).Funcs(
		func(m *[]metav1.ManagedFieldsEntry, c randfill.Continue) {
			*m = nil
		},
		func(t *metav1.Time, c randfill.Continue) {
			*t = metav1.NewTime(metav1.Now().Truncate(time.Second))
		},
	)
}

// TestStrategyConvertRoundTrip_AlphaToBeta covers B3's own requirement: a
// v1alpha1 Strategy must round-trip through v1beta1 and back without
// losing anything - Strategy has no fields v1beta1 can't represent, unlike
// TradeBot/TradeBotConfig, so unlike their round-trip tests this one
// doesn't need to clear anything before converting.
func TestStrategyConvertRoundTrip_AlphaToBeta(t *testing.T) {
	for i := 0; i < 200; i++ {
		original := &Strategy{}
		strategyFuzzer(int64(i)).Fill(original)
		original.TypeMeta = metav1.TypeMeta{} // not part of any conversion function's contract

		var converted v1beta1.Strategy
		if err := original.ConvertTo(&converted); err != nil {
			t.Fatalf("iteration %d: ConvertTo failed: %v", i, err)
		}

		roundTripped := &Strategy{}
		if err := roundTripped.ConvertFrom(&converted); err != nil {
			t.Fatalf("iteration %d: ConvertFrom failed: %v", i, err)
		}

		if !apiequality.Semantic.DeepEqual(original, roundTripped) {
			t.Errorf("iteration %d: round trip mismatch.\noriginal:  %+v\nroundtrip: %+v", i, original, roundTripped)
		}
	}
}

// TestStrategyConvertRoundTrip_BetaToAlpha covers the other direction.
func TestStrategyConvertRoundTrip_BetaToAlpha(t *testing.T) {
	for i := 0; i < 200; i++ {
		original := &v1beta1.Strategy{}
		strategyFuzzer(int64(i)).Fill(original)
		original.TypeMeta = metav1.TypeMeta{}

		converted := &Strategy{}
		if err := converted.ConvertFrom(original); err != nil {
			t.Fatalf("iteration %d: ConvertFrom failed: %v", i, err)
		}

		roundTripped := &v1beta1.Strategy{}
		if err := converted.ConvertTo(roundTripped); err != nil {
			t.Fatalf("iteration %d: ConvertTo failed converting back: %v", i, err)
		}

		if !apiequality.Semantic.DeepEqual(original, roundTripped) {
			t.Errorf("iteration %d: round trip mismatch.\noriginal:  %+v\nroundtrip: %+v", i, original, roundTripped)
		}
	}
}
