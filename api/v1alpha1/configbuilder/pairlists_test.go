package configbuilder

import (
	"testing"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildPairlistsConfig(t *testing.T) {
	// Helper function to create int pointer
	intPtr := func(i int) *int {
		return &i
	}

	// Helper function to create int64 pointer
	int64Ptr := func(i int64) *int64 {
		return &i
	}

	// Helper function to create float64 pointer
	float64Ptr := func(f float64) *float64 {
		return &f
	}

	// Helper function to create bool pointer
	boolPtr := func(b bool) *bool {
		return &b
	}

	tests := []struct {
		name      string
		pairlists *freqtradev1alpha1.Pairlists
		expected  map[string]interface{}
	}{
		{
			name: "StaticPairList",
			pairlists: &freqtradev1alpha1.Pairlists{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pairlists",
					Namespace: "default",
				},
				Spec: freqtradev1alpha1.PairlistsSpec{
					Pairlists: []freqtradev1alpha1.PairlistConfig{
						{
							Method:        freqtradev1alpha1.StaticPairList,
							AllowInactive: boolPtr(false),
						},
					},
				},
			},
			expected: map[string]interface{}{
				"pairlists": []map[string]interface{}{
					{
						"method":         "StaticPairList",
						"allow_inactive": false,
					},
				},
			},
		},
		{
			name: "VolumePairList",
			pairlists: &freqtradev1alpha1.Pairlists{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pairlists",
					Namespace: "default",
				},
				Spec: freqtradev1alpha1.PairlistsSpec{
					Pairlists: []freqtradev1alpha1.PairlistConfig{
						{
							Method:        freqtradev1alpha1.VolumePairList,
							NumberAssets:  intPtr(20),
							RefreshPeriod: int64Ptr(1800),
							SortKey:       "quoteVolume",
						},
					},
				},
			},
			expected: map[string]interface{}{
				"pairlists": []map[string]interface{}{
					{
						"method":         "VolumePairList",
						"number_assets":  20,
						"refresh_period": int64(1800),
						"sort_key":       "quoteVolume",
					},
				},
			},
		},
		{
			name: "Multiple Pairlists",
			pairlists: &freqtradev1alpha1.Pairlists{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pairlists",
					Namespace: "default",
				},
				Spec: freqtradev1alpha1.PairlistsSpec{
					Pairlists: []freqtradev1alpha1.PairlistConfig{
						{
							Method:        freqtradev1alpha1.StaticPairList,
							AllowInactive: boolPtr(false),
						},
						{
							Method:        freqtradev1alpha1.VolumePairList,
							NumberAssets:  intPtr(20),
							RefreshPeriod: int64Ptr(1800),
							SortKey:       "quoteVolume",
						},
						{
							Method:        freqtradev1alpha1.AgeFilter,
							MinDaysListed: intPtr(10),
							MaxDaysListed: intPtr(100),
						},
						{
							Method:         freqtradev1alpha1.SpreadFilter,
							MaxSpreadRatio: float64Ptr(0.005),
						},
					},
				},
			},
			expected: map[string]interface{}{
				"pairlists": []map[string]interface{}{
					{
						"method":         "StaticPairList",
						"allow_inactive": false,
					},
					{
						"method":         "VolumePairList",
						"number_assets":  20,
						"refresh_period": int64(1800),
						"sort_key":       "quoteVolume",
					},
					{
						"method":          "AgeFilter",
						"min_days_listed": 10,
						"max_days_listed": 100,
					},
					{
						"method":           "SpreadFilter",
						"max_spread_ratio": 0.005,
					},
				},
			},
		},
		{
			name:      "Nil Pairlists",
			pairlists: nil,
			expected:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildPairlistsConfig(tt.pairlists)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			assert.NotNil(t, result)

			expectedPairlists, ok := tt.expected["pairlists"].([]map[string]interface{})
			assert.True(t, ok)

			resultPairlists, ok := result["pairlists"].([]map[string]interface{})
			assert.True(t, ok)

			assert.Equal(t, len(expectedPairlists), len(resultPairlists))

			for i, expectedPairlist := range expectedPairlists {
				resultPairlist := resultPairlists[i]
				for key, expectedValue := range expectedPairlist {
					assert.Equal(t, expectedValue, resultPairlist[key])
				}
			}
		})
	}
}
