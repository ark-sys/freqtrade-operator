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
			{Method: v1alpha1.ShuffleFilter},
			{Method: v1alpha1.PrecisionFilter},
			{Method: v1alpha1.PerformanceFilter},
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
}
