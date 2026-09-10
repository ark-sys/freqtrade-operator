package configbuilder

import (
	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
)

// BuildRiskManagementConfig builds risk management related sections for Freqtrade config.json
func BuildRiskManagementConfig(riskManagement *v1alpha1.RiskManagementSpec) map[string]interface{} {
	if riskManagement == nil {
		return nil
	}

	cfg := map[string]interface{}{}

	if riskManagement.MinimalROI != nil {
		cfg["minimal_roi"] = riskManagement.MinimalROI
	}

	if riskManagement.Stoploss != nil {
		cfg["stoploss"] = *riskManagement.Stoploss
	}

	if riskManagement.TrailingStop {
		cfg["trailing_stop"] = true
	}

	if riskManagement.TrailingStopPositive != nil {
		cfg["trailing_stop_positive"] = *riskManagement.TrailingStopPositive
	}

	if riskManagement.TrailingStopPositiveOffset != nil {
		cfg["trailing_stop_positive_offset"] = *riskManagement.TrailingStopPositiveOffset
	}

	if riskManagement.TrailingOnlyOffsetIsReached {
		cfg["trailing_only_offset_is_reached"] = riskManagement.TrailingOnlyOffsetIsReached
	}

	if riskManagement.UseExitSignal {
		cfg["use_exit_signal"] = true
	}

	if riskManagement.ExitProfitOnly {
		cfg["exit_profit_only"] = true
	}

	if riskManagement.ExitProfitOffset != nil {
		cfg["exit_profit_offset"] = *riskManagement.ExitProfitOffset
	}

	if riskManagement.Fee != nil {
		cfg["fee"] = *riskManagement.Fee
	}

	if riskManagement.IgnoreRoiIfEntrySignal {
		cfg["ignore_roi_if_entry_signal"] = true
	}

	if riskManagement.IgnoreBuyingExpiredCandleAfter != nil {
		cfg["ignore_buying_expired_candle_after"] = *riskManagement.IgnoreBuyingExpiredCandleAfter
	}

	if riskManagement.MinimumTradeAmount != nil {
		cfg["minimum_trade_amount"] = *riskManagement.MinimumTradeAmount
	}

	if riskManagement.TargetedTradeAmount != nil {
		cfg["targeted_trade_amount"] = *riskManagement.TargetedTradeAmount
	}

	if riskManagement.LookaheadAnalysisExportFilename != "" {
		cfg["lookahead_analysis_exportfilename"] = riskManagement.LookaheadAnalysisExportFilename
	}

	if riskManagement.StartupCandle != nil {
		cfg["startup_candle"] = *riskManagement.StartupCandle
	}

	if riskManagement.LiquidationBuffer != nil {
		cfg["liquidation_buffer"] = *riskManagement.LiquidationBuffer
	}

	if riskManagement.BacktestBreakdown != nil {
		cfg["backtest_breakdown"] = riskManagement.BacktestBreakdown
	}

	return cfg
}
