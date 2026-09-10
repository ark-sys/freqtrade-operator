package configbuilder

import (
	"testing"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// TestBuildPairlistMethodsConfig covers every PairlistMethod branch (the
// golden-file fixture only exercises three of the fifteen).
func TestBuildPairlistMethodsConfig(t *testing.T) {
	spec := &v1alpha1.PairlistMethodsSpec{
		Methods: []v1alpha1.PairlistConfig{
			{Method: v1alpha1.StaticPairList, AllowInactive: true},
			{Method: v1alpha1.VolumePairList, SortKey: "quoteVolume"},
			{
				Method:            v1alpha1.PercentChangePairList,
				LookbackTimeframe: "1h",
				LookbackPeriod:    ptrInt(24),
				LookbackDays:      ptrInt(1),
				MinChangeRate:     ptrFloat64(0.01),
			},
			{Method: v1alpha1.ProducerPairList, ProducerName: "upstream"},
			{
				Method:                v1alpha1.RemotePairList,
				Mode:                  "static",
				ProcessingMode:        "filter",
				PairlistURL:           "https://example.com/pairlist.json",
				KeepPairlistOnFailure: ptrBool(true),
				ReadTimeout:           ptrInt(10),
				BearerToken:           "token",
				SaveToFile:            "/tmp/pairlist.json",
			},
			{Method: v1alpha1.MarketCapPairList, MaxRank: ptrInt(100), Categories: []string{"layer-1"}},
			{Method: v1alpha1.AgeFilter, MinDaysListed: ptrInt(10), MaxDaysListed: ptrInt(365)},
			{Method: v1alpha1.PriceFilter, MinPrice: ptrFloat64(0.001), MaxPrice: ptrFloat64(1000)},
			{Method: v1alpha1.SpreadFilter, MaxSpreadRatio: ptrFloat64(0.005)},
			{
				Method:            v1alpha1.RangeStabilityFilter,
				LookbackDaysRange: ptrInt(3),
				MinRateOfChange:   ptrFloat64(0.01),
				MaxRateOfChange:   ptrFloat64(0.5),
			},
			{
				Method:                 v1alpha1.VolatilityFilter,
				LookbackDaysVolatility: ptrInt(3),
				MinVolatility:          ptrFloat64(0.01),
				MaxVolatility:          ptrFloat64(0.5),
			},
			{Method: v1alpha1.OffsetFilter, Offset: ptrInt(5)},
			{Method: v1alpha1.ShuffleFilter, ShuffleFrequency: "iteration", Seed: ptrInt(42)},
			{Method: v1alpha1.PrecisionFilter},
			{Method: v1alpha1.PerformanceFilter, Minutes: ptrInt(60), MinProfit: ptrFloat64(0.01)},
			{Method: v1alpha1.FullTradesFilter},
		},
	}

	got := BuildPairlistMethodsConfig(spec)
	pairlists, ok := got["pairlists"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected a pairlists slice, got %T: %v", got["pairlists"], got["pairlists"])
	}
	if len(pairlists) != len(spec.Methods) {
		t.Fatalf("expected %d rendered methods, got %d", len(spec.Methods), len(pairlists))
	}

	remote := pairlists[4]
	if remote["method"] != string(v1alpha1.RemotePairList) {
		t.Fatalf("expected pairlists[4] to be RemotePairList, got %v", remote["method"])
	}
	if remote["pairlist_url"] != "https://example.com/pairlist.json" {
		t.Errorf("expected pairlist_url to round-trip, got %v", remote["pairlist_url"])
	}
	if remote["keep_pairlist_on_failure"] != true {
		t.Errorf("expected keep_pairlist_on_failure true, got %v", remote["keep_pairlist_on_failure"])
	}

	// D1: ShuffleFilter/PerformanceFilter's options were previously
	// dropped by a bare `// TODO` case; PrecisionFilter/FullTradesFilter
	// (verified against freqtrade's own source) take none at all, so
	// their only expectation is that they still render a method entry.
	shuffle := pairlists[12]
	if shuffle["method"] != string(v1alpha1.ShuffleFilter) {
		t.Fatalf("expected pairlists[12] to be ShuffleFilter, got %v", shuffle["method"])
	}
	if shuffle["shuffle_frequency"] != "iteration" {
		t.Errorf("expected shuffle_frequency to round-trip, got %v", shuffle["shuffle_frequency"])
	}
	if shuffle["seed"] != 42 {
		t.Errorf("expected seed to round-trip, got %v", shuffle["seed"])
	}

	precision := pairlists[13]
	if precision["method"] != string(v1alpha1.PrecisionFilter) {
		t.Fatalf("expected pairlists[13] to be PrecisionFilter, got %v", precision["method"])
	}

	performance := pairlists[14]
	if performance["method"] != string(v1alpha1.PerformanceFilter) {
		t.Fatalf("expected pairlists[14] to be PerformanceFilter, got %v", performance["method"])
	}
	if performance["minutes"] != 60 {
		t.Errorf("expected minutes to round-trip, got %v", performance["minutes"])
	}
	if performance["min_profit"] != 0.01 {
		t.Errorf("expected min_profit to round-trip, got %v", performance["min_profit"])
	}

	fullTrades := pairlists[15]
	if fullTrades["method"] != string(v1alpha1.FullTradesFilter) {
		t.Fatalf("expected pairlists[15] to be FullTradesFilter, got %v", fullTrades["method"])
	}
}
