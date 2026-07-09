package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

const (
	DefaultAppPort                             = 4000
	DefaultLogLevel                            = "debug"
	DefaultLogFormat                           = "json"
	DefaultPoolMaxConns                  int32 = 20
	DefaultPoolMinConns                  int32 = 5
	DefaultPoolMaxConnLifetime                 = 1 * time.Hour
	DefaultPoolMaxConnIdleTime                 = 30 * time.Minute
	DefaultPoolHealthCheck                     = 1 * time.Minute
	DefaultJWTAccessTTL                        = 15 * time.Minute
	DefaultJWTRefreshTTL                       = 7 * 24 * time.Hour // 1 week
	DefaultBillingTrialDays                    = 7
	DefaultPluginsDir                          = "./plugins"
	DefaultMaxPlugins                          = 50
	DefaultPluginHotReload                     = false
	DefaultHealthCheckInterval                 = 10 // seconds
	DefaultMaxConcurrentChecks                 = 50
	DefaultSpeedTestPort                       = 4203
	DefaultSubscriptionProxyPort               = 4100
	DefaultCheckoutMaxPerHour                  = 10
	DefaultSubscriptionMaxPerDay               = 5
	DefaultLoginMaxPerWindow                   = 20
	DefaultLoginWindowMinutes                  = 15
	DefaultForgotPwdMaxPerWindow               = 3
	DefaultForgotPwdWindowMinutes              = 60
	DefaultAcceptInvitationMaxPerWindow        = 3
	DefaultAcceptInvitationWindowMinutes       = 60
	DefaultOutboxRelayWorkers                  = 1
	DefaultOutboxPartitionLookahead            = 2
	DefaultOutboxRetentionDays                 = 90
	DefaultHooksSubscriptionEnabled            = false
	DefaultHooksVPNProviderEnabled             = false

	// Smart router default weights — browsing.
	DefaultWeightGeo     = 0.33
	DefaultWeightLatency = 0.34
	DefaultWeightLoad    = 0.33

	// Smart router default weights — gaming.
	DefaultWeightGamingGeo     = 0.2
	DefaultWeightGamingLatency = 0.6
	DefaultWeightGamingLoad    = 0.2

	// Smart router default weights — streaming.
	DefaultWeightStreamingGeo     = 0.5
	DefaultWeightStreamingLatency = 0.2
	DefaultWeightStreamingLoad    = 0.3

	// Speed test rate limiting defaults.
	DefaultSpeedTestMaxConcurrent  = 10
	DefaultSpeedTestPerIPRateLimit = 3
	DefaultSpeedTestMaxUploadBytes = 10 * 1024 * 1024 // 10 MB
)

// DefaultAppVersion is used when no APP_VERSION environment variable is set.
const DefaultAppVersion = "dev"

type AppConfig struct {
	Port      int    `koanf:"port"`
	Version   string `koanf:"version"`
	LogLevel  string `koanf:"log_level"`
	LogFormat string `koanf:"log_format"`
}

type DatabaseConfig struct {
	URL               string        `koanf:"url"`
	MaxConns          int32         `koanf:"max_conns"`
	MinConns          int32         `koanf:"min_conns"`
	MaxConnLifetime   time.Duration `koanf:"max_conn_lifetime"`
	MaxConnIdleTime   time.Duration `koanf:"max_conn_idle_time"`
	HealthCheckPeriod time.Duration `koanf:"health_check_period"`
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
	// AllowEphemeralKey permits generating a throwaway in-memory signing key when
	// the key file is missing. DEV ONLY: an ephemeral key invalidates all tokens
	// on restart and makes multi-replica deployments reject each other's tokens,
	// so production must ship a real key file. Defaults to false (boot fails if
	// the key is absent).
	AllowEphemeralKey bool `koanf:"allow_ephemeral_key"`
}

type RemnawaveConfig struct {
	URL                   string       `koanf:"url"`
	APIToken              SecretString `koanf:"api_token"`
	WebhookSecret         SecretString `koanf:"webhook_secret"`
	DefaultInternalSquads []string     `koanf:"default_internal_squads"`
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
	BotToken   SecretString `koanf:"bot_token"`
	WebhookURL string       `koanf:"webhook_url"`
	CabinetURL string       `koanf:"cabinet_url"`
}

