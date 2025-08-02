package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildPricingConfig builds the "entry_pricing" section for Freqtrade config.json
func BuildPricingConfig(pricing *v1alpha1.PricingSpec) map[string]interface{} {

	if pricing == nil {
		return nil
	}

	cfg := map[string]interface{}{}

	if pricing.PriceSide != "" {
		cfg["price_side"] = pricing.PriceSide
	}

	if pricing.UseOrderBook != nil {
		cfg["use_order_book"] = true
		if pricing.OrderBookTop != nil {
			cfg["order_book_top"] = *pricing.OrderBookTop
		}
	}

	if pricing.PriceLastBalance != nil {
		cfg["price_last_balance"] = *pricing.PriceLastBalance
	}

	if pricing.CheckDepthOfMarket != nil {

		cfg["check_depth_of_market"] = map[string]interface{}{}
		if pricing.CheckDepthOfMarket.Enabled != nil {
			cfg["check_depth_of_market"].(map[string]interface{})["enabled"] = *pricing.CheckDepthOfMarket.Enabled
		}
		if pricing.CheckDepthOfMarket.BidsToAskDelta != nil {
			cfg["check_depth_of_market"].(map[string]interface{})["bids_to_ask_delta"] = *pricing.CheckDepthOfMarket.BidsToAskDelta
		}

	}

	return cfg
}
