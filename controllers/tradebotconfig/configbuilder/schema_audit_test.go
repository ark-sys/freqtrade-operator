package configbuilder

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestConfigbuilderKeysExistInSchema is the D2 audit (REMAINING-WORK.md):
// A1 found its fee-rate key bug by accident, via a spelling linter.
// Freqtrade ignores unknown config keys silently, so a wrong key name is
// invisible at runtime - misspell only catches the subset of wrong names
// that happen to look like a typo of a dictionary word, which is why this
// needs its own test rather than relying on lint. This renders a fully-populated
// TradeBotConfig and walks every key in the result against schema.json (the
// vendored upstream Freqtrade config schema), failing loudly on any key
// schema.json doesn't recognize at that position.
//
// Coverage limits, both inherent to schema.json itself rather than a
// shortcut taken here: pairlist method-specific fields (number_assets,
// sort_key, min_days_listed, ...) aren't checked beyond "method" itself,
// because schema.json's pairlists.items.properties only declares "method" -
// it doesn't model the different fields each method type accepts. And any
// value under a schema node with no fixed "properties" (e.g. exchange's
// ccxt_config/ccxt_async_config/ccxt_sync_config, which are deliberately
// opaque user-supplied CCXT library config) is treated as opaque and not
// recursed into, for the same reason: schema.json doesn't model its shape.
// knownSchemaGaps are audit findings that are real but not safe to
// autocorrect the way the A1-class rename fixes were, because the correct
// fix needs either a Go API type change (a v1alpha1 breaking change, same
// deferral as A1's UnkownFeeRate - see B2) or more certainty than
// schema.json alone can offer. Each is a deliberate, tracked exception, not
// a weakening of the audit - fixing configbuilder's OTHER key names must
// not silently widen this list. Keyed by the same "path" the walk above
// reports failures at.
var knownSchemaGaps = map[string]map[string]bool{
	// NotificationWebhook's Entry/EntryFill/.../Status fields are plain
	// strings rendered as flat "webhookentry"/"webhookexit"/... keys - a
	// convention from an older Freqtrade webhook format. Current
	// schema.json types webhook.entry/exit/status/... (unprefixed) as
	// "object", i.e. an arbitrary user-defined payload dict, not a string.
	// Fixing this needs an API decision (does NotificationWebhook's Go
	// shape change to map[string]string per field, and is that a v1beta1-only
	// change like B2's credential fields?), not a rename.
	"config.json.webhook": {
		"webhookentry": true, "webhookentrycancel": true, "webhookentryfill": true,
		"webhookexit": true, "webhookexitcancel": true, "webhookexitfill": true,
		"webhookstatus": true,
		// allow_custom_messages isn't in schema.json's webhook properties at
		// all (it's a real key under telegram) - likely copy-pasted from
		// there; needs a decision on whether to drop it from webhook
		// rendering rather than a rename.
		"allow_custom_messages": true,
	},
	"config.json": {
		// BotConfig.PositionAdjustment is a string, rendered as
		// "position_adjustment". schema.json's actual key is
		// position_adjustment_enable, typed boolean - a type mismatch, not
		// just a name mismatch, so renaming the key alone would hand
		// freqtrade a string where it validates a bool at startup.
		"position_adjustment": true,
		// LoggingConfig.Version renders top-level "logging". schema.json
		// defines a "logging" schema (definitions.logging) but never
		// references it from a top-level property - i.e. this schema
		// doesn't consider top-level "logging" a valid config key at all.
		// Unclear whether freqtrade reads it via some other, unvalidated
		// path; left alone pending that answer rather than deleting a
		// feature on a guess.
		"logging": true,
	},
	"config.json.api_server": {
		// APIServerConfig.EnableOpenAPI renders "enable_openapi". No key
		// resembling this appears anywhere in schema.json - api_server's
		// full property set is enabled/listen_ip_address/listen_port/
		// username/password/ws_token/jwt_secret_key/CORS_origins/verbosity.
		// Left alone rather than guessing at a replacement with no evidence
		// for what it should be.
		"enable_openapi": true,
	},
}

