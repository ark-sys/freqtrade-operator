package configbuilder

import (
	"testing"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// TestBuildExchangeConfig_SecretOverridesPlaintext covers the P3-1 fix:
// secretRef must always win over a deprecated plaintext credential field
// when both happen to be set (only possible at all with
// freqtrade.io/allow-plaintext-credentials, which the webhook requires for
// the plaintext field to be accepted in the first place).
func TestBuildExchangeConfig_SecretOverridesPlaintext(t *testing.T) {
	exchange := &v1alpha1.ExchangeSpec{
		Name:      "binance",
		SecretRef: "exchange-creds",
		Key:       "plaintext-key",
		UID:       "plaintext-uid",
	}
	secretData := map[string][]byte{
		"api-key": []byte("secret-key"),
		"secret":  []byte("secret-value"),
	}

	got, err := BuildExchangeConfig(exchange, secretData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["key"] != "secret-key" {
		t.Errorf("expected the secretRef value to win over the plaintext field, got %v", got["key"])
	}
	if got["secret"] != "secret-value" {
		t.Errorf("expected the secret-only field to still come through, got %v", got["secret"])
	}
	if got["uid"] != "plaintext-uid" {
		t.Errorf("expected the plaintext field to still apply when the Secret has no corresponding key, got %v", got["uid"])
	}
}

func TestBuildExchangeConfig_CcxtAsyncAndSyncConfig(t *testing.T) {
	exchange := &v1alpha1.ExchangeSpec{
		Name:            "binance",
		CcxtAsyncConfig: apiextensionsv1.JSON{Raw: []byte(`{"aiohttp_trust_env":true}`)},
		CcxtSyncConfig:  apiextensionsv1.JSON{Raw: []byte(`{"verbose":false}`)},
	}

	got, err := BuildExchangeConfig(exchange, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	asyncCfg, ok := got["ccxt_async_config"].(map[string]interface{})
	if !ok || asyncCfg["aiohttp_trust_env"] != true {
		t.Errorf("expected ccxt_async_config to round-trip, got %v", got["ccxt_async_config"])
	}
	syncCfg, ok := got["ccxt_sync_config"].(map[string]interface{})
	if !ok || syncCfg["verbose"] != false {
		t.Errorf("expected ccxt_sync_config to round-trip, got %v", got["ccxt_sync_config"])
	}
}

// TestBuildNotificationConfig_TelegramPlaintextFallback covers the branch
// the golden fixture doesn't: no SecretRef means Token/ChatID are read from
// the plaintext spec fields instead of a Secret.
func TestBuildNotificationConfig_TelegramPlaintextFallback(t *testing.T) {
	notification := &v1alpha1.NotificationSpec{
		Telegram: &v1alpha1.NotificationTelegram{
			Enabled: ptrBool(true),
			Token:   "plaintext-token",
			ChatID:  "plaintext-chat-id",
		},
	}

	got, err := BuildNotificationConfig(notification, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	telegram, ok := got["telegram"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a telegram section, got %v", got["telegram"])
	}
	if telegram["token"] != "plaintext-token" {
		t.Errorf("expected plaintext token, got %v", telegram["token"])
	}
	if telegram["chat_id"] != "plaintext-chat-id" {
		t.Errorf("expected plaintext chat_id, got %v", telegram["chat_id"])
	}
}
