package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

// testConfig returns a fully populated Config for tests. All secret fields
// contain recognisable values so tests can verify they are masked.
func testConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Port:      4000,
			Version:   "v1.2.3",
			LogLevel:  "info",
			LogFormat: "json",
		},
		Database: config.DatabaseConfig{
			URL:               "postgres://admin:supersecret@db.example.com:5432/remna",
			MaxConns:          20,
			MinConns:          5,
			MaxConnLifetime:   time.Hour,
			MaxConnIdleTime:   30 * time.Minute,
			HealthCheckPeriod: time.Minute,
		},
		Valkey: config.ValkeyConfig{
			URL: "redis://default:valkeypass@valkey.example.com:6379",
		},
		NATS: config.NATSConfig{
			URL: "nats://natsuser:natspass@nats.example.com:4222",
		},
		JWT: config.JWTConfig{
			PrivateKeyPath:  "/etc/remna/jwt.key",
			PublicKeyPath:   "/etc/remna/jwt.pub",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 7 * 24 * time.Hour,
		},
		Remnawave: config.RemnawaveConfig{
			URL:           "https://panel.example.com",
			APIToken:      config.NewSecretString("rw-token-secret"),
			WebhookSecret: config.NewSecretString("rw-webhook-secret"),
		},
		Billing: config.BillingConfig{
			TrialDays: 7,
		},
		Plugin: config.PluginConfig{
			PluginsDir:      "./plugins",
			MaxPlugins:      50,
			EnableHotReload: true,
		},
		Telegram: config.TelegramConfig{
			BotToken:   config.NewSecretString("123456:ABC-DEF"),
			WebhookURL: "https://api.example.com/webhooks/telegram",
			CabinetURL: "https://cabinet.example.com",
		},
		Infra: config.InfraConfig{
			HealthCheckInterval:   10 * time.Second,
			MaxConcurrentChecks:   50,
			SpeedTestPort:         4203,
			SubscriptionProxyPort: 4100,
		},
		SpeedTest: config.SpeedTestConfig{
			MaxConcurrent:  10,
			PerIPRateLimit: 3,
			MaxUploadBytes: 10 * 1024 * 1024,
		},
		RateLimit: config.RateLimitConfig{
			CheckoutMaxPerHour:     10,
			SubscriptionMaxPerDay:  5,
			LoginMaxPerWindow:      20,
			LoginWindowMinutes:     15,
			ForgotPwdMaxPerWindow:  3,
			ForgotPwdWindowMinutes: 60,
		},
		Outbox: config.OutboxConfig{
			RelayWorkers:       1,
			PartitionLookahead: 2,
			RetentionDays:      90,
		},
		SmartRouter: config.SmartRouterConfig{
			WeightGeo:              0.33,
			WeightLatency:          0.34,
			WeightLoad:             0.33,
			WeightGamingGeo:        0.2,
			WeightGamingLatency:    0.6,
			WeightGamingLoad:       0.2,
			WeightStreamingGeo:     0.5,
			WeightStreamingLatency: 0.2,
			WeightStreamingLoad:    0.3,
		},
		FeatureFlags: config.FeatureFlags{
			HooksSubscriptionEnabled: true,
			HooksVPNProviderEnabled:  false,
		},
		CircuitBreaker: config.CircuitBreakerConfig{
			Remnawave:   circuitbreaker.DefaultConfig(),
			OutboxNATS:  circuitbreaker.DefaultConfigNoInterval(),
			Valkey:      circuitbreaker.DefaultConfigNoInterval(),
			VPNProvider: circuitbreaker.DefaultConfig(),
		},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"https://app.example.com"},
		},
		Tracing: config.TracingConfig{
			Endpoint: "localhost:4318",
		},
	}
}

