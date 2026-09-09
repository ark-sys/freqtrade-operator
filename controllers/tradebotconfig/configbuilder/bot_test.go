package configbuilder

import "testing"

// Regression test: freqtrade's own config schema types stake_amount as
// `number` or `string`, but only accepts the literal string "unlimited" -
// any other numeric-looking string fails freqtrade's own startup validation
// with "does not match 'unlimited'" (verified directly against a real
// TradeBot pod, which crash-looped on exactly that before this fix), since a
// quoted JSON string is never a number to that schema no matter its
// characters. BotConfig.StakeAmount has to stay a Go string (the CRD field
// needs to accept "unlimited" too), so renderStakeAmount is what actually
// decides which JSON type gets written.
func TestRenderStakeAmount(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want interface{}
	}{
		{"unlimited stays a string", "unlimited", "unlimited"},
		{"integer becomes a JSON number", "100", float64(100)},
		{"decimal becomes a JSON number", "12.5", float64(12.5)},
		{"garbage passes through unchanged, for a clear freqtrade-side error", "not-a-number", "not-a-number"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderStakeAmount(tc.in)
			if got != tc.want {
				t.Errorf("renderStakeAmount(%q) = %#v (%T), want %#v (%T)", tc.in, got, got, tc.want, tc.want)
			}
		})
	}
}
