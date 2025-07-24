package configbuilder

import (
	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildPairlistMethodsConfig builds the pairlists configuration from a PairlistMethods resource
func BuildPairlistMethodsConfig(pairlistMethods *freqtradev1alpha1.PairlistMethods) map[string]interface{} {
	if pairlistMethods == nil {
		return nil
	}

	// Convert the methods to a map
	methodsConfig := make([]map[string]interface{}, 0, len(pairlistMethods.Spec.Methods))
	for _, method := range pairlistMethods.Spec.Methods {
		// Convert the method to a map
		methodMap := make(map[string]interface{})
		methodMap["method"] = string(method.Method)

		// Add common configuration options if they are set
		if method.NumberAssets != nil {
			methodMap["number_assets"] = *method.NumberAssets
		}
		if method.RefreshPeriod != nil {
			methodMap["refresh_period"] = *method.RefreshPeriod
		}

		// Add method-specific configuration options
		switch method.Method {
		case freqtradev1alpha1.StaticPairList:
			if method.AllowInactive != nil {
				methodMap["allow_inactive"] = method.AllowInactive
			}

		case freqtradev1alpha1.VolumePairList:
			if method.SortKey != "" {
				methodMap["sort_key"] = method.SortKey
			}

		case freqtradev1alpha1.PercentChangePairList:
			if method.LookbackTimeframe != "" {
				methodMap["lookback_timeframe"] = method.LookbackTimeframe
			}
			if method.LookbackPeriod != nil {
				methodMap["lookback_period"] = *method.LookbackPeriod
			}
			if method.LookbackDays != nil {
				methodMap["lookback_days"] = *method.LookbackDays
			}
			if method.MinChangeRate != nil {
				methodMap["min_change_rate"] = *method.MinChangeRate
			}

		case freqtradev1alpha1.ProducerPairList:
			if method.ProducerName != "" {
				methodMap["producer_name"] = method.ProducerName
			}

		case freqtradev1alpha1.RemotePairList:
			if method.Mode != "" {
				methodMap["mode"] = method.Mode
			}
			if method.ProcessingMode != "" {
				methodMap["processing_mode"] = method.ProcessingMode
			}
			if method.PairlistURL != "" {
				methodMap["pairlist_url"] = method.PairlistURL
			}
			if method.KeepPairlistOnFailure != nil {
				methodMap["keep_pairlist_on_failure"] = method.KeepPairlistOnFailure
			}
			if method.ReadTimeout != nil {
				methodMap["read_timeout"] = *method.ReadTimeout
			}
			if method.BearerToken != "" {
				methodMap["bearer_token"] = method.BearerToken
			}
			if method.SaveToFile != "" {
				methodMap["save_to_file"] = method.SaveToFile
			}

		case freqtradev1alpha1.MarketCapPairList:
			if method.MaxRank != nil {
				methodMap["max_rank"] = *method.MaxRank
			}
			if len(method.Categories) > 0 {
				methodMap["categories"] = method.Categories
			}

		case freqtradev1alpha1.AgeFilter:
			if method.MinDaysListed != nil {
				methodMap["min_days_listed"] = *method.MinDaysListed
			}
			if method.MaxDaysListed != nil {
				methodMap["max_days_listed"] = *method.MaxDaysListed
			}

		case freqtradev1alpha1.PriceFilter:
			if method.MinPrice != nil {
				methodMap["min_price"] = *method.MinPrice
			}
			if method.MaxPrice != nil {
				methodMap["max_price"] = *method.MaxPrice
			}

		case freqtradev1alpha1.SpreadFilter:
			if method.MaxSpreadRatio != nil {
				methodMap["max_spread_ratio"] = *method.MaxSpreadRatio
			}

		case freqtradev1alpha1.RangeStabilityFilter:
			if method.LookbackDaysRange != nil {
				methodMap["lookback_days"] = *method.LookbackDaysRange
			}
			if method.MinRateOfChange != nil {
				methodMap["min_rate_of_change"] = *method.MinRateOfChange
			}
			if method.MaxRateOfChange != nil {
				methodMap["max_rate_of_change"] = *method.MaxRateOfChange
			}

		case freqtradev1alpha1.VolatilityFilter:
			if method.LookbackDaysVolatility != nil {
				methodMap["lookback_days"] = *method.LookbackDaysVolatility
			}
			if method.MinVolatility != nil {
				methodMap["min_volatility"] = *method.MinVolatility
			}
			if method.MaxVolatility != nil {
				methodMap["max_volatility"] = *method.MaxVolatility
			}

		case freqtradev1alpha1.OffsetFilter:
			if method.Offset != nil {
				methodMap["offset"] = *method.Offset
			}
		}

		methodsConfig = append(methodsConfig, methodMap)
	}

	return map[string]interface{}{
		"pairlists": methodsConfig,
	}
}
