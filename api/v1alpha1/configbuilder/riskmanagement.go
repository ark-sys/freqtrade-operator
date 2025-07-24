package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildRiskManagementConfig builds risk management related sections for Freqtrade config.json
func BuildRiskManagementConfig(riskManagement *v1alpha1.RiskManagement) map[string]interface{} {
	if riskManagement == nil {
		return nil
	}

	cfg := map[string]interface{}{}

	if riskManagement.Spec.MinimalROI >= 0.0 {

		cfg["minimal_roi"] = riskManagement.Spec.MinimalROI
	}

	if riskManagement.Spec.Stoploss >= 0.0 {
		cfg["stoploss"] = riskManagement.Spec.Stoploss
	}

	// TODO: provide missing params

	// Set trailing stop settings
	if riskManagement.Spec.TrailingStop {
		cfg["trailing_stop"] = true
		if riskManagement.Spec.TrailingStopPositive != nil {

			cfg["trailing_stop_positive"] = riskManagement.Spec.TrailingStopPositive
		}
		if riskManagement.Spec.TrailingStopPositiveOffset != nil {

			cfg["trailing_stop_positive_offset"] = riskManagement.Spec.TrailingStopPositiveOffset
		}
		if riskManagement.Spec.TrailingOnlyOffsetIsReached != nil {

			cfg["trailing_only_offset_is_reached"] = riskManagement.Spec.TrailingOnlyOffsetIsReached
		}
	}

	return cfg
}
