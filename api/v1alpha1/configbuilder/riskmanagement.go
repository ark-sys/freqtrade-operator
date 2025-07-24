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

	if riskManagement.Spec.MinimalROI != nil {
		cfg["minimal_roi"] = *riskManagement.Spec.MinimalROI
	}

	if riskManagement.Spec.Stoploss != nil {
		cfg["stoploss"] = *riskManagement.Spec.Stoploss
	}

	if riskManagement.Spec.TrailingStop {
		cfg["trailing_stop"] = true
	}

	if riskManagement.Spec.TrailingStopPositive != nil {
		cfg["trailing_stop_positive"] = *riskManagement.Spec.TrailingStopPositive
	}

	if riskManagement.Spec.TrailingStopPositiveOffset != nil {
		cfg["trailing_stop_positive_offset"] = *riskManagement.Spec.TrailingStopPositiveOffset
	}

	if riskManagement.Spec.TrailingOnlyOffsetIsReached {
		cfg["trailing_only_offset_is_reached"] = riskManagement.Spec.TrailingOnlyOffsetIsReached
	}

	if riskManagement.Spec.UseExitSignal {
		cfg["use_exit_signal"] = true
	}

	if riskManagement.Spec.ExitProfitOnly {
		cfg["exit_profit_only"] = true
	}

	if riskManagement.Spec.ExitProfitOffset != nil {
		cfg["exit_profit_offset"] = riskManagement.Spec.ExitProfitOffset
	}

	if riskManagement.Spec.Fee != nil {
		cfg["fee"] = *riskManagement.Spec.Fee
	}

	if riskManagement.Spec.IgnoreRoiIfEntrySignal {
		cfg["ignore_roi_if_entry_signal"] = true
	}

	if riskManagement.Spec.IgnoreBuyingExpiredCandleAfter != nil {
		cfg["ignore_buying_expired_candle_after"] = *riskManagement.Spec.IgnoreBuyingExpiredCandleAfter
	}

	if riskManagement.Spec.MinimumTradeAmount != nil {
		cfg["minimum_trade_amount"] = *riskManagement.Spec.MinimumTradeAmount
	}

	if riskManagement.Spec.TargetedTradeAmount != nil {
		cfg["targeted_trade_amount"] = *riskManagement.Spec.TargetedTradeAmount
	}

	if riskManagement.Spec.LookaheadAnalysisExportFilename != "" {
		cfg["lookahead_analysis_export_filename"] = riskManagement.Spec.LookaheadAnalysisExportFilename
	}

	if riskManagement.Spec.StartupCandle != nil {
		cfg["startup_candle"] = riskManagement.Spec.StartupCandle
	}

	if riskManagement.Spec.LiquidationBuffer != nil {
		cfg["liquidation_buffer"] = *riskManagement.Spec.LiquidationBuffer
	}

	if riskManagement.Spec.BacktestBreakdown != nil {
		cfg["backtest_breakdown"] = riskManagement.Spec.BacktestBreakdown
	}

	return cfg
}
