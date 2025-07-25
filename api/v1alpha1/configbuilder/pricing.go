package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildPricingConfig builds the "entry_pricing" section for Freqtrade config.json
func BuildPricingConfig(pricing *v1alpha1.Pricing) map[string]interface{} {
	if pricing == nil {
		return nil
	}

	cfg := map[string]interface{}{}

	if pricing.Spec.PriceSide != "" {
		cfg["price_side"] = pricing.Spec.PriceSide
	}

	if pricing.Spec.UseOrderBook {
		cfg["use_order_book"] = true
		if pricing.Spec.OrderBookTop != nil {
			cfg["order_book_top"] = *pricing.Spec.OrderBookTop
		}
	}

	if pricing.Spec.PriceLastBalance {
		cfg["price_last_balance"] = pricing.Spec.PriceLastBalance
	}

	if pricing.Spec.CheckDepthOfMarket != nil {
		if pricing.Spec.CheckDepthOfMarket.Enabled {
			cfg["check_depth_of_market"] = map[string]interface{}{
				"enabled":           true,
				"bids_to_ask_delta": pricing.Spec.CheckDepthOfMarket.BidsToAskDelta,
			}
		}
	}

	return cfg
}
