package collectresults

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	freqtradev1beta1 "github.com/ark-sys/freqtrade-operator/api/v1beta1"
)

// freqtradeResultFile is freqtrade's own backtest-result-<ts>.json shape,
// as best understood without a real sample to verify field names/nesting
// against (P6-2 - flagged as such in the commit that introduced this).
// Only the subset BacktestResults needs is modeled; the real file carries
// far more (per-trade records, per-pair breakdowns, config echo, ...),
// all deliberately ignored here - the raw file itself stays on the
// results PVC for anyone who needs more than this summary.
//
// A field this doesn't recognize, or that freqtrade's actual JSON nests
// differently in whatever version produced it, just leaves the
// corresponding BacktestResults field at its zero value - see
// parseResultFile's own doc comment for why that's a deliberate choice,
// not a bug.
type freqtradeResultFile struct {
	Strategy map[string]freqtradeStrategyStats `json:"strategy"`
}

type freqtradeStrategyStats struct {
	TotalTrades    int     `json:"total_trades"`
	ProfitTotalAbs float64 `json:"profit_total_abs"`
	// ProfitTotal is a ratio (e.g. -0.00083, not -0.08) - there is no sibling
	// "profit_total_pct" field at this (strategy) level, unlike best_pair/
	// worst_pair below, which carry both. Verified directly against a real
	// run's own result file: parseResultFile's formatPercent (not
	// formatFloat) is what turns this into the percentage BacktestResults
	// reports.
	ProfitTotal        float64            `json:"profit_total"`
	Winrate            float64            `json:"winrate"`
	MaxDrawdownAccount float64            `json:"max_drawdown_account"`
	Sharpe             float64            `json:"sharpe"`
	Sortino            float64            `json:"sortino"`
	CAGR               float64            `json:"cagr"`
	BestPair           *freqtradePairStat `json:"best_pair"`
	WorstPair          *freqtradePairStat `json:"worst_pair"`
}

type freqtradePairStat struct {
	Key string `json:"key"`
}

// readResultBytes returns the raw bytes of freqtrade's own
// backtest-result-<ts>.json, whether path points at that file directly or -
// confirmed directly against a real run's own output, not assumed - at the
// backtest-result-<ts>.zip archive .last_result.json actually names by
// default. The results JSON inside that zip always shares the zip's own
// basename (only the extension differs), alongside a same-prefixed
// _config.json (a config echo, not results - reading whichever .json
// happened to come first would silently pick the wrong one) and per-run
// .py/.feather files this package has no use for.
func readResultBytes(path string) ([]byte, error) {
	if !strings.HasSuffix(path, ".zip") {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		return raw, nil
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s as a zip: %w", path, err)
	}
	defer func() { _ = r.Close() }()

	entryName := strings.TrimSuffix(filepath.Base(path), ".zip") + ".json"
	for _, f := range r.File {
		if f.Name != entryName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opening %s inside %s: %w", entryName, path, err)
		}
		defer func() { _ = rc.Close() }()
		raw, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("reading %s inside %s: %w", entryName, path, err)
		}
		return raw, nil
	}
	return nil, fmt.Errorf("%s has no %s entry", path, entryName)
}

// parseResultFile reads and extracts a summary from one freqtrade result
// file. Only two things fail this outright (returning an error, which the
// caller turns into ResultsAvailable=False/ResultsUnavailable rather than
// failing the Backtest itself - see Run's doc comment): the file isn't
// readable, or its JSON doesn't parse at all. Once past that, a missing or
// renamed individual field (a real risk given this package's own
// unverified field-name guesses) just leaves that one BacktestResults
// field at its zero value instead of failing the whole extraction -
// partial results are better than none, and every field here is
// `omitempty` on BacktestResults anyway, so a zero value renders as
// "not reported" rather than a misleading "0".
func parseResultFile(path, strategyName string) (*freqtradev1beta1.BacktestResults, error) {
	raw, err := readResultBytes(path)
	if err != nil {
		return nil, err
	}

	var parsed freqtradeResultFile
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	stats, ok := parsed.Strategy[strategyName]
	if !ok {
		return nil, fmt.Errorf("%s has no results for strategy %q", path, strategyName)
	}

	results := &freqtradev1beta1.BacktestResults{
		TotalTrades:    stats.TotalTrades,
		ProfitAbs:      formatFloat(stats.ProfitTotalAbs),
		ProfitPct:      formatPercent(stats.ProfitTotal),
		WinRatePct:     formatPercent(stats.Winrate),
		MaxDrawdownPct: formatPercent(stats.MaxDrawdownAccount),
		SharpeRatio:    formatFloat(stats.Sharpe),
		SortinoRatio:   formatFloat(stats.Sortino),
		CAGRPct:        formatPercent(stats.CAGR),
		ResultFile:     path,
	}
	if stats.BestPair != nil {
		results.BestPair = stats.BestPair.Key
	}
	if stats.WorstPair != nil {
		results.WorstPair = stats.WorstPair.Key
	}
	return results, nil
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// formatPercent converts a ratio (freqtrade's own convention, e.g. 0.55)
// to a percentage and formats it. Rounded to 4 decimal places before
// formatting - not for display polish, but because ratio*100 in float64
// arithmetic routinely lands a few ULPs off a clean value (0.55*100 is
// 55.00000000000001, not 55), and formatFloat's shortest-round-trip
// formatting would otherwise faithfully reproduce that noise in status.
func formatPercent(ratio float64) string {
	const decimalPlaces = 1e4
	return formatFloat(math.Round(ratio*100*decimalPlaces) / decimalPlaces)
}
