package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

const (
	DefaultAppPort           = 4000
	DefaultLogLevel          = "debug"
	DefaultLogFormat         = "json"
	DefaultDBMaxOpenConns    = 25
	DefaultDBMaxIdleConns    = 5
	DefaultDBConnMaxLifetime = 5 * time.Minute
	DefaultJWTAccessTTL      = 15 * time.Minute
	DefaultJWTRefreshTTL     = 7 * 24 * time.Hour // 1 week
	DefaultBillingTrialDays  = 7
	DefaultPluginsDir             = "./plugins"
	DefaultMaxPlugins             = 50
	DefaultPluginHotReload        = false
	DefaultHealthCheckInterval    = 10 // seconds
	DefaultMaxConcurrentChecks    = 50
	DefaultSpeedTestPort          = 4203
	DefaultSubscriptionProxyPort  = 4100
)

type AppConfig struct {
	Port      int    `koanf:"port"`
	LogLevel  string `koanf:"log_level"`
	LogFormat string `koanf:"log_format"`
}

type DatabaseConfig struct {
	URL             string        `koanf:"url"`
	MaxOpenConns    int           `koanf:"max_open_conns"`
	MaxIdleConns    int           `koanf:"max_idle_conns"`
	ConnMaxLifetime time.Duration `koanf:"conn_max_lifetime"`
}

type ValkeyConfig struct {
	URL string `koanf:"url"`
}

type NATSConfig struct {
	URL string `koanf:"url"`
}

type JWTConfig struct {
	PrivateKeyPath  string        `koanf:"private_key_path"`
	PublicKeyPath   string        `koanf:"public_key_path"`
	AccessTokenTTL  time.Duration `koanf:"access_token_ttl"`
	RefreshTokenTTL time.Duration `koanf:"refresh_token_ttl"`
}

type RemnawaveConfig struct {
	URL           string `koanf:"url"`
	APIToken      string `koanf:"api_token"`
	WebhookSecret string `koanf:"webhook_secret"`
}

type BillingConfig struct {
	TrialDays int `koanf:"trial_days"`
}

type PluginConfig struct {
	PluginsDir      string `koanf:"dir"`
	MaxPlugins      int    `koanf:"max_plugins"`
	EnableHotReload bool   `koanf:"hot_reload"`
}

type TelegramConfig struct {
	BotToken   string `koanf:"bot_token"`
	WebhookURL string `koanf:"webhook_url"`
	CabinetURL string `koanf:"cabinet_url"`
}

// InfraConfig holds settings for in-process infrastructure services.
type InfraConfig struct {
	HealthCheckInterval   time.Duration `koanf:"health_check_interval"`
	MaxConcurrentChecks   int           `koanf:"max_concurrent_checks"`
	SpeedTestPort         int           `koanf:"speed_test_port"`
	SubscriptionProxyPort int           `koanf:"subscription_proxy_port"`
}

// CORSConfig holds the Cross-Origin Resource Sharing configuration.
type CORSConfig struct {
	AllowedOrigins []string `koanf:"allowed_origins"`
}

type Config struct {
	App       AppConfig       `koanf:"app"`
	Database  DatabaseConfig  `koanf:"database"`
	Valkey    ValkeyConfig    `koanf:"valkey"`
	NATS      NATSConfig      `koanf:"nats"`
	JWT       JWTConfig       `koanf:"jwt"`
	Remnawave RemnawaveConfig `koanf:"remnawave"`
	Billing   BillingConfig   `koanf:"billing"`
	Plugin    PluginConfig    `koanf:"plugin"`
	Telegram  TelegramConfig  `koanf:"telegram"`
	Infra     InfraConfig     `koanf:"infra"`
	CORS      CORSConfig      `koanf:"cors"`
}

// requiredField maps an environment variable name to the koanf key path used
// for validation after loading.
type requiredField struct {
	envVar   string
	koanfKey string
}

var requiredFields = []requiredField{
	{envVar: "DATABASE_URL", koanfKey: "database.url"},
	{envVar: "VALKEY_URL", koanfKey: "valkey.url"},
	{envVar: "NATS_URL", koanfKey: "nats.url"},
	{envVar: "REMNAWAVE_URL", koanfKey: "remnawave.url"},
	{envVar: "REMNAWAVE_API_TOKEN", koanfKey: "remnawave.api_token"},
	{envVar: "JWT_PRIVATE_KEY_PATH", koanfKey: "jwt.private_key_path"},
	{envVar: "JWT_PUBLIC_KEY_PATH", koanfKey: "jwt.public_key_path"},
}

// Load reads configuration from environment variables and returns a validated
// Config. Required fields that are empty cause an error.
func Load() (*Config, error) {
	k := koanf.New(".")

	// Set defaults
	defaults := map[string]any{
		"app.port":                   DefaultAppPort,
		"app.log_level":             DefaultLogLevel,
		"app.log_format":            DefaultLogFormat,
		"database.max_open_conns":   DefaultDBMaxOpenConns,
		"database.max_idle_conns":   DefaultDBMaxIdleConns,
		"database.conn_max_lifetime": DefaultDBConnMaxLifetime,
		"jwt.access_token_ttl":      DefaultJWTAccessTTL,
		"jwt.refresh_token_ttl":     DefaultJWTRefreshTTL,
		"billing.trial_days":        DefaultBillingTrialDays,
		"plugin.dir":                     DefaultPluginsDir,
		"plugin.max_plugins":             DefaultMaxPlugins,
		"plugin.hot_reload":              DefaultPluginHotReload,
		"infra.health_check_interval":    time.Duration(DefaultHealthCheckInterval) * time.Second,
		"infra.max_concurrent_checks":    DefaultMaxConcurrentChecks,
		"infra.speed_test_port":          DefaultSpeedTestPort,
		"infra.subscription_proxy_port":  DefaultSubscriptionProxyPort,
	}
	for key, val := range defaults {
		k.Set(key, val) //nolint:errcheck // Set on a fresh koanf instance cannot fail
	}

	// Load each prefix group from environment variables.
	prefixes := []string{"APP_", "DATABASE_", "VALKEY_", "NATS_", "JWT_", "REMNAWAVE_", "BILLING_", "PLUGIN_", "TELEGRAM_", "INFRA_", "CORS_"}
	for _, prefix := range prefixes {
		provider := env.Provider(prefix, ".", func(s string) string {
			// Strip prefix then lowercase and replace _ with . for nesting
			// e.g. "DATABASE_MAX_OPEN_CONNS" → "database.max_open_conns"
			section := strings.ToLower(strings.TrimPrefix(s, prefix))
			group := strings.ToLower(strings.TrimSuffix(prefix, "_"))
			return group + "." + section
		})
		if err := k.Load(provider, nil); err != nil {
			return nil, fmt.Errorf("loading env vars with prefix %s: %w", prefix, err)
		}
	}

	var cfg Config
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	if err := validateRequired(k); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateRequired(k *koanf.Koanf) error {
	var missing []string
	for _, f := range requiredFields {
		if strings.TrimSpace(k.String(f.koanfKey)) == "" {
			missing = append(missing, f.envVar)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}
