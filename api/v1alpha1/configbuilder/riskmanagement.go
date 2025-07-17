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

	// Set minimal ROI
	if len(riskManagement.Spec.MinimalROI) > 0 {
		roi := make(map[string]interface{})
		for timeStr, value := range riskManagement.Spec.MinimalROI {
			roi[timeStr] = value
		}
		cfg["minimal_roi"] = roi
	}

	// Set stoploss
	cfg["stoploss"] = riskManagement.Spec.Stoploss

	// Set trailing stop settings
	if riskManagement.Spec.TrailingStop {
		cfg["trailing_stop"] = true
		cfg["trailing_stop_positive"] = riskManagement.Spec.TrailingStopPositive
		cfg["trailing_stop_positive_offset"] = riskManagement.Spec.TrailingStopPositiveOffset
		cfg["trailing_only_offset_is_reached"] = riskManagement.Spec.TrailingOnlyOffsetIsReached
	}

	return cfg
}