type SecurityConfig struct {
	// EncryptionKey is a base64-encoded 32-byte AES key for at-rest secret
	// encryption (e.g. per-shop bot tokens). Env: SECURITY_ENCRYPTION_KEY.
	EncryptionKey SecretString `koanf:"encryption_key"`
}

// InfraConfig holds settings for in-process infrastructure services.
type InfraConfig struct {
	HealthCheckInterval   time.Duration `koanf:"health_check_interval"`
	MaxConcurrentChecks   int           `koanf:"max_concurrent_checks"`
	SpeedTestPort         int           `koanf:"speed_test_port"`
	SubscriptionProxyPort int           `koanf:"subscription_proxy_port"`
}

// SmartRouterConfig holds configurable weights for the smart routing algorithm.
// Each weight triple controls the relative importance of geo proximity, latency,
// and load for a given connection purpose.
type SmartRouterConfig struct {
	WeightGeo     float64 `koanf:"weight_geo"`
	WeightLatency float64 `koanf:"weight_latency"`
	WeightLoad    float64 `koanf:"weight_load"`

	WeightGamingGeo     float64 `koanf:"weight_gaming_geo"`
	WeightGamingLatency float64 `koanf:"weight_gaming_latency"`
	WeightGamingLoad    float64 `koanf:"weight_gaming_load"`

	WeightStreamingGeo     float64 `koanf:"weight_streaming_geo"`
	WeightStreamingLatency float64 `koanf:"weight_streaming_latency"`
	WeightStreamingLoad    float64 `koanf:"weight_streaming_load"`
}

// SpeedTestConfig holds settings for the speed test server rate limiting.
type SpeedTestConfig struct {
	MaxConcurrent  int `koanf:"max_concurrent"`
	PerIPRateLimit int `koanf:"per_ip_rate_limit"`
	MaxUploadBytes int `koanf:"max_upload_bytes"`
}

// TracingConfig holds OpenTelemetry tracing configuration.
type TracingConfig struct {
	Endpoint string `koanf:"endpoint"` // OTLP HTTP endpoint (e.g., "localhost:4318"); empty disables tracing
}

// RateLimitConfig holds domain-level rate limit thresholds.
type RateLimitConfig struct {
	CheckoutMaxPerHour            int `koanf:"checkout_max_per_hour"`
	SubscriptionMaxPerDay         int `koanf:"subscription_max_per_day"`
	LoginMaxPerWindow             int `koanf:"login_max_per_window"`
	LoginWindowMinutes            int `koanf:"login_window_minutes"`
	ForgotPwdMaxPerWindow         int `koanf:"forgot_pwd_max_per_window"`
	ForgotPwdWindowMinutes        int `koanf:"forgot_pwd_window_minutes"`
	AcceptInvitationMaxPerWindow  int `koanf:"accept_invitation_max_per_window"`
	AcceptInvitationWindowMinutes int `koanf:"accept_invitation_window_minutes"`
}

// OutboxConfig holds settings for the transactional outbox relay and
// automatic partition lifecycle management.
type OutboxConfig struct {
	RelayWorkers       int `koanf:"relay_workers"`
	PartitionLookahead int `koanf:"partition_lookahead"` // quarters ahead to pre-create, default 2
	RetentionDays      int `koanf:"retention_days"`      // 0 = disable cleanup, default 90
}

// FeatureFlags controls gradual rollout of new capabilities.
type FeatureFlags struct {
	// HooksSubscriptionEnabled enables subscription lifecycle hook dispatch points.
	// When false, BillingService skips all hook dispatches (default behavior).
	HooksSubscriptionEnabled bool `koanf:"hooks_subscription_enabled"`

	// HooksVPNProviderEnabled enables plugin-driven VPN provisioning.
	// When false, the hardcoded Remnawave adapter is used directly.
	HooksVPNProviderEnabled bool `koanf:"hooks_vpn_provider_enabled"`
}

