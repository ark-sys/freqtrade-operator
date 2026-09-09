package collectresults

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempZip builds a zip at dir/name whose entries are exactly {entryName: content}
// plus a same-prefixed "_config.json" decoy entry - matching the shape a real freqtrade
// backtest-result-<ts>.zip carries (results JSON, a config echo, strategy source, feather
// files), so the "pick the right entry" logic in readResultBytes has something to get wrong
// if it regresses to e.g. "the first .json entry in the archive".
func writeTempZip(t *testing.T, dir, name, entryName, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	zw := zip.NewWriter(f)
	decoyName := strings.TrimSuffix(entryName, ".json") + "_config.json"
	for _, entry := range []struct{ name, content string }{
		{decoyName, `{"this": "is a config echo, not results"}`},
		{entryName, content},
	} {
		w, err := zw.Create(entry.name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", entry.name, err)
		}
		if _, err := w.Write([]byte(entry.content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", entry.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	return path
}

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
				"profit_total": 0.123,
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
	// Regression: the real field is "profit_total" (a ratio), not "profit_total_pct" - there is
	// no such sibling field at the strategy level, only on best_pair/worst_pair. Confirmed
	// directly against a real freqtrade run's own result file.
	if results.ProfitPct != "12.3" {
		t.Errorf("expected ProfitPct 12.3 (profit_total ratio * 100), got %q", results.ProfitPct)
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

// Regression test: freqtrade's own default is to write backtest-result-<ts>.zip, not a plain
// .json file - confirmed directly against a real run's own output. parseResultFile previously
// tried to json.Unmarshal the zip's raw bytes directly ("invalid character 'P' looking for
// beginning of value" - PK, the zip magic bytes), never extracting anything at all.
func TestParseResultFile_ZipArchiveExtractsTheMatchingEntry(t *testing.T) {
	dir := t.TempDir()
	resultJSON := `{"strategy": {"SampleStrategy": {"total_trades": 9, "profit_total_abs": 1}}}`
	path := writeTempZip(t, dir, "backtest-result-1.zip", "backtest-result-1.json", resultJSON)

	results, err := parseResultFile(path, "SampleStrategy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results.TotalTrades != 9 {
		t.Errorf("expected TotalTrades 9 (read from the zip's matching entry, not its _config.json decoy), got %d",
			results.TotalTrades)
	}
}

func TestParseResultFile_ZipMissingMatchingEntryIsAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create %s: %v", path, err)
	}
	if err := zip.NewWriter(f).Close(); err != nil {
		t.Fatalf("failed to write empty zip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close %s: %v", path, err)
	}

	_, err = parseResultFile(path, "SampleStrategy")
	if err == nil {
		t.Fatal("expected an error for a zip with no matching entry, got nil")
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