func TestConfigbuilderKeysExistInSchema(t *testing.T) {
	schema := loadFreqtradeSchema(t)
	scheme := newTestScheme(t)

	exchangeSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "exchange-creds", Namespace: "trading"},
		Data: map[string][]byte{
			"api-key": []byte("test-api-key"),
			"secret":  []byte("test-api-secret"),
		},
	}
	telegramSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "telegram-creds", Namespace: "trading"},
		Data: map[string][]byte{
			"token":   []byte("test-telegram-token"),
			"chat-id": []byte("test-chat-id"),
		},
	}
	apiSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "api-creds", Namespace: "trading"},
		Data: map[string][]byte{
			"user":           []byte("test-user"),
			"password":       []byte("test-password"),
			"jwt_secret_key": []byte("test-jwt-secret-well-over-the-minimum-length"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(exchangeSecret, telegramSecret, apiSecret).Build()

	tradeBotConfig := fullTradeBotConfigFixture()
	extraCorsHosts := []string{"https://full-bot.frequi.example.com"}
	data, err := BuildConfig(context.Background(), c, "full-bot", "trading", tradeBotConfig, extraCorsHosts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rendered map[string]interface{}
	if err := json.Unmarshal([]byte(data["config.json"]), &rendered); err != nil {
		t.Fatalf("rendered config.json is not valid JSON: %v", err)
	}

	topProps, _ := schema["properties"].(map[string]interface{})
	walkSchemaKeys(t, schema, "config.json", topProps, rendered)
}

// loadFreqtradeSchema loads the vendored upstream Freqtrade config schema
// from the repo root.
func loadFreqtradeSchema(t *testing.T) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "schema.json"))
	if err != nil {
		t.Fatalf("failed to read schema.json: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("failed to parse schema.json: %v", err)
	}
	return schema
}

// resolveSchemaRef follows a {"$ref": "#/definitions/xxx"} node to the
// definition it points at, returning node unchanged if it isn't a $ref.
func resolveSchemaRef(schema, node map[string]interface{}) map[string]interface{} {
	ref, ok := node["$ref"].(string)
	if !ok {
		return node
	}
	name := strings.TrimPrefix(ref, "#/definitions/")
	defs, _ := schema["definitions"].(map[string]interface{})
	resolved, _ := defs[name].(map[string]interface{})
	return resolved
}

// schemaObjectProperties returns propNode's declared "properties" (after
// resolving $ref), or nil if it doesn't declare a fixed set - which the
// walk below treats as an opaque value and does not recurse into.
func schemaObjectProperties(schema, propNode map[string]interface{}) map[string]interface{} {
	resolved := resolveSchemaRef(schema, propNode)
	if resolved == nil {
		return nil
	}
	props, _ := resolved["properties"].(map[string]interface{})
	return props
}

// walkSchemaKeys asserts every key in got exists in props (schema.json's
// property set for this position), recursing into nested objects and the
// "pairlists" array. path is for failure messages only.
func walkSchemaKeys(
	t *testing.T, schema map[string]interface{}, path string, props map[string]interface{}, got map[string]interface{},
) {
	t.Helper()
	if props == nil {
		return // not modeled by schema.json - opaque, e.g. ccxt_config.
	}
	for key, val := range got {
		propNode, ok := props[key].(map[string]interface{})
		if !ok {
			if knownSchemaGaps[path][key] {
				continue
			}
			t.Errorf(
				"configbuilder renders key %q at %s, which is not a property in schema.json - "+
					"freqtrade silently ignores keys it doesn't recognize, so this is likely a wrong key name",
				key, path,
			)
			continue
		}

		childPath := path + "." + key
		switch v := val.(type) {
		case map[string]interface{}:
			walkSchemaKeys(t, schema, childPath, schemaObjectProperties(schema, propNode), v)
		case []interface{}:
			if key == "pairlists" {
				walkPairlists(t, childPath, propNode, v)
			}
		}
	}
}

// walkPairlists checks only each pairlist method entry's "method" key -
// schema.json's pairlists.items.properties doesn't model the fields
// specific to each method type (see the test's doc comment).
func walkPairlists(t *testing.T, path string, itemNode map[string]interface{}, items []interface{}) {
	t.Helper()
	itemSchema, _ := itemNode["items"].(map[string]interface{})
	itemProps, _ := itemSchema["properties"].(map[string]interface{})
	if _, ok := itemProps["method"]; !ok {
		t.Fatalf("schema.json's pairlists.items no longer declares \"method\" - walkPairlists needs updating for %s", path)
	}
	for i, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := item["method"]; !ok {
			t.Errorf("%s[%d] has no \"method\" key", path, i)
		}
	}
}