// CORSConfig holds the Cross-Origin Resource Sharing configuration.
type CORSConfig struct {
	AllowedOrigins []string `koanf:"allowed_origins"`
}

// CircuitBreakerConfig holds per-component circuit breaker settings.
// All fields default to circuitbreaker.DefaultConfig() values when
// unset, preserving backward compatibility with existing deployments.
type CircuitBreakerConfig struct {
	Remnawave   circuitbreaker.Config `koanf:"remnawave"`
	OutboxNATS  circuitbreaker.Config `koanf:"outbox_nats"`
	Valkey      circuitbreaker.Config `koanf:"valkey"`
	VPNProvider circuitbreaker.Config `koanf:"vpn_provider"`
}

type Config struct {
	App            AppConfig            `koanf:"app"`
	Database       DatabaseConfig       `koanf:"database"`
	Valkey         ValkeyConfig         `koanf:"valkey"`
	NATS           NATSConfig           `koanf:"nats"`
	JWT            JWTConfig            `koanf:"jwt"`
	Remnawave      RemnawaveConfig      `koanf:"remnawave"`
	Billing        BillingConfig        `koanf:"billing"`
	Plugin         PluginConfig         `koanf:"plugin"`
	Telegram       TelegramConfig       `koanf:"telegram"`
	Security       SecurityConfig       `koanf:"security"`
	Infra          InfraConfig          `koanf:"infra"`
	Outbox         OutboxConfig         `koanf:"outbox"`
	CORS           CORSConfig           `koanf:"cors"`
	Tracing        TracingConfig        `koanf:"tracing"`
	RateLimit      RateLimitConfig      `koanf:"ratelimit"`
	FeatureFlags   FeatureFlags         `koanf:"featureflags"`
	CircuitBreaker CircuitBreakerConfig `koanf:"circuitbreaker"`
	SmartRouter    SmartRouterConfig    `koanf:"smartrouter"`
	SpeedTest      SpeedTestConfig      `koanf:"speedtest"`
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
	{envVar: "JWT_PRIVATE_KEY_PATH", koanfKey: "jwt.private_key_path"},
	{envVar: "JWT_PUBLIC_KEY_PATH", koanfKey: "jwt.public_key_path"},
}

