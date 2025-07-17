package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildExitPricingConfig builds the "exit_pricing" section for Freqtrade config.json
func BuildExitPricingConfig(exitPricing *v1alpha1.ExitPricing) map[string]interface{} {
	if exitPricing == nil {
		return nil
	}

	cfg := map[string]interface{}{}

	if exitPricing.Spec.PriceSide != "" {
		cfg["price_side"] = exitPricing.Spec.PriceSide
	}

	if exitPricing.Spec.UseOrderBook {
		cfg["use_order_book"] = true
		cfg["order_book_top"] = exitPricing.Spec.OrderBookTop
	}

	if exitPricing.Spec.PriceLastBalance {
		cfg["price_last_balance"] = exitPricing.Spec.PriceLastBalance
	}

	// Check depth of market settings
	if exitPricing.Spec.CheckDepthOfMarket.Enabled {
		cfg["check_depth_of_market"] = map[string]interface{}{
			"enabled":           true,
			"bids_to_ask_delta": exitPricing.Spec.CheckDepthOfMarket.BidsToAskDelta,
		}
	}

	return cfg
}
