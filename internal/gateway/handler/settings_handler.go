package handler

import (
	"net/http"
	"strings"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

// secretMask is the replacement string for fully-redacted secret values.
const secretMask = "****"

// SettingsHandler serves the admin settings endpoint.
type SettingsHandler struct {
	cfg *config.Config
}

// NewSettingsHandler creates a SettingsHandler backed by the given config.
func NewSettingsHandler(cfg *config.Config) *SettingsHandler {
	return &SettingsHandler{cfg: cfg}
}

// --- Response DTOs ---

// SettingsResponse organises the system configuration by category.
type SettingsResponse struct {
	App            AppSettings            `json:"app"`
	Database       DatabaseSettings       `json:"database"`
	Cache          CacheSettings          `json:"cache"`
	MessageQueue   MessageQueueSettings   `json:"message_queue"`
	JWT            JWTSettings            `json:"jwt"`
	Remnawave      RemnawaveSettings      `json:"remnawave"`
	Billing        BillingSettings        `json:"billing"`
	Telegram       TelegramSettings       `json:"telegram"`
	Plugins        PluginSettings         `json:"plugins"`
	Infrastructure InfraSettings          `json:"infrastructure"`
	SpeedTest      SpeedTestSettings      `json:"speed_test"`
	RateLimit      RateLimitSettings      `json:"rate_limit"`
	Outbox         OutboxSettings         `json:"outbox"`
	SmartRouter    SmartRouterSettings    `json:"smart_router"`
	FeatureFlags   FeatureFlagSettings    `json:"feature_flags"`
	CircuitBreaker CircuitBreakerSettings `json:"circuit_breaker"`
	CORS           CORSSettings           `json:"cors"`
	Tracing        TracingSettings        `json:"tracing"`
}

// AppSettings mirrors config.AppConfig.
type AppSettings struct {
	Port      int    `json:"port"`
	Version   string `json:"version"`
	LogLevel  string `json:"log_level"`
	LogFormat string `json:"log_format"`
}

// DatabaseSettings mirrors config.DatabaseConfig with URL credentials masked.
type DatabaseSettings struct {
	URL               string `json:"url"`
	MaxConns          int32  `json:"max_conns"`
	MinConns          int32  `json:"min_conns"`
	MaxConnLifetime   string `json:"max_conn_lifetime"`
	MaxConnIdleTime   string `json:"max_conn_idle_time"`
	HealthCheckPeriod string `json:"health_check_period"`
}

// CacheSettings mirrors config.ValkeyConfig with URL credentials masked.
type CacheSettings struct {
	URL string `json:"url"`
}

// MessageQueueSettings mirrors config.NATSConfig with URL credentials masked.
type MessageQueueSettings struct {
	URL string `json:"url"`
}

// JWTSettings mirrors config.JWTConfig. Key paths are not secrets.
type JWTSettings struct {
	PrivateKeyPath  string `json:"private_key_path"`
	PublicKeyPath   string `json:"public_key_path"`
	AccessTokenTTL  string `json:"access_token_ttl"`
	RefreshTokenTTL string `json:"refresh_token_ttl"`
}

// RemnawaveSettings mirrors config.RemnawaveConfig with tokens masked.
type RemnawaveSettings struct {
	URL           string `json:"url"`
	APIToken      string `json:"api_token"`
	WebhookSecret string `json:"webhook_secret"`
}

// BillingSettings mirrors config.BillingConfig.
type BillingSettings struct {
	TrialDays int `json:"trial_days"`
}

// TelegramSettings mirrors config.TelegramConfig with bot token masked.
type TelegramSettings struct {
	BotToken   string `json:"bot_token"`
	WebhookURL string `json:"webhook_url"`
	CabinetURL string `json:"cabinet_url"`
}

// PluginSettings mirrors config.PluginConfig.
type PluginSettings struct {
	PluginsDir      string `json:"plugins_dir"`
	MaxPlugins      int    `json:"max_plugins"`
	EnableHotReload bool   `json:"enable_hot_reload"`
}

// InfraSettings mirrors config.InfraConfig.
type InfraSettings struct {
	HealthCheckInterval   string `json:"health_check_interval"`
	MaxConcurrentChecks   int    `json:"max_concurrent_checks"`
	SpeedTestPort         int    `json:"speed_test_port"`
	SubscriptionProxyPort int    `json:"subscription_proxy_port"`
}

// SpeedTestSettings mirrors config.SpeedTestConfig.
type SpeedTestSettings struct {
	MaxConcurrent  int `json:"max_concurrent"`
	PerIPRateLimit int `json:"per_ip_rate_limit"`
	MaxUploadBytes int `json:"max_upload_bytes"`
}

// RateLimitSettings mirrors config.RateLimitConfig.
type RateLimitSettings struct {
	CheckoutMaxPerHour     int `json:"checkout_max_per_hour"`
	SubscriptionMaxPerDay  int `json:"subscription_max_per_day"`
	LoginMaxPerWindow      int `json:"login_max_per_window"`
	LoginWindowMinutes     int `json:"login_window_minutes"`
	ForgotPwdMaxPerWindow  int `json:"forgot_pwd_max_per_window"`
	ForgotPwdWindowMinutes int `json:"forgot_pwd_window_minutes"`
}

// OutboxSettings mirrors config.OutboxConfig.
type OutboxSettings struct {
	RelayWorkers       int `json:"relay_workers"`
	PartitionLookahead int `json:"partition_lookahead"`
	RetentionDays      int `json:"retention_days"`
}

// SmartRouterSettings mirrors config.SmartRouterConfig.
type SmartRouterSettings struct {
	WeightGeo     float64 `json:"weight_geo"`
	WeightLatency float64 `json:"weight_latency"`
	WeightLoad    float64 `json:"weight_load"`

	WeightGamingGeo     float64 `json:"weight_gaming_geo"`
	WeightGamingLatency float64 `json:"weight_gaming_latency"`
	WeightGamingLoad    float64 `json:"weight_gaming_load"`

	WeightStreamingGeo     float64 `json:"weight_streaming_geo"`
	WeightStreamingLatency float64 `json:"weight_streaming_latency"`
	WeightStreamingLoad    float64 `json:"weight_streaming_load"`
}

// FeatureFlagSettings mirrors config.FeatureFlags.
type FeatureFlagSettings struct {
	HooksSubscriptionEnabled bool `json:"hooks_subscription_enabled"`
	HooksVPNProviderEnabled  bool `json:"hooks_vpn_provider_enabled"`
}

// CircuitBreakerComponentSettings mirrors circuitbreaker.Config for JSON output.
type CircuitBreakerComponentSettings struct {
	MaxFailures uint32 `json:"max_failures"`
	Timeout     string `json:"timeout"`
	MaxRequests uint32 `json:"max_requests"`
	Interval    string `json:"interval"`
}

// CircuitBreakerSettings mirrors config.CircuitBreakerConfig.
type CircuitBreakerSettings struct {
	Remnawave   CircuitBreakerComponentSettings `json:"remnawave"`
	OutboxNATS  CircuitBreakerComponentSettings `json:"outbox_nats"`
	Valkey      CircuitBreakerComponentSettings `json:"valkey"`
	VPNProvider CircuitBreakerComponentSettings `json:"vpn_provider"`
}

// CORSSettings mirrors config.CORSConfig.
type CORSSettings struct {
	AllowedOrigins []string `json:"allowed_origins"`
}

// TracingSettings mirrors config.TracingConfig.
type TracingSettings struct {
	Endpoint string `json:"endpoint"`
}

// --- Handler ---

// GetSettings handles GET /api/admin/settings -- returns the current system
// configuration with all sensitive fields masked.
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, _ *http.Request) {
	resp := SettingsResponse{
		App: AppSettings{
			Port:      h.cfg.App.Port,
			Version:   h.cfg.App.Version,
			LogLevel:  h.cfg.App.LogLevel,
			LogFormat: h.cfg.App.LogFormat,
		},
		Database: DatabaseSettings{
			URL:               maskConnectionString(h.cfg.Database.URL),
			MaxConns:          h.cfg.Database.MaxConns,
			MinConns:          h.cfg.Database.MinConns,
			MaxConnLifetime:   h.cfg.Database.MaxConnLifetime.String(),
			MaxConnIdleTime:   h.cfg.Database.MaxConnIdleTime.String(),
			HealthCheckPeriod: h.cfg.Database.HealthCheckPeriod.String(),
		},
		Cache: CacheSettings{
			URL: maskConnectionString(h.cfg.Valkey.URL),
		},
		MessageQueue: MessageQueueSettings{
			URL: maskConnectionString(h.cfg.NATS.URL),
		},
		JWT: JWTSettings{
			PrivateKeyPath:  h.cfg.JWT.PrivateKeyPath,
			PublicKeyPath:   h.cfg.JWT.PublicKeyPath,
			AccessTokenTTL:  h.cfg.JWT.AccessTokenTTL.String(),
			RefreshTokenTTL: h.cfg.JWT.RefreshTokenTTL.String(),
		},
		Remnawave: RemnawaveSettings{
			URL:           h.cfg.Remnawave.URL,
			APIToken:      secretMask,
			WebhookSecret: secretMask,
		},
		Billing: BillingSettings{
			TrialDays: h.cfg.Billing.TrialDays,
		},
		Telegram: TelegramSettings{
			BotToken:   maskOptionalSecret(h.cfg.Telegram.BotToken),
			WebhookURL: h.cfg.Telegram.WebhookURL,
			CabinetURL: h.cfg.Telegram.CabinetURL,
		},
		Plugins: PluginSettings{
			PluginsDir:      h.cfg.Plugin.PluginsDir,
			MaxPlugins:      h.cfg.Plugin.MaxPlugins,
			EnableHotReload: h.cfg.Plugin.EnableHotReload,
		},
		Infrastructure: InfraSettings{
			HealthCheckInterval:   h.cfg.Infra.HealthCheckInterval.String(),
			MaxConcurrentChecks:   h.cfg.Infra.MaxConcurrentChecks,
			SpeedTestPort:         h.cfg.Infra.SpeedTestPort,
			SubscriptionProxyPort: h.cfg.Infra.SubscriptionProxyPort,
		},
		SpeedTest: SpeedTestSettings{
			MaxConcurrent:  h.cfg.SpeedTest.MaxConcurrent,
			PerIPRateLimit: h.cfg.SpeedTest.PerIPRateLimit,
			MaxUploadBytes: h.cfg.SpeedTest.MaxUploadBytes,
		},
		RateLimit: RateLimitSettings{
			CheckoutMaxPerHour:     h.cfg.RateLimit.CheckoutMaxPerHour,
			SubscriptionMaxPerDay:  h.cfg.RateLimit.SubscriptionMaxPerDay,
			LoginMaxPerWindow:      h.cfg.RateLimit.LoginMaxPerWindow,
			LoginWindowMinutes:     h.cfg.RateLimit.LoginWindowMinutes,
			ForgotPwdMaxPerWindow:  h.cfg.RateLimit.ForgotPwdMaxPerWindow,
			ForgotPwdWindowMinutes: h.cfg.RateLimit.ForgotPwdWindowMinutes,
		},
		Outbox: OutboxSettings{
			RelayWorkers:       h.cfg.Outbox.RelayWorkers,
			PartitionLookahead: h.cfg.Outbox.PartitionLookahead,
			RetentionDays:      h.cfg.Outbox.RetentionDays,
		},
		SmartRouter: SmartRouterSettings{
			WeightGeo:              h.cfg.SmartRouter.WeightGeo,
			WeightLatency:          h.cfg.SmartRouter.WeightLatency,
			WeightLoad:             h.cfg.SmartRouter.WeightLoad,
			WeightGamingGeo:        h.cfg.SmartRouter.WeightGamingGeo,
			WeightGamingLatency:    h.cfg.SmartRouter.WeightGamingLatency,
			WeightGamingLoad:       h.cfg.SmartRouter.WeightGamingLoad,
			WeightStreamingGeo:     h.cfg.SmartRouter.WeightStreamingGeo,
			WeightStreamingLatency: h.cfg.SmartRouter.WeightStreamingLatency,
			WeightStreamingLoad:    h.cfg.SmartRouter.WeightStreamingLoad,
		},
		FeatureFlags: FeatureFlagSettings{
			HooksSubscriptionEnabled: h.cfg.FeatureFlags.HooksSubscriptionEnabled,
			HooksVPNProviderEnabled:  h.cfg.FeatureFlags.HooksVPNProviderEnabled,
		},
		CircuitBreaker: CircuitBreakerSettings{
			Remnawave:   mapCBConfig(h.cfg.CircuitBreaker.Remnawave),
			OutboxNATS:  mapCBConfig(h.cfg.CircuitBreaker.OutboxNATS),
			Valkey:      mapCBConfig(h.cfg.CircuitBreaker.Valkey),
			VPNProvider: mapCBConfig(h.cfg.CircuitBreaker.VPNProvider),
		},
		CORS: CORSSettings{
			AllowedOrigins: corsOrigins(h.cfg.CORS.AllowedOrigins),
		},
		Tracing: TracingSettings{
			Endpoint: h.cfg.Tracing.Endpoint,
		},
	}

	writeJSON(w, http.StatusOK, resp)
}

