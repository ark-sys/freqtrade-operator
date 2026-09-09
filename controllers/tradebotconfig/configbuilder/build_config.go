package configbuilder

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ark-sys/freqtrade-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ErrMissingExchange is returned when a TradeBotConfig has no exchange section.
// Exchange is an optional pointer at the API level, but Freqtrade cannot start
// without one, so BuildConfig treats it as effectively required.
var ErrMissingExchange = errors.New("tradeBotConfig.spec.exchange is required")

// minAPIServerJWTSecretKeyLength is the shortest jwt_secret_key BuildConfig
// will accept from a user-supplied source (P3-4). Below this, freqtrade
// would still start and sign JWTs with it - it just wouldn't mean much.
// generateAPIServerJWTSecretKey always produces a key well above this, so
// the floor only ever rejects an actual weak value, never one of ours.
const minAPIServerJWTSecretKeyLength = 32

// BuildConfig merges all configuration sections into a complete Freqtrade config
func BuildConfig(
	ctx context.Context,
	k8sClient client.Client,
	tradeBot *v1alpha1.TradeBot,
	tradeBotConfig *v1alpha1.TradeBotConfig,
	extraCorsHosts []string,
) (map[string]string, error) {
	if tradeBotConfig.Spec.Exchange == nil {
		return nil, ErrMissingExchange
	}

	// Create the main config map
	config := make(map[string]interface{})

	// Retrieve apiCredentials if any. APIServer is optional; keep the secret ref empty when unset.
	var apiServerSecretRef string
	if tradeBotConfig.Spec.APIServer != nil {
		apiServerSecretRef = tradeBotConfig.Spec.APIServer.SecretRef
	}
	apiCredentials, err := GetSecretData(ctx, k8sClient, tradeBot.Namespace, apiServerSecretRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get API credentials: %w", err)
	}

	jwtSecretKey, err := resolveAPIServerJWTSecretKey(ctx, k8sClient, tradeBot, tradeBotConfig, apiCredentials)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve API server JWT secret key: %w", err)
	}

	tradeBotName := tradeBot.Name

	// Add bot-level configuration
	botConfig, err := BuildTradeBotConfig(tradeBotName, tradeBotConfig, apiCredentials, jwtSecretKey, extraCorsHosts)
	if err != nil {
		return nil, fmt.Errorf("failed to build TradeBot config: %w", err)
	}

	for k, v := range botConfig {
		config[k] = v
	}

	// Add exchange configuration
	exchangeSecretData, err := GetSecretData(ctx, k8sClient, tradeBot.Namespace, tradeBotConfig.Spec.Exchange.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("failed to get exchange secret data: %w", err)
	}

	exchangeConfig, err := BuildExchangeConfig(tradeBotConfig.Spec.Exchange, exchangeSecretData)
	if err != nil {
		return nil, fmt.Errorf("failed to build Exchange config: %w", err)
	}

	if exchangeConfig != nil {
		config["exchange"] = exchangeConfig
	}

	// Add pairlists configuration if specified
	pairlistsConfig := BuildPairlistMethodsConfig(tradeBotConfig.Spec.PairlistMethod)
	if pairlistsConfig != nil {
		for k, v := range pairlistsConfig {
			config[k] = v
		}
	}

	// Add entry pricing configuration
	entryPricingConfig := BuildPricingConfig(tradeBotConfig.Spec.EntryPricing)
	if entryPricingConfig != nil {
		config["entry_pricing"] = entryPricingConfig
	}

	// Add exit pricing configuration
	exitPricingConfig := BuildPricingConfig(tradeBotConfig.Spec.ExitPricing)
	if exitPricingConfig != nil {
		config["exit_pricing"] = exitPricingConfig
	}

	// Add order types configuration
	orderTypesConfig := BuildOrderTypesConfig(tradeBotConfig.Spec.Order)
	if orderTypesConfig != nil {
		config["order_types"] = orderTypesConfig
	}

	// Add order time in force configuration
	orderTimeInForceConfig := BuildOrderTimeInForceConfig(tradeBotConfig.Spec.Order)
	if orderTimeInForceConfig != nil {
		config["order_time_in_force"] = orderTimeInForceConfig
	}

	// Add orderflow configuration
	orderflowConfig := BuildOrderflowConfig(tradeBotConfig.Spec.Order)
	if orderflowConfig != nil {
		config["orderflow"] = orderflowConfig
	}

	// Add risk management configuration
	riskConfig := BuildRiskManagementConfig(tradeBotConfig.Spec.RiskManagement)
	if riskConfig != nil {
		for k, v := range riskConfig {
			config[k] = v
		}
	}

	// Add notification configuration
	var notificationSecretData map[string][]byte
	if tradeBotConfig.Spec.Notification != nil && tradeBotConfig.Spec.Notification.Telegram != nil {
		notificationSecretData, err = GetSecretData(ctx, k8sClient, tradeBot.Namespace, tradeBotConfig.Spec.Notification.Telegram.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to get notification secret data: %w", err)
		}
	}
	notificationConfig, err := BuildNotificationConfig(tradeBotConfig.Spec.Notification, notificationSecretData)
	if err != nil {
		return nil, fmt.Errorf("failed to build Notification config: %w", err)
	}

	if notificationConfig != nil {
		for k, v := range notificationConfig {
			config[k] = v
		}
	}

	// Marshal to JSON
	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	return map[string]string{"config.json": string(configBytes)}, nil
}

