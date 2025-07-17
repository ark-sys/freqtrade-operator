package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildEntryPricingConfig builds the "entry_pricing" section for Freqtrade config.json
func BuildEntryPricingConfig(entryPricing *v1alpha1.EntryPricing) map[string]interface{} {
	if entryPricing == nil {
		return nil
	}

	cfg := map[string]interface{}{}

	if entryPricing.Spec.PriceSide != "" {
		cfg["price_side"] = entryPricing.Spec.PriceSide
	}

	if entryPricing.Spec.UseOrderBook {
		cfg["use_order_book"] = true
		cfg["order_book_top"] = entryPricing.Spec.OrderBookTop
	}

	if entryPricing.Spec.PriceLastBalance {
		cfg["price_last_balance"] = entryPricing.Spec.PriceLastBalance
	}

	// Check depth of market settings
	if entryPricing.Spec.CheckDepthOfMarket.Enabled {
		cfg["check_depth_of_market"] = map[string]interface{}{
			"enabled":           true,
			"bids_to_ask_delta": entryPricing.Spec.CheckDepthOfMarket.BidsToAskDelta,
		}
	}

	return cfg
}
