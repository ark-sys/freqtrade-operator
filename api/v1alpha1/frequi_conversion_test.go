package v1alpha1

import (
	"fmt"
	"testing"
	"time"

	"github.com/ark-sys/freqtrade-operator/api/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/randfill"
)

// freqUIFuzzer fills a FreqUI with random-but-valid data for the
// round-trip tests below - same Funcs as tradeBotFuzzer
// (tradebot_conversion_test.go) and for the same reasons: FUPodSpec.Resources
// is a corev1.ResourceRequirements, which carries resource.Quantity values
// with the same cached-string-representation invariant.
func freqUIFuzzer(seed int64) *randfill.Filler {
	return randfill.NewWithSeed(seed).NilChance(0.5).NumElements(1, 3).Funcs(
		func(q *resource.Quantity, c randfill.Continue) {
			*q = resource.MustParse(fmt.Sprintf("%dm", c.Intn(100000)))
		},
		func(m *[]metav1.ManagedFieldsEntry, c randfill.Continue) {
			*m = nil
		},
		func(t *metav1.Time, c randfill.Continue) {
			*t = metav1.NewTime(metav1.Now().Truncate(time.Second))
		},
	)
}

// TestFreqUIConvertRoundTrip_AlphaToBeta covers B3's own requirement: a
// v1alpha1 FreqUI must round-trip through v1beta1 and back without losing
// anything - TradeBotRefs' bare-string -> corev1.LocalObjectReference
// conversion can't lose data (every string becomes exactly one reference's
// Name), so unlike TradeBot/TradeBotConfig's round-trip tests this one
// doesn't need to clear anything before converting.
func TestFreqUIConvertRoundTrip_AlphaToBeta(t *testing.T) {
	for i := 0; i < 200; i++ {
		original := &FreqUI{}
		freqUIFuzzer(int64(i)).Fill(original)
		original.TypeMeta = metav1.TypeMeta{} // not part of any conversion function's contract

		var converted v1beta1.FreqUI
		if err := original.ConvertTo(&converted); err != nil {
			t.Fatalf("iteration %d: ConvertTo failed: %v", i, err)
		}

		roundTripped := &FreqUI{}
		if err := roundTripped.ConvertFrom(&converted); err != nil {
			t.Fatalf("iteration %d: ConvertFrom failed: %v", i, err)
		}

		if !apiequality.Semantic.DeepEqual(original, roundTripped) {
			t.Errorf("iteration %d: round trip mismatch.\noriginal:  %+v\nroundtrip: %+v", i, original, roundTripped)
		}
	}
}

// TestFreqUIConvertRoundTrip_BetaToAlpha covers the other direction.
func TestFreqUIConvertRoundTrip_BetaToAlpha(t *testing.T) {
	for i := 0; i < 200; i++ {
		original := &v1beta1.FreqUI{}
		freqUIFuzzer(int64(i)).Fill(original)
		original.TypeMeta = metav1.TypeMeta{}

		converted := &FreqUI{}
		if err := converted.ConvertFrom(original); err != nil {
			t.Fatalf("iteration %d: ConvertFrom failed: %v", i, err)
		}

		roundTripped := &v1beta1.FreqUI{}
		if err := converted.ConvertTo(roundTripped); err != nil {
			t.Fatalf("iteration %d: ConvertTo failed converting back: %v", i, err)
		}

		if !apiequality.Semantic.DeepEqual(original, roundTripped) {
			t.Errorf("iteration %d: round trip mismatch.\noriginal:  %+v\nroundtrip: %+v", i, original, roundTripped)
		}
	}
}

// TestFreqUIConvertTo_TradeBotRefsBecomeTypedReferences covers P6-5's own
// requirement, extended to FreqUI here (B3).
func TestFreqUIConvertTo_TradeBotRefsBecomeTypedReferences(t *testing.T) {
	f := &FreqUI{
		ObjectMeta: metav1.ObjectMeta{Name: "ui", Namespace: "trading"},
		Spec:       FreqUISpec{TradeBotRefs: []string{"bot-a", "bot-b"}},
	}

	var converted v1beta1.FreqUI
	if err := f.ConvertTo(&converted); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(converted.Spec.TradeBotRefs) != 2 ||
		converted.Spec.TradeBotRefs[0].Name != "bot-a" || converted.Spec.TradeBotRefs[1].Name != "bot-b" {
		t.Errorf("expected [bot-a bot-b] as typed references, got %+v", converted.Spec.TradeBotRefs)
	}
}
