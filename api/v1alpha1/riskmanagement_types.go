package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MinimalROI maps duration (in minutes) to ROI ratio

// RiskManagementSpec defines the desired state of RiskManagement
type RiskManagementSpec struct {
	MinimalROI                  *float64 `json:"minimal_roi,omitempty"`
	Stoploss                    *float64 `json:"stoploss,omitempty"`
	TrailingStop                bool     `json:"trailing_stop,omitempty"`
	TrailingStopPositive        *float64 `json:"trailing_stop_positive,omitempty"`
	TrailingStopPositiveOffset  *float64 `json:"trailing_stop_positive_offset,omitempty"`
	TrailingOnlyOffsetIsReached bool     `json:"trailing_only_offset_is_reached,omitempty"`

	UseExitSignal                   bool     `json:"use_exit_signal,omitempty"`
	ExitProfitOnly                  bool     `json:"exit_profit_only,omitempty"`
	ExitProfitOffset                *float64 `json:"exit_profit_offset,omitempty"`
	Fee                             *float64 `json:"fee,omitempty"`
	IgnoreRoiIfEntrySignal          bool     `json:"ignore_roi_if_entry_signal,omitempty"`
	IgnoreBuyingExpiredCandleAfter  *int     `json:"ignore_buying_expired_candle_after,omitempty"`
	MinimumTradeAmount              *int     `json:"minimum_trade_amount,omitempty"`
	TargetedTradeAmount             *int     `json:"targeted_trade_amount,omitempty"`
	LookaheadAnalysisExportFilename string   `json:"lookahead_analysis_export_filename,omitempty"`
	StartupCandle                   []int    `json:"startup_candle,omitempty"`     // Array of integers representing startup candles
	LiquidationBuffer               *float64 `json:"liquidation_buffer,omitempty"` // Buffer for liquidation
	BacktestBreakdown               []string `json:"backtest_breakdown,omitempty"`
}

// RiskManagementStatus defines the observed state of RiskManagement
type RiskManagementStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type RiskManagement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RiskManagementSpec   `json:"spec,omitempty"`
	Status RiskManagementStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type RiskManagementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RiskManagement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RiskManagement{}, &RiskManagementList{})
}
