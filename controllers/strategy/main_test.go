package strategy

import (
	"strings"
	"testing"
)

func TestIsValidPythonClassName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "empty string", input: "", want: false},
		{name: "simple CamelCase", input: "SampleStrategy", want: true},
		{name: "leading underscore", input: "_PrivateStrategy", want: true},
		{name: "leading lowercase", input: "sampleStrategy", want: true},
		{name: "digits after first char", input: "Strategy2024", want: true},
		{name: "underscores throughout", input: "My_Strategy_V2", want: true},
		{name: "leading digit is invalid", input: "2024Strategy", want: false},
		{name: "hyphen is invalid", input: "My-Strategy", want: false},
		{name: "space is invalid", input: "My Strategy", want: false},
		{name: "dot is invalid", input: "My.Strategy", want: false},
		{name: "single letter", input: "A", want: true},
		{name: "single underscore", input: "_", want: true},
		{name: "single digit is invalid", input: "1", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidPythonClassName(tt.input); got != tt.want {
				t.Errorf("isValidPythonClassName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

const validScript = `
import freqtrade
from freqtrade.strategy import IStrategy

class SampleStrategy(IStrategy):
    def populate_indicators(self, dataframe, metadata):
        return dataframe

    def populate_entry_trend(self, dataframe, metadata):
        return dataframe

    def populate_exit_trend(self, dataframe, metadata):
        return dataframe
`

func TestValidateStrategyScript(t *testing.T) {
	r := &Reconciler{}

	tests := []struct {
		name        string
		script      string
		strategy    string
		wantErr     bool
		errContains string
	}{
		{name: "valid script", script: validScript, strategy: "SampleStrategy", wantErr: false},
		{name: "empty script", script: "   ", strategy: "SampleStrategy", wantErr: true, errContains: "cannot be empty"},
		{
			name:        "missing class definition",
			script:      "class OtherStrategy(IStrategy):\n    pass",
			strategy:    "SampleStrategy",
			wantErr:     true,
			errContains: "class definition",
		},
		{
			name: "missing required method",
			script: `import freqtrade
class SampleStrategy(IStrategy):
    def populate_indicators(self, dataframe, metadata):
        return dataframe`,
			strategy:    "SampleStrategy",
			wantErr:     true,
			errContains: "populate_entry_trend",
		},
		{
			name: "missing freqtrade import",
			script: `class SampleStrategy(IStrategy):
    def populate_indicators(self, dataframe, metadata):
        return dataframe
    def populate_entry_trend(self, dataframe, metadata):
        return dataframe
    def populate_exit_trend(self, dataframe, metadata):
        return dataframe`,
			strategy:    "SampleStrategy",
			wantErr:     true,
			errContains: "freqtrade imports",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.validateStrategyScript(tt.script, tt.strategy)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateStrategyScript() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %v", tt.errContains, err)
				}
			}
		})
	}
}
