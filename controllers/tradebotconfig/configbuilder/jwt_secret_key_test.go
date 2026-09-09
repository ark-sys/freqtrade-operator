package configbuilder

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func apiServerConfigFixture() *v1alpha1.TradeBotConfig {
	return &v1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "default"},
		Spec: v1alpha1.TradeBotConfigSpec{
			Bot:      &v1alpha1.BotConfig{},
			Exchange: &v1alpha1.ExchangeSpec{Name: "binance"},
			APIServer: &v1alpha1.APIServerConfig{
				Enabled: ptrBool(true),
			},
		},
	}
}

func apiServerJWTSecretKeyFrom(t *testing.T, data map[string]string) string {
	t.Helper()
	var parsed struct {
		APIServer struct {
			JWTSecretKey string `json:"jwt_secret_key"`
		} `json:"api_server"`
	}
	if err := json.Unmarshal([]byte(data["config.json"]), &parsed); err != nil {
		t.Fatalf("config.json is not valid JSON: %v", err)
	}
	return parsed.APIServer.JWTSecretKey
}

func TestBuildConfig_JWTSecretKeyGeneratedWhenAbsent(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	data, err := BuildConfig(context.Background(), c, newTestTradeBot(), apiServerConfigFixture(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := apiServerJWTSecretKeyFrom(t, data)
	if len(got) < minAPIServerJWTSecretKeyLength {
		t.Errorf("expected a generated key at least %d characters, got %q (%d characters)",
			minAPIServerJWTSecretKeyLength, got, len(got))
	}
}

func TestBuildConfig_JWTSecretKeyNotGeneratedWhenDisabled(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	tradeBotConfig := apiServerConfigFixture()
	tradeBotConfig.Spec.APIServer.Enabled = ptrBool(false)

	data, err := BuildConfig(context.Background(), c, newTestTradeBot(), tradeBotConfig, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := apiServerJWTSecretKeyFrom(t, data); got != "" {
		t.Errorf("expected no jwt_secret_key when api_server is disabled, got %q", got)
	}
}

// TestBuildConfig_JWTSecretKeyReusedAcrossReconciles covers the point of
// generating rather than just validating: a freshly generated key has to
// survive every later reconcile unchanged, or every JWT freqtrade already
// issued would be invalidated on the next config touch for no reason. The
// only place that value can be read back from is this TradeBot's own
// previously rendered config Secret (see existingAPIServerJWTSecretKey).
func TestBuildConfig_JWTSecretKeyReusedAcrossReconciles(t *testing.T) {
	scheme := newTestScheme(t)
	tradeBot := newTestTradeBot()
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: tradeBot.Name + "-config", Namespace: tradeBot.Namespace},
		Data: map[string][]byte{
			"config.json": []byte(`{"api_server": {"jwt_secret_key": "previously-generated-key-still-in-use"}}`),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existingSecret).Build()

	data, err := BuildConfig(context.Background(), c, tradeBot, apiServerConfigFixture(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := apiServerJWTSecretKeyFrom(t, data); got != "previously-generated-key-still-in-use" {
		t.Errorf("expected the existing generated key to be reused, got %q", got)
	}
}

func TestBuildConfig_JWTSecretKeyTooShortIsRejected(t *testing.T) {
	scheme := newTestScheme(t)
	apiSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "api-creds", Namespace: "default"},
		Data:       map[string][]byte{"jwt_secret_key": []byte("too-short")},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(apiSecret).Build()

	tradeBotConfig := apiServerConfigFixture()
	tradeBotConfig.Spec.APIServer.SecretRef = "api-creds"

	_, err := BuildConfig(context.Background(), c, newTestTradeBot(), tradeBotConfig, nil)
	if err == nil {
		t.Fatal("expected an error for a jwt_secret_key shorter than the minimum, got nil")
	}
}

// TestBuildConfig_JWTSecretKeySecretRefOverridesPlaintext mirrors the
// precedence already established for username/password (P3-1): a secretRef
// value must win over the deprecated plaintext field whenever both resolve
// to something, not just when the plaintext field is absent.
func TestBuildConfig_JWTSecretKeySecretRefOverridesPlaintext(t *testing.T) {
	scheme := newTestScheme(t)
	apiSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "api-creds", Namespace: "default"},
		Data:       map[string][]byte{"jwt_secret_key": []byte("from-secret-ref-well-over-the-minimum")},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(apiSecret).Build()

	tradeBotConfig := apiServerConfigFixture()
	tradeBotConfig.Spec.APIServer.SecretRef = "api-creds"
	tradeBotConfig.Spec.APIServer.JWTSecretKey = "from-plaintext-field-well-over-the-minimum"

	data, err := BuildConfig(context.Background(), c, newTestTradeBot(), tradeBotConfig, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := apiServerJWTSecretKeyFrom(t, data); got != "from-secret-ref-well-over-the-minimum" {
		t.Errorf("expected the secretRef value to win over plaintext, got %q", got)
	}
}
