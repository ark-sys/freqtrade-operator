package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PairlistMethod represents the method used for pair selection
type PairlistMethod string

const (
	// StaticPairList uses a statically defined pair whitelist from the configuration
	StaticPairList PairlistMethod = "StaticPairList"
	// VolumePairList employs sorting/filtering of pairs by their trading volume
	VolumePairList PairlistMethod = "VolumePairList"
	// PercentChangePairList selects pairs based on percent change
	PercentChangePairList PairlistMethod = "PercentChangePairList"
	// ProducerPairList reuses the pairlist from a Producer
	ProducerPairList PairlistMethod = "ProducerPairList"
	// RemotePairList fetches a pairlist from a remote server or a locally stored json file
	RemotePairList PairlistMethod = "RemotePairList"
	// MarketCapPairList selects pairs based on market capitalization
	MarketCapPairList PairlistMethod = "MarketCapPairList"
	// AgeFilter removes pairs that have been listed on the exchange for less than min_days_listed
	AgeFilter PairlistMethod = "AgeFilter"
	// FullTradesFilter shrinks whitelist to consist only in-trade pairs when the trade slots are full
	FullTradesFilter PairlistMethod = "FullTradesFilter"
	// OffsetFilter applies an offset to the pairlist
	OffsetFilter PairlistMethod = "OffsetFilter"
	// PerformanceFilter filters pairs by their performance
	PerformanceFilter PairlistMethod = "PerformanceFilter"
	// PrecisionFilter filters pairs by their precision
	PrecisionFilter PairlistMethod = "PrecisionFilter"
	// PriceFilter filters pairs by their price
	PriceFilter PairlistMethod = "PriceFilter"
	// ShuffleFilter shuffles the pairlist
	ShuffleFilter PairlistMethod = "ShuffleFilter"
	// SpreadFilter filters pairs by their spread
	SpreadFilter PairlistMethod = "SpreadFilter"
	// RangeStabilityFilter filters pairs by their range stability
	RangeStabilityFilter PairlistMethod = "RangeStabilityFilter"
	// VolatilityFilter filters pairs by their volatility
	VolatilityFilter PairlistMethod = "VolatilityFilter"
)

// PairlistConfig represents the configuration for a pairlist method
type PairlistConfig struct {
	// Method is the pairlist method to use
	Method PairlistMethod `json:"method"`

	// Common configuration options
	NumberAssets  *int   `json:"number_assets,omitempty"`
	RefreshPeriod *int64 `json:"refresh_period,omitempty"`

	// StaticPairList specific options
	AllowInactive *bool `json:"allow_inactive,omitempty"`

	// VolumePairList specific options
	SortKey string `json:"sort_key,omitempty"` // Only "quoteVolume" is supported

	// PercentChangePairList specific options
	LookbackTimeframe string   `json:"lookback_timeframe,omitempty"`
	LookbackPeriod    *int     `json:"lookback_period,omitempty"`
	LookbackDays      *int     `json:"lookback_days,omitempty"`
	MinChangeRate     *float64 `json:"min_change_rate,omitempty"`

	// ProducerPairList specific options
	ProducerName string `json:"producer_name,omitempty"`

	// RemotePairList specific options
	Mode                  string `json:"mode,omitempty"`
	ProcessingMode        string `json:"processing_mode,omitempty"`
	PairlistURL           string `json:"pairlist_url,omitempty"`
	KeepPairlistOnFailure *bool  `json:"keep_pairlist_on_failure,omitempty"`
	ReadTimeout           *int   `json:"read_timeout,omitempty"`
	BearerToken           string `json:"bearer_token,omitempty"`
	SaveToFile            string `json:"save_to_file,omitempty"`

	// MarketCapPairList specific options
	MaxRank    *int     `json:"max_rank,omitempty"`
	Categories []string `json:"categories,omitempty"`

	// AgeFilter specific options
	MinDaysListed *int `json:"min_days_listed,omitempty"`
	MaxDaysListed *int `json:"max_days_listed,omitempty"`

	// PriceFilter specific options
	MinPrice *float64 `json:"min_price,omitempty"`
	MaxPrice *float64 `json:"max_price,omitempty"`

	// SpreadFilter specific options
	MaxSpreadRatio *float64 `json:"max_spread_ratio,omitempty"`

	// RangeStabilityFilter specific options
	LookbackDaysRange *int     `json:"lookback_days_range,omitempty"`
	MinRateOfChange   *float64 `json:"min_rate_of_change,omitempty"`
	MaxRateOfChange   *float64 `json:"max_rate_of_change,omitempty"`

	// VolatilityFilter specific options
	LookbackDaysVolatility *int     `json:"lookback_days_volatility,omitempty"`
	MinVolatility          *float64 `json:"min_volatility,omitempty"`
	MaxVolatility          *float64 `json:"max_volatility,omitempty"`

	// OffsetFilter specific options
	Offset *int `json:"offset,omitempty"`
}

// PairlistMethodsSpec defines the desired state of PairlistMethods
type PairlistMethodsSpec struct {
	// Methods is an array of pairlist configurations
	Methods []PairlistConfig `json:"methods"`
}

// PairlistMethodsStatus defines the observed state of PairlistMethods
type PairlistMethodsStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PairlistMethods is the Schema for the pairlist methods API
type PairlistMethods struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PairlistMethodsSpec   `json:"spec,omitempty"`
	Status PairlistMethodsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PairlistMethodsList contains a list of PairlistMethods
type PairlistMethodsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PairlistMethods `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PairlistMethods{}, &PairlistMethodsList{})
}
