package configbuilder

import (
	"context"
	"fmt"

	freqtradev1alpha1 "github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// BuildPairlists builds the pairlists configuration for a TradeBot
func BuildPairlists(ctx context.Context, k8sClient client.Client, tradebot *freqtradev1alpha1.TradeBot) (map[string]interface{}, error) {
	// If no pairlistsRef is specified, return nil
	if tradebot.Spec.PairlistsRef == "" {
		return nil, nil
	}

	// Get the Pairlists resource
	var pairlists freqtradev1alpha1.Pairlists
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: tradebot.Namespace,
		Name:      tradebot.Spec.PairlistsRef,
	}, &pairlists); err != nil {
		return nil, fmt.Errorf("failed to get pairlists %s: %v", tradebot.Spec.PairlistsRef, err)
	}

	return BuildPairlistsConfig(&pairlists), nil
}

// BuildPairlistsConfig builds the pairlists configuration from a Pairlists resource
func BuildPairlistsConfig(pairlists *freqtradev1alpha1.Pairlists) map[string]interface{} {
	if pairlists == nil {
		return nil
	}

	// Convert the pairlists to a map
	pairlistsConfig := make([]map[string]interface{}, 0, len(pairlists.Spec.Pairlists))
	for _, pairlist := range pairlists.Spec.Pairlists {
		// Convert the pairlist to a map
		pairlistMap := make(map[string]interface{})
		pairlistMap["method"] = string(pairlist.Method)

		// Add common configuration options if they are set
		if pairlist.NumberAssets != nil {
			pairlistMap["number_assets"] = *pairlist.NumberAssets
		}
		if pairlist.RefreshPeriod != nil {
			pairlistMap["refresh_period"] = *pairlist.RefreshPeriod
		}

		// Add method-specific configuration options
		switch pairlist.Method {
		case freqtradev1alpha1.StaticPairList:
			if pairlist.AllowInactive != nil {
				pairlistMap["allow_inactive"] = *pairlist.AllowInactive
			}

		case freqtradev1alpha1.VolumePairList:
			if pairlist.SortKey != "" {
				pairlistMap["sort_key"] = pairlist.SortKey
			}

		case freqtradev1alpha1.PercentChangePairList:
			if pairlist.LookbackTimeframe != "" {
				pairlistMap["lookback_timeframe"] = pairlist.LookbackTimeframe
			}
			if pairlist.LookbackPeriod != nil {
				pairlistMap["lookback_period"] = *pairlist.LookbackPeriod
			}
			if pairlist.LookbackDays != nil {
				pairlistMap["lookback_days"] = *pairlist.LookbackDays
			}
			if pairlist.MinChangeRate != nil {
				pairlistMap["min_change_rate"] = *pairlist.MinChangeRate
			}

		case freqtradev1alpha1.ProducerPairList:
			if pairlist.ProducerName != "" {
				pairlistMap["producer_name"] = pairlist.ProducerName
			}

		case freqtradev1alpha1.RemotePairList:
			if pairlist.Mode != "" {
				pairlistMap["mode"] = pairlist.Mode
			}
			if pairlist.ProcessingMode != "" {
				pairlistMap["processing_mode"] = pairlist.ProcessingMode
			}
			if pairlist.PairlistURL != "" {
				pairlistMap["pairlist_url"] = pairlist.PairlistURL
			}
			if pairlist.KeepPairlistOnFailure != nil {
				pairlistMap["keep_pairlist_on_failure"] = *pairlist.KeepPairlistOnFailure
			}
			if pairlist.ReadTimeout != nil {
				pairlistMap["read_timeout"] = *pairlist.ReadTimeout
			}
			if pairlist.BearerToken != "" {
				pairlistMap["bearer_token"] = pairlist.BearerToken
			}
			if pairlist.SaveToFile != "" {
				pairlistMap["save_to_file"] = pairlist.SaveToFile
			}

		case freqtradev1alpha1.MarketCapPairList:
			if pairlist.MaxRank != nil {
				pairlistMap["max_rank"] = *pairlist.MaxRank
			}
			if len(pairlist.Categories) > 0 {
				pairlistMap["categories"] = pairlist.Categories
			}

		case freqtradev1alpha1.AgeFilter:
			if pairlist.MinDaysListed != nil {
				pairlistMap["min_days_listed"] = *pairlist.MinDaysListed
			}
			if pairlist.MaxDaysListed != nil {
				pairlistMap["max_days_listed"] = *pairlist.MaxDaysListed
			}

		case freqtradev1alpha1.PriceFilter:
			if pairlist.MinPrice != nil {
				pairlistMap["min_price"] = *pairlist.MinPrice
			}
			if pairlist.MaxPrice != nil {
				pairlistMap["max_price"] = *pairlist.MaxPrice
			}

		case freqtradev1alpha1.SpreadFilter:
			if pairlist.MaxSpreadRatio != nil {
				pairlistMap["max_spread_ratio"] = *pairlist.MaxSpreadRatio
			}

		case freqtradev1alpha1.RangeStabilityFilter:
			if pairlist.LookbackDaysRange != nil {
				pairlistMap["lookback_days"] = *pairlist.LookbackDaysRange
			}
			if pairlist.MinRateOfChange != nil {
				pairlistMap["min_rate_of_change"] = *pairlist.MinRateOfChange
			}
			if pairlist.MaxRateOfChange != nil {
				pairlistMap["max_rate_of_change"] = *pairlist.MaxRateOfChange
			}

		case freqtradev1alpha1.VolatilityFilter:
			if pairlist.LookbackDaysVolatility != nil {
				pairlistMap["lookback_days"] = *pairlist.LookbackDaysVolatility
			}
			if pairlist.MinVolatility != nil {
				pairlistMap["min_volatility"] = *pairlist.MinVolatility
			}
			if pairlist.MaxVolatility != nil {
				pairlistMap["max_volatility"] = *pairlist.MaxVolatility
			}

		case freqtradev1alpha1.OffsetFilter:
			if pairlist.Offset != nil {
				pairlistMap["offset"] = *pairlist.Offset
			}
		}

		pairlistsConfig = append(pairlistsConfig, pairlistMap)
	}

	return map[string]interface{}{
		"pairlists": pairlistsConfig,
	}
}