func TestGetSettings_ReturnsOK(t *testing.T) {
	h := NewSettingsHandler(testConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
	rec := httptest.NewRecorder()

	h.GetSettings(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp SettingsResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	// Non-secret fields are passed through.
	assert.Equal(t, 4000, resp.App.Port)
	assert.Equal(t, "v1.2.3", resp.App.Version)
	assert.Equal(t, 7, resp.Billing.TrialDays)
	assert.Equal(t, "/etc/remna/jwt.key", resp.JWT.PrivateKeyPath)
	assert.Equal(t, "/etc/remna/jwt.pub", resp.JWT.PublicKeyPath)
	assert.Equal(t, "https://panel.example.com", resp.Remnawave.URL)
}

func TestGetSettings_MasksSecrets(t *testing.T) {
	h := NewSettingsHandler(testConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
	rec := httptest.NewRecorder()

	h.GetSettings(rec, req)

	var resp SettingsResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	// Fully masked secrets.
	assert.Equal(t, secretMask, resp.Remnawave.APIToken)
	assert.Equal(t, secretMask, resp.Remnawave.WebhookSecret)
	assert.Equal(t, secretMask, resp.Telegram.BotToken)

	// Connection string credentials masked, host preserved.
	assert.NotContains(t, resp.Database.URL, "supersecret")
	assert.Contains(t, resp.Database.URL, "db.example.com:5432")
	assert.Contains(t, resp.Database.URL, "postgres://")

	assert.NotContains(t, resp.Cache.URL, "valkeypass")
	assert.Contains(t, resp.Cache.URL, "valkey.example.com:6379")

	assert.NotContains(t, resp.MessageQueue.URL, "natspass")
	assert.Contains(t, resp.MessageQueue.URL, "nats.example.com:4222")
}

func TestMaskConnectionString(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "postgres with user and password",
			raw:  "postgres://admin:supersecret@db.example.com:5432/remna",
			want: "postgres://****@db.example.com:5432/remna",
		},
		{
			name: "redis with default user",
			raw:  "redis://default:pass123@valkey.example.com:6379",
			want: "redis://****@valkey.example.com:6379",
		},
		{
			name: "nats with credentials",
			raw:  "nats://user:pwd@nats.example.com:4222",
			want: "nats://****@nats.example.com:4222",
		},
		{
			name: "url without credentials",
			raw:  "nats://nats.example.com:4222",
			want: "nats://nats.example.com:4222",
		},
		{
			name: "empty string",
			raw:  "",
			want: "",
		},
		{
			name: "user only no password",
			raw:  "postgres://admin@db.example.com:5432/remna",
			want: "postgres://****@db.example.com:5432/remna",
		},
		{
			name: "password containing @ symbol",
			raw:  "postgres://user:p@ss@host:5432/db",
			want: "postgres://****@host:5432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskConnectionString(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMaskOptionalSecret(t *testing.T) {
	tests := []struct {
		name string
		val  config.SecretString
		want string
	}{
		{
			name: "non-empty secret is masked",
			val:  config.NewSecretString("my-token"),
			want: secretMask,
		},
		{
			name: "empty secret returns empty",
			val:  config.NewSecretString(""),
			want: "",
		},
		{
			name: "zero value returns empty",
			val:  config.SecretString{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskOptionalSecret(tt.val)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetSettings_NeverLeaksRawSecrets(t *testing.T) {
	h := NewSettingsHandler(testConfig())

	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
	rec := httptest.NewRecorder()

	h.GetSettings(rec, req)

	body := rec.Body.String()

	// None of the raw secret values should appear anywhere in the response.
	assert.NotContains(t, body, "supersecret")
	assert.NotContains(t, body, "valkeypass")
	assert.NotContains(t, body, "natspass")
	assert.NotContains(t, body, "rw-token-secret")
	assert.NotContains(t, body, "rw-webhook-secret")
	assert.NotContains(t, body, "123456:ABC-DEF")
}

func TestGetSettings_TelegramBotTokenEmptyNotMasked(t *testing.T) {
	cfg := testConfig()
	cfg.Telegram.BotToken = config.NewSecretString("")

	h := NewSettingsHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/settings", nil)
	rec := httptest.NewRecorder()

	h.GetSettings(rec, req)

	var resp SettingsResponse
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "", resp.Telegram.BotToken)
}
