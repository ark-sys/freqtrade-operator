package v1alpha1

import (
	"testing"
	"time"

	"github.com/ark-sys/freqtrade-operator/api/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/randfill"
)

// tradeBotConfigFuzzer fills a TradeBotConfig with random-but-valid data
// for the round-trip tests below - same Funcs as tradeBotFuzzer
// (tradebot_conversion_test.go) and for the same reasons: ManagedFields is
// server-side-apply's own generated content, never something conversion
// code constructs, and metav1.Time needs truncating to the second it's
// actually stored at.
func tradeBotConfigFuzzer(seed int64) *randfill.Filler {
	return randfill.NewWithSeed(seed).NilChance(0.5).NumElements(1, 3).Funcs(
		func(m *[]metav1.ManagedFieldsEntry, c randfill.Continue) {
			*m = nil
		},
		func(t *metav1.Time, c randfill.Continue) {
			*t = metav1.NewTime(metav1.Now().Truncate(time.Second))
		},
		// RiskManagementSpec.StartupCandle is *[]int - a pointer to a slice,
		// not just a slice. NumElements(1, 3) already keeps a non-nil slice
		// from ever being empty (see this fuzzer's own doc comment on why
		// that matters), but that's about the slice's own nil-ness, not the
		// outer pointer's: without this override, the outer pointer's
		// NilChance and the slice's own NilChance are decided independently,
		// so a non-nil pointer to a nil slice is reachable - and JSON
		// marshals that exactly as "null", identical to a nil pointer,
		// making convertJSON look lossy for a reason that has nothing to do
		// with the conversion code. Tying both decisions to one coin flip
		// avoids that state entirely.
		func(s **[]int, c randfill.Continue) {
			if c.Float64() < 0.5 {
				*s = nil
				return
			}
			v := make([]int, c.Intn(3)+1)
			for i := range v {
				v[i] = c.Int()
			}
			*s = &v
		},
	)
}

// clearPlaintextCredentials zeroes every field plaintextCredentialFields
// checks, leaving a TradeBotConfig that ConvertTo can actually represent in
// v1beta1 - the round-trip tests below cover the representable shape;
// TestTradeBotConfigConvertTo_RejectsPlaintextCredentials covers the
// rejected one.
func clearPlaintextCredentials(c *TradeBotConfig) {
	if c.Spec.Exchange != nil {
		c.Spec.Exchange.Key = ""
		c.Spec.Exchange.Secret = ""
		c.Spec.Exchange.Password = ""
		c.Spec.Exchange.UID = ""
		c.Spec.Exchange.WalletAddress = ""
		c.Spec.Exchange.PrivateKey = ""
	}
	if c.Spec.APIServer != nil {
		c.Spec.APIServer.Password = ""
		c.Spec.APIServer.JWTSecretKey = ""
	}
	if c.Spec.Notification != nil && c.Spec.Notification.Telegram != nil {
		c.Spec.Notification.Telegram.Token = ""
	}
}

// TestTradeBotConfigConvertRoundTrip_AlphaToBeta covers B2's own
// requirement: a v1alpha1 TradeBotConfig with no plaintext credentials set
// must round-trip through v1beta1 and back without losing anything.
// TypeMeta is cleared for the same reason tradebot_conversion_test.go
// clears it - never part of any conversion function's own contract.
func TestTradeBotConfigConvertRoundTrip_AlphaToBeta(t *testing.T) {
	for i := 0; i < 200; i++ {
		original := &TradeBotConfig{}
		tradeBotConfigFuzzer(int64(i)).Fill(original)
		original.TypeMeta = metav1.TypeMeta{}
		clearPlaintextCredentials(original)

		var converted v1beta1.TradeBotConfig
		if err := original.ConvertTo(&converted); err != nil {
			t.Fatalf("iteration %d: ConvertTo failed for a credential-free TradeBotConfig: %v", i, err)
		}

		roundTripped := &TradeBotConfig{}
		if err := roundTripped.ConvertFrom(&converted); err != nil {
			t.Fatalf("iteration %d: ConvertFrom failed: %v", i, err)
		}

		if !apiequality.Semantic.DeepEqual(original, roundTripped) {
			t.Errorf("iteration %d: round trip mismatch.\noriginal:  %+v\nroundtrip: %+v", i, original, roundTripped)
		}
	}
}

// TestTradeBotConfigConvertRoundTrip_BetaToAlpha covers the other
// direction: starting from v1beta1 (the shape a v1beta1-native client
// would actually send) must round-trip cleanly, since ConvertFrom never
// rejects anything (a v1beta1 TradeBotConfig is always representable in
// v1alpha1 - v1beta1's shape is a strict subset).
func TestTradeBotConfigConvertRoundTrip_BetaToAlpha(t *testing.T) {
	for i := 0; i < 200; i++ {
		original := &v1beta1.TradeBotConfig{}
		tradeBotConfigFuzzer(int64(i)).Fill(original)
		original.TypeMeta = metav1.TypeMeta{}

		converted := &TradeBotConfig{}
		if err := converted.ConvertFrom(original); err != nil {
			t.Fatalf("iteration %d: ConvertFrom failed: %v", i, err)
		}

		roundTripped := &v1beta1.TradeBotConfig{}
		if err := converted.ConvertTo(roundTripped); err != nil {
			t.Fatalf("iteration %d: ConvertTo failed converting back: %v", i, err)
		}

		if !apiequality.Semantic.DeepEqual(original, roundTripped) {
			t.Errorf("iteration %d: round trip mismatch.\noriginal:  %+v\nroundtrip: %+v", i, original, roundTripped)
		}
	}
}