// GetSecretData retrieves data from a Kubernetes Secret
func GetSecretData(ctx context.Context, k8sClient client.Client, namespace, name string) (map[string][]byte, error) {
	if name == "" {
		return nil, nil
	}

	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
	}

	return secret.Data, nil
}

// resolveAPIServerJWTSecretKey decides what, if anything, BuildTradeBotConfig
// should write as api_server.jwt_secret_key (P3-4). Freqtrade signs API JWTs
// with this value, so an absent or weak one is a real weakness, not just a
// missing default - but it only matters while the API server is actually
// enabled; a present-but-off apiServer block is left alone entirely.
//
// secretRef (via apiCredentials) always wins over the deprecated plaintext
// field, matching the precedence used for username/password everywhere else
// in this package. Either way, a value that's clearly not a real secret is
// rejected rather than silently forwarded to freqtrade. An absent value is
// generated - and, so a config change doesn't rotate it on every reconcile
// for no reason, reused from whatever this TradeBot's own previously
// rendered config Secret already has, if anything.
func resolveAPIServerJWTSecretKey(
	ctx context.Context,
	k8sClient client.Client,
	tradeBot *v1alpha1.TradeBot,
	tradeBotConfig *v1alpha1.TradeBotConfig,
	apiCredentials map[string][]byte,
) (string, error) {
	apiServer := tradeBotConfig.Spec.APIServer
	if apiServer == nil || apiServer.Enabled == nil || !*apiServer.Enabled {
		return "", nil
	}

	key := apiServer.JWTSecretKey // deprecated plaintext fallback
	if len(apiCredentials["jwt_secret_key"]) > 0 {
		key = string(apiCredentials["jwt_secret_key"])
	}
	if key != "" {
		if len(key) < minAPIServerJWTSecretKeyLength {
			return "", fmt.Errorf("api_server.jwt_secret_key is %d characters, shorter than the required minimum of %d",
				len(key), minAPIServerJWTSecretKeyLength)
		}
		return key, nil
	}

	existing, err := existingAPIServerJWTSecretKey(ctx, k8sClient, tradeBot)
	if err != nil {
		return "", err
	}
	if existing != "" {
		return existing, nil
	}
	return generateAPIServerJWTSecretKey()
}

// existingAPIServerJWTSecretKey reads back whatever jwt_secret_key (if any)
// is already in this TradeBot's own rendered config Secret, so a
// once-generated key survives every later reconcile instead of rotating -
// which would invalidate every JWT freqtrade had already issued for no
// reason. "" (with no error) covers both a first-ever reconcile (the Secret
// doesn't exist yet) and any other reason the field isn't cleanly readable -
// either way, the caller's fallback is simply to generate a fresh one.
func existingAPIServerJWTSecretKey(
	ctx context.Context, k8sClient client.Client, tradeBot *v1alpha1.TradeBot,
) (string, error) {
	secret := &corev1.Secret{}
	// Must match resources.BuildSecret's naming in controllers/tradebot/resources/secret.go.
	name := types.NamespacedName{Namespace: tradeBot.Namespace, Name: tradeBot.Name + "-config"}
	if err := k8sClient.Get(ctx, name, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to get existing config secret %s: %w", name, err)
	}

	var existing struct {
		APIServer struct {
			JWTSecretKey string `json:"jwt_secret_key"`
		} `json:"api_server"`
	}
	if err := json.Unmarshal(secret.Data["config.json"], &existing); err != nil {
		return "", nil
	}
	return existing.APIServer.JWTSecretKey, nil
}

// generateAPIServerJWTSecretKey returns 32 crypto/rand bytes, hex-encoded -
// comfortably past minAPIServerJWTSecretKeyLength and, unlike math/rand, not
// predictable from other output.
func generateAPIServerJWTSecretKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate a JWT secret key: %w", err)
	}
	return hex.EncodeToString(b), nil
}
