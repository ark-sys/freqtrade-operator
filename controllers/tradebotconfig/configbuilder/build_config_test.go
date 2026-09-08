package configbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1alpha1 to scheme: %v", err)
	}
	return scheme
}

func newTestTradeBot() *v1alpha1.TradeBot {
	return &v1alpha1.TradeBot{
		ObjectMeta: metav1.ObjectMeta{Name: "test-bot", Namespace: "default"},
		Spec: v1alpha1.TradeBotSpec{
			Config:   "test-config",
			Strategy: "test-strategy",
		},
	}
}

func TestBuildConfig_MissingExchangeReturnsError(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	tradeBotConfig := &v1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "default"},
		Spec: v1alpha1.TradeBotConfigSpec{
			Bot: &v1alpha1.BotConfig{},
			// Exchange intentionally omitted.
		},
	}

	_, err := BuildConfig(context.Background(), c, newTestTradeBot(), tradeBotConfig, nil)
	if err == nil {
		t.Fatal("expected an error when Spec.Exchange is nil, got nil")
	}
	if !errors.Is(err, ErrMissingExchange) {
		t.Fatalf("expected ErrMissingExchange, got: %v", err)
	}
}

func TestBuildConfig_OptionalSectionsOmittedNoPanic(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	tradeBotConfig := &v1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "default"},
		Spec: v1alpha1.TradeBotConfigSpec{
			Bot:      &v1alpha1.BotConfig{BotName: "test-bot"},
			Exchange: &v1alpha1.ExchangeSpec{Name: "binance"},
			// APIServer and Notification intentionally nil.
		},
	}

	data, err := BuildConfig(context.Background(), c, newTestTradeBot(), tradeBotConfig, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := data["config.json"]
	if !ok {
		t.Fatal("expected config.json key in returned data")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("config.json is not valid JSON: %v", err)
	}
	if parsed["bot_name"] != "test-bot" {
		t.Errorf("expected bot_name %q, got %v", "test-bot", parsed["bot_name"])
	}
	exchange, ok := parsed["exchange"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected exchange section, got %v", parsed["exchange"])
	}
	if exchange["name"] != "binance" {
		t.Errorf("expected exchange.name %q, got %v", "binance", exchange["name"])
	}
	if _, present := parsed["api_server"]; present {
		t.Errorf("did not expect api_server section when APIServer is nil, got %v", parsed["api_server"])
	}
}

// TestBuildConfig_NilOptionalBoolPointersNoPanic exercises the other nil-pointer
// traps found alongside the APIServer/Exchange bug: Experimental.BlockBadExchanges
// and Notification.Webhook.Enabled/AllowCustomMessages can each be a non-nil
// parent struct with a nil *bool field.
func TestBuildConfig_NilOptionalBoolPointersNoPanic(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	tradeBotConfig := &v1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "default"},
		Spec: v1alpha1.TradeBotConfigSpec{
			Bot:          &v1alpha1.BotConfig{},
			Exchange:     &v1alpha1.ExchangeSpec{Name: "binance"},
			Experimental: &v1alpha1.ExperimentalConfig{}, // BlockBadExchanges left nil
			Notification: &v1alpha1.NotificationSpec{
				Webhook: &v1alpha1.NotificationWebhook{URL: "http://example.invalid"}, // Enabled left nil
			},
		},
	}

	data, err := BuildConfig(context.Background(), c, newTestTradeBot(), tradeBotConfig, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := data["config.json"]; !ok {
		t.Fatal("expected config.json key in returned data")
	}
}

func TestBuildConfig_ExchangeSecretPrecedence(t *testing.T) {
	scheme := newTestScheme(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "exchange-creds", Namespace: "default"},
		Data: map[string][]byte{
			"api-key": []byte("secret-key"),
			"secret":  []byte("secret-value"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	tradeBotConfig := &v1alpha1.TradeBotConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "default"},
		Spec: v1alpha1.TradeBotConfigSpec{
			Bot: &v1alpha1.BotConfig{},
			Exchange: &v1alpha1.ExchangeSpec{
				Name:      "binance",
				SecretRef: "exchange-creds",
			},
		},
	}

	data, err := BuildConfig(context.Background(), c, newTestTradeBot(), tradeBotConfig, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(data["config.json"]), &parsed); err != nil {
		t.Fatalf("config.json is not valid JSON: %v", err)
	}
	exchange := parsed["exchange"].(map[string]interface{})
	if exchange["key"] != "secret-key" {
		t.Errorf("expected exchange.key from secret, got %v", exchange["key"])
	}
}
