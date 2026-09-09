package collectresults

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
	return path
}

func TestParseResultFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	resultJSON := `{
		"strategy": {
			"SampleStrategy": {
				"total_trades": 42,
				"profit_total_abs": 123.45,
				"profit_total_pct": 12.3,
				"winrate": 0.55,
				"max_drawdown_account": 0.08,
				"sharpe": 1.5,
				"sortino": 2.1,
				"cagr": 0.25,
				"best_pair": {"key": "BTC/USDT"},
				"worst_pair": {"key": "ETH/USDT"}
			}
		}
	}`
	path := writeTempFile(t, dir, "backtest-result-1.json", resultJSON)

	results, err := parseResultFile(path, "SampleStrategy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results.TotalTrades != 42 {
		t.Errorf("expected TotalTrades 42, got %d", results.TotalTrades)
	}
	if results.ProfitAbs != "123.45" {
		t.Errorf("expected ProfitAbs 123.45, got %q", results.ProfitAbs)
	}
	if results.WinRatePct != "55" {
		t.Errorf("expected WinRatePct 55 (ratio * 100), got %q", results.WinRatePct)
	}
	if results.BestPair != "BTC/USDT" {
		t.Errorf("expected BestPair BTC/USDT, got %q", results.BestPair)
	}
	if results.WorstPair != "ETH/USDT" {
		t.Errorf("expected WorstPair ETH/USDT, got %q", results.WorstPair)
	}
	if results.ResultFile != path {
		t.Errorf("expected ResultFile %q, got %q", path, results.ResultFile)
	}
}

func TestParseResultFile_MissingFileIsAnError(t *testing.T) {
	_, err := parseResultFile(filepath.Join(t.TempDir(), "does-not-exist.json"), "SampleStrategy")
	if err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}

func TestParseResultFile_MalformedJSONIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "backtest-result-1.json", `{"strategy": not valid json`)

	_, err := parseResultFile(path, "SampleStrategy")
	if err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

func TestParseResultFile_UnknownStrategyIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "backtest-result-1.json", `{"strategy": {"OtherStrategy": {}}}`)

	_, err := parseResultFile(path, "SampleStrategy")
	if err == nil {
		t.Fatal("expected an error when the requested strategy isn't in the file, got nil")
	}
}

// TestParseResultFile_MissingFieldsDegradeGracefully covers P6-2's own
// design intent: this package's field-name mapping is a best-effort guess
// (unverified against a real freqtrade result file - see the commit that
// introduced it), so a field that doesn't match must not fail the whole
// extraction - only a fully unreadable/unparseable file or an unknown
// strategy key does.
func TestParseResultFile_MissingFieldsDegradeGracefully(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "backtest-result-1.json",
		`{"strategy": {"SampleStrategy": {"total_trades": 7, "some_unrecognized_field": "whatever"}}}`)

	results, err := parseResultFile(path, "SampleStrategy")
	if err != nil {
		t.Fatalf("expected missing/unrecognized fields to degrade gracefully, got error: %v", err)
	}
	if results.TotalTrades != 7 {
		t.Errorf("expected the one recognized field to still be extracted, got TotalTrades=%d", results.TotalTrades)
	}
	if results.ProfitAbs != "0" {
		t.Errorf("expected ProfitAbs to be the zero value for an absent field, got %q", results.ProfitAbs)
	}
}

func TestLatestResultFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, ".last_result.json", `{"latest_backtest": "backtest-result-2024.json"}`)

	got, err := latestResultFile(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(dir, "backtest-result-2024.json")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestLatestResultFile_MissingPointerIsAnError(t *testing.T) {
	_, err := latestResultFile(t.TempDir())
	if err == nil {
		t.Fatal("expected an error for a missing .last_result.json, got nil")
	}
}

func TestLatestResultFile_EmptyPointerIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, ".last_result.json", `{}`)

	_, err := latestResultFile(dir)
	if err == nil {
		t.Fatal("expected an error for a .last_result.json with no latest_backtest, got nil")
	}
}

// sanity-check json.Marshal round-trips through the same path Run() uses,
// so the ConfigMap the operator later reads is exactly what a real
// extraction would produce.
func TestExtractedResultsMarshalCleanly(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, ".last_result.json", `{"latest_backtest": "r.json"}`)
	writeTempFile(t, dir, "r.json", `{"strategy": {"S": {"total_trades": 1}}}`)

	path, err := latestResultFile(dir)
	if err != nil {
		t.Fatalf("latestResultFile: %v", err)
	}
	results, err := parseResultFile(path, "S")
	if err != nil {
		t.Fatalf("parseResultFile: %v", err)
	}
	if _, err := json.Marshal(results); err != nil {
		t.Errorf("expected extracted results to marshal cleanly, got: %v", err)
	}
}
