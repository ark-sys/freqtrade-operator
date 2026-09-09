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

// tradeBotFuzzer fills a TradeBot with random-but-valid data for the
// round-trip tests below. Custom Funcs for resource.Quantity/metav1.Time:
// raw field-by-field filling would violate their internal invariants
// (Quantity keeps a cached string representation that must stay in sync
// with its parsed value) - MustParse/a fixed valid time keep every fuzzed
// object actually marshalable, which convertJSON (the mechanism under
// test for the App/Introspection/Bot sub-structs) depends on.
func tradeBotFuzzer(seed int64) *randfill.Filler {
	// NumElements' minimum is 1, not 0: a non-nil-but-empty slice/map and a
	// nil one are indistinguishable after any JSON round trip (both encode
	// as "absent" under omitempty, and both decode back to nil) -
	// generating the former here would make convertJSON look lossy for a
	// reason that has nothing to do with the conversion code, the same
	// trap the metav1.Time/FieldsV1 Funcs below avoid for other types.
	// NilChance still produces plenty of actually-nil fields on its own.
	return randfill.NewWithSeed(seed).NilChance(0.5).NumElements(1, 3).Funcs(
		func(q *resource.Quantity, c randfill.Continue) {
			*q = resource.MustParse(fmt.Sprintf("%dm", c.Intn(100000)))
		},
		// ManagedFields is server-side-apply's own generated content -
		// never something a real client (or this conversion code)
		// constructs, so there's nothing worth fuzzing here. Cleared at
		// the slice level, not by fuzzing FieldsV1.Raw to something
		// JSON-safe: a Funcs override for *FieldsV1 only gets to mutate an
		// already-non-nil pointer's contents, and a non-nil pointer to an
		// empty FieldsV1 isn't the same value JSON round-trips a nil-Raw
		// one to (MarshalJSON emits literal `null`, which unmarshals back
		// to a nil *FieldsV1, not a non-nil pointer to a zero-value one) -
		// avoiding ManagedFields entirely sidesteps that asymmetry rather
		// than working around it field by field.
		func(m *[]metav1.ManagedFieldsEntry, c randfill.Continue) {
			*m = nil
		},
		func(t *metav1.Time, c randfill.Continue) {
			// Truncated to the second: metav1.Time.MarshalJSON always
			// formats via RFC3339 with no fractional seconds, so a real
			// stored object never carries sub-second precision in the
			// first place - fuzzing full nanosecond precision here would
			// make convertJSON's JSON round trip look lossy for a reason
			// that has nothing to do with the conversion code itself.
			*t = metav1.NewTime(metav1.Now().Truncate(time.Second))
		},
	)
}

// TestTradeBotConvertRoundTrip_AlphaToBeta covers P6-5's own requirement:
// a trade-mode v1alpha1 TradeBot must round-trip through v1beta1 and back
// without losing anything v1beta1 can actually represent. Two fields are
// cleared before comparing, for different reasons: TypeMeta is never part
// of a ConvertTo/ConvertFrom contract at all (the API machinery stamps
// GVKs outside it - fuzzing it and comparing it here would just be
// testing this test, not the conversion code), while
// FreqtradeCommand/FreqtradeArguments/Data are the real, deliberate
// exception - v1beta1 has no field for them at all (P6-4), so a round
// trip through the storage version genuinely can't preserve them.
func TestTradeBotConvertRoundTrip_AlphaToBeta(t *testing.T) {
	for i := 0; i < 200; i++ {
		original := &TradeBot{}
		tradeBotFuzzer(int64(i)).Fill(original)
		original.TypeMeta = metav1.TypeMeta{} // not part of any conversion function's contract - see this test's own doc comment

		// Only the shape v1beta1 can represent at all - see this test's own
		// doc comment.
		if i%2 == 0 {
			original.Spec.FreqtradeCommand = ""
		} else {
			original.Spec.FreqtradeCommand = "trade"
		}
		original.Spec.FreqtradeArguments = nil
		original.Spec.Data = nil

		var converted v1beta1.TradeBot
		if err := original.ConvertTo(&converted); err != nil {
			t.Fatalf("iteration %d: ConvertTo failed for a trade-mode TradeBot: %v", i, err)
		}

		roundTripped := &TradeBot{}
		if err := roundTripped.ConvertFrom(&converted); err != nil {
			t.Fatalf("iteration %d: ConvertFrom failed: %v", i, err)
		}

		want := original.DeepCopy()
		want.Spec.FreqtradeCommand = "" // "" and "trade" both convert to "" on the way back - see ConvertFrom
		if !apiequality.Semantic.DeepEqual(want, roundTripped) {
			t.Errorf("iteration %d: round trip mismatch.\noriginal:  %+v\nroundtrip: %+v", i, want, roundTripped)
		}
	}
}

// TestTradeBotConvertRoundTrip_BetaToAlpha covers the other direction:
// starting from v1beta1 (the shape a v1beta1-native client would actually
// send) must round-trip cleanly too, since ConvertFrom never rejects
// anything (a v1beta1 TradeBot is always representable in v1alpha1 - trade
// mode was always v1alpha1's own default).
func TestTradeBotConvertRoundTrip_BetaToAlpha(t *testing.T) {
	for i := 0; i < 200; i++ {
		original := &v1beta1.TradeBot{}
		tradeBotFuzzer(int64(i)).Fill(original)
		original.TypeMeta = metav1.TypeMeta{} // not part of any conversion function's contract - see this test's own doc comment

		converted := &TradeBot{}
		if err := converted.ConvertFrom(original); err != nil {
			t.Fatalf("iteration %d: ConvertFrom failed: %v", i, err)
		}

		roundTripped := &v1beta1.TradeBot{}
		if err := converted.ConvertTo(roundTripped); err != nil {
			t.Fatalf("iteration %d: ConvertTo failed converting back: %v", i, err)
		}

		if !apiequality.Semantic.DeepEqual(original, roundTripped) {
			t.Errorf("iteration %d: round trip mismatch.\noriginal:  %+v\nroundtrip: %+v", i, original, roundTripped)
		}
	}
}

// TestTradeBotConvertTo_RejectsJobMode covers the one deliberately lossy
// direction: a Job-mode TradeBot (backtesting/hyperopt/...) has no
// v1beta1 equivalent (P6-4) and must fail loudly, not silently drop
// spec.freqtrade_command and convert as if it were a live bot.
func TestTradeBotConvertTo_RejectsJobMode(t *testing.T) {
	for _, cmd := range []string{"backtesting", "hyperopt", "download-data", "lookahead-analysis"} {
		t.Run(cmd, func(t *testing.T) {
			tradeBot := &TradeBot{Spec: TradeBotSpec{FreqtradeCommand: cmd, Config: "cfg", Strategy: "strategy-name"}}
			var converted v1beta1.TradeBot
			err := tradeBot.ConvertTo(&converted)
			if err == nil {
				t.Fatalf("expected ConvertTo to reject freqtrade_command=%q, got nil error", cmd)
			}
		})
	}
}