// maskConnectionString replaces user credentials in a connection string URI.
// Input:  "postgres://user:p@ss@host:5432/db"
// Output: "postgres://****@host:5432/db"
// Uses strings.LastIndex("@") to handle passwords containing "@" safely.
func maskConnectionString(raw string) string {
	if raw == "" {
		return ""
	}
	schemeEnd := strings.Index(raw, "://")
	if schemeEnd < 0 {
		return raw
	}
	atIdx := strings.LastIndex(raw, "@")
	if atIdx < 0 || atIdx <= schemeEnd+3 {
		return raw
	}
	return raw[:schemeEnd+3] + secretMask + raw[atIdx:]
}

// maskOptionalSecret returns secretMask when the SecretString contains a
// non-empty value, or an empty string when it is unset.
func maskOptionalSecret(s config.SecretString) string {
	if s.Expose() != "" {
		return secretMask
	}
	return ""
}

// corsOrigins returns a non-nil slice to avoid JSON null.
func corsOrigins(origins []string) []string {
	if origins == nil {
		return []string{}
	}
	return origins
}

// mapCBConfig converts a circuitbreaker.Config to the JSON-safe response struct.
func mapCBConfig(cfg circuitbreaker.Config) CircuitBreakerComponentSettings {
	return CircuitBreakerComponentSettings{
		MaxFailures: cfg.MaxFailures,
		Timeout:     cfg.Timeout.String(),
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval.String(),
	}
}