// Load reads configuration from environment variables and returns a validated
// Config. Required fields that are empty cause an error.
func Load() (*Config, error) {
	k := koanf.New(".")

	// Set defaults
	defaults := map[string]any{
		"app.port":                                   DefaultAppPort,
		"app.version":                                DefaultAppVersion,
		"app.log_level":                              DefaultLogLevel,
		"app.log_format":                             DefaultLogFormat,
		"database.max_conns":                         DefaultPoolMaxConns,
		"database.min_conns":                         DefaultPoolMinConns,
		"database.max_conn_lifetime":                 DefaultPoolMaxConnLifetime,
		"database.max_conn_idle_time":                DefaultPoolMaxConnIdleTime,
		"database.health_check_period":               DefaultPoolHealthCheck,
		"jwt.access_token_ttl":                       DefaultJWTAccessTTL,
		"jwt.refresh_token_ttl":                      DefaultJWTRefreshTTL,
		"billing.trial_days":                         DefaultBillingTrialDays,
		"plugin.dir":                                 DefaultPluginsDir,
		"plugin.max_plugins":                         DefaultMaxPlugins,
		"plugin.hot_reload":                          DefaultPluginHotReload,
		"infra.health_check_interval":                time.Duration(DefaultHealthCheckInterval) * time.Second,
		"infra.max_concurrent_checks":                DefaultMaxConcurrentChecks,
		"infra.speed_test_port":                      DefaultSpeedTestPort,
		"infra.subscription_proxy_port":              DefaultSubscriptionProxyPort,
		"ratelimit.checkout_max_per_hour":            DefaultCheckoutMaxPerHour,
		"ratelimit.subscription_max_per_day":         DefaultSubscriptionMaxPerDay,
		"ratelimit.login_max_per_window":             DefaultLoginMaxPerWindow,
		"ratelimit.login_window_minutes":             DefaultLoginWindowMinutes,
		"ratelimit.forgot_pwd_max_per_window":        DefaultForgotPwdMaxPerWindow,
		"ratelimit.forgot_pwd_window_minutes":        DefaultForgotPwdWindowMinutes,
		"ratelimit.accept_invitation_max_per_window": DefaultAcceptInvitationMaxPerWindow,
		"ratelimit.accept_invitation_window_minutes": DefaultAcceptInvitationWindowMinutes,
		"outbox.relay_workers":                       DefaultOutboxRelayWorkers,
		"outbox.partition_lookahead":                 DefaultOutboxPartitionLookahead,
		"outbox.retention_days":                      DefaultOutboxRetentionDays,
		"featureflags.hooks_subscription_enabled":    DefaultHooksSubscriptionEnabled,
		"featureflags.hooks_vpn_provider_enabled":    DefaultHooksVPNProviderEnabled,
		// Smart router weight defaults.
		"smartrouter.weight_geo":               DefaultWeightGeo,
		"smartrouter.weight_latency":           DefaultWeightLatency,
		"smartrouter.weight_load":              DefaultWeightLoad,
		"smartrouter.weight_gaming_geo":        DefaultWeightGamingGeo,
		"smartrouter.weight_gaming_latency":    DefaultWeightGamingLatency,
		"smartrouter.weight_gaming_load":       DefaultWeightGamingLoad,
		"smartrouter.weight_streaming_geo":     DefaultWeightStreamingGeo,
		"smartrouter.weight_streaming_latency": DefaultWeightStreamingLatency,
		"smartrouter.weight_streaming_load":    DefaultWeightStreamingLoad,
		// Speed test defaults.
		"speedtest.max_concurrent":    DefaultSpeedTestMaxConcurrent,
		"speedtest.per_ip_rate_limit": DefaultSpeedTestPerIPRateLimit,
		"speedtest.max_upload_bytes":  DefaultSpeedTestMaxUploadBytes,
		// Circuit breaker defaults — Remnawave and VPN provider use DefaultConfig
		// (with interval), outbox relay and Valkey use DefaultConfigNoInterval.
		"circuitbreaker.remnawave.max_failures":    circuitbreaker.DefaultMaxFailures,
		"circuitbreaker.remnawave.timeout":         circuitbreaker.DefaultTimeout,
		"circuitbreaker.remnawave.max_requests":    circuitbreaker.DefaultMaxRequests,
		"circuitbreaker.remnawave.interval":        circuitbreaker.DefaultInterval,
		"circuitbreaker.outbox_nats.max_failures":  circuitbreaker.DefaultMaxFailures,
		"circuitbreaker.outbox_nats.timeout":       circuitbreaker.DefaultTimeout,
		"circuitbreaker.outbox_nats.max_requests":  uint32(1),
		"circuitbreaker.outbox_nats.interval":      time.Duration(0),
		"circuitbreaker.valkey.max_failures":       circuitbreaker.DefaultMaxFailures,
		"circuitbreaker.valkey.timeout":            circuitbreaker.DefaultTimeout,
		"circuitbreaker.valkey.max_requests":       circuitbreaker.DefaultMaxRequests,
		"circuitbreaker.valkey.interval":           time.Duration(0),
		"circuitbreaker.vpn_provider.max_failures": circuitbreaker.DefaultMaxFailures,
		"circuitbreaker.vpn_provider.timeout":      circuitbreaker.DefaultTimeout,
		"circuitbreaker.vpn_provider.max_requests": circuitbreaker.DefaultMaxRequests,
		"circuitbreaker.vpn_provider.interval":     circuitbreaker.DefaultInterval,
	}
	for key, val := range defaults {
		k.Set(key, val) //nolint:errcheck // Set on a fresh koanf instance cannot fail
	}

	// Load each prefix group from environment variables.
	prefixes := []string{"APP_", "DATABASE_", "VALKEY_", "NATS_", "JWT_", "BILLING_", "PLUGIN_", "TELEGRAM_", "SECURITY_", "INFRA_", "OUTBOX_", "CORS_", "TRACING_", "RATELIMIT_", "FEATUREFLAGS_", "CIRCUITBREAKER_", "SMARTROUTER_", "SPEEDTEST_"}
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