// TestTradeBotConfigConvertTo_RejectsPlaintextCredentials is B2's most
// important test (REMAINING-WORK.md's own warning: "the conversion path is
// the dangerous part... fail the conversion instead, loudly, and test that
// path before you test the happy one"). A bot that starts with no exchange
// API key against a live exchange is worse than a failed conversion - every
// deprecated plaintext credential field must independently reject
// conversion, not just be silently dropped.
func TestTradeBotConfigConvertTo_RejectsPlaintextCredentials(t *testing.T) {
	const plaintextValue = "plaintext"

	base := func() *TradeBotConfig {
		return &TradeBotConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "trading"},
			Spec: TradeBotConfigSpec{
				Bot:      &BotConfig{},
				Exchange: &ExchangeSpec{Name: "binance"},
			},
		}
	}

	tests := []struct {
		name   string
		mutate func(*TradeBotConfig)
	}{
		{"exchange.key", func(c *TradeBotConfig) { c.Spec.Exchange.Key = plaintextValue }},
		{"exchange.secret", func(c *TradeBotConfig) { c.Spec.Exchange.Secret = plaintextValue }},
		{"exchange.password", func(c *TradeBotConfig) { c.Spec.Exchange.Password = plaintextValue }},
		{"exchange.uid", func(c *TradeBotConfig) { c.Spec.Exchange.UID = plaintextValue }},
		{"exchange.wallet_address", func(c *TradeBotConfig) { c.Spec.Exchange.WalletAddress = plaintextValue }},
		{"exchange.private_key", func(c *TradeBotConfig) { c.Spec.Exchange.PrivateKey = plaintextValue }},
		{"apiServer.password", func(c *TradeBotConfig) {
			c.Spec.APIServer = &APIServerConfig{Password: plaintextValue}
		}},
		{"apiServer.jwtSecretKey", func(c *TradeBotConfig) {
			c.Spec.APIServer = &APIServerConfig{JWTSecretKey: plaintextValue}
		}},
		{"notification.telegram.token", func(c *TradeBotConfig) {
			c.Spec.Notification = &NotificationSpec{Telegram: &NotificationTelegram{Token: plaintextValue}}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base()
			tt.mutate(cfg)

			var converted v1beta1.TradeBotConfig
			err := cfg.ConvertTo(&converted)
			if err == nil {
				t.Fatalf("expected ConvertTo to reject %s, got nil error", tt.name)
			}
		})
	}
}

// TestTradeBotConfigConvertTo_AccountIDIsNotACredential covers the
// deliberate exception: account_id is an identifier, not a secret (see
// configbuilder/exchange.go's own doc comment), so it's never gated by
// allowPlaintextCredentialsAnnotation and must convert cleanly.
func TestTradeBotConfigConvertTo_AccountIDIsNotACredential(t *testing.T) {
	cfg := &TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "trading"},
		Spec: TradeBotConfigSpec{
			Bot:      &BotConfig{},
			Exchange: &ExchangeSpec{Name: "binance", AccountID: "12345"},
		},
	}

	var converted v1beta1.TradeBotConfig
	if err := cfg.ConvertTo(&converted); err != nil {
		t.Fatalf("expected account_id alone not to block conversion, got: %v", err)
	}
	if converted.Spec.Exchange.AccountID != "12345" {
		t.Errorf("expected account_id to carry over, got %q", converted.Spec.Exchange.AccountID)
	}
}

// TestTradeBotConfigConvertTo_UnknownFeeRateKeepsA1sFix covers A1/B2
// together: the v1alpha1 field stays misspelled (UnkownFeeRate), but
// converts into the correctly-spelled v1beta1 field.
func TestTradeBotConfigConvertTo_UnknownFeeRateKeepsA1sFix(t *testing.T) {
	feeRate := true
	cfg := &TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "trading"},
		Spec: TradeBotConfigSpec{
			Bot:      &BotConfig{},
			Exchange: &ExchangeSpec{Name: "binance", UnkownFeeRate: &feeRate},
		},
	}

	var converted v1beta1.TradeBotConfig
	if err := cfg.ConvertTo(&converted); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted.Spec.Exchange.UnknownFeeRate == nil || !*converted.Spec.Exchange.UnknownFeeRate {
		t.Errorf("expected UnknownFeeRate=true, got %v", converted.Spec.Exchange.UnknownFeeRate)
	}
}

// TestTradeBotConfigConvertTo_SecretRefBecomesTypedReference covers the
// bare-string -> corev1.LocalObjectReference typing (P6-5's own
// requirement, extended to TradeBotConfig here).
func TestTradeBotConfigConvertTo_SecretRefBecomesTypedReference(t *testing.T) {
	cfg := &TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "trading"},
		Spec: TradeBotConfigSpec{
			Bot:       &BotConfig{},
			Exchange:  &ExchangeSpec{Name: "binance", SecretRef: "exchange-creds"},
			APIServer: &APIServerConfig{SecretRef: "api-creds"},
			Notification: &NotificationSpec{
				Telegram: &NotificationTelegram{SecretRef: "telegram-creds"},
			},
		},
	}

	var converted v1beta1.TradeBotConfig
	if err := cfg.ConvertTo(&converted); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if converted.Spec.Exchange.SecretRef.Name != "exchange-creds" {
		t.Errorf("expected exchange secretRef %q, got %q", "exchange-creds", converted.Spec.Exchange.SecretRef.Name)
	}
	if converted.Spec.APIServer.SecretRef.Name != "api-creds" {
		t.Errorf("expected apiServer secretRef %q, got %q", "api-creds", converted.Spec.APIServer.SecretRef.Name)
	}
	if converted.Spec.Notification.Telegram.SecretRef.Name != "telegram-creds" {
		t.Errorf("expected telegram secretRef %q, got %q",
			"telegram-creds", converted.Spec.Notification.Telegram.SecretRef.Name)
	}
}
