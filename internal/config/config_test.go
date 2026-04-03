package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
)

// requiredEnv sets the minimum required environment variables and returns a
// cleanup function that unsets them all.
func setRequiredEnv(t *testing.T) {
	t.Helper()
	required := map[string]string{
		"DATABASE_URL":         "postgres://localhost:5432/test",
		"VALKEY_URL":           "redis://localhost:6379",
		"NATS_URL":             "nats://localhost:4222",
		"REMNAWAVE_URL":        "https://api.remnawave.example.com",
		"REMNAWAVE_API_TOKEN":  "token-abc-123",
		"JWT_PRIVATE_KEY_PATH": "/etc/keys/private.pem",
		"JWT_PUBLIC_KEY_PATH":  "/etc/keys/public.pem",
	}
	for k, v := range required {
		t.Setenv(k, v)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := config.Load()
	require.NoError(t, err)

	// App defaults
	assert.Equal(t, config.DefaultAppPort, cfg.App.Port)
	assert.Equal(t, config.DefaultLogLevel, cfg.App.LogLevel)
	assert.Equal(t, config.DefaultLogFormat, cfg.App.LogFormat)

	// Database defaults
	assert.Equal(t, "postgres://localhost:5432/test", cfg.Database.URL)
	assert.Equal(t, config.DefaultDBMaxOpenConns, cfg.Database.MaxOpenConns)
	assert.Equal(t, config.DefaultDBMaxIdleConns, cfg.Database.MaxIdleConns)
	assert.Equal(t, config.DefaultDBConnMaxLifetime, cfg.Database.ConnMaxLifetime)

	// JWT defaults
	assert.Equal(t, config.DefaultJWTAccessTTL, cfg.JWT.AccessTokenTTL)
	assert.Equal(t, config.DefaultJWTRefreshTTL, cfg.JWT.RefreshTokenTTL)
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	requiredVars := []string{
		"DATABASE_URL",
		"VALKEY_URL",
		"NATS_URL",
		"REMNAWAVE_URL",
		"REMNAWAVE_API_TOKEN",
		"JWT_PRIVATE_KEY_PATH",
		"JWT_PUBLIC_KEY_PATH",
	}

	for _, envVar := range requiredVars {
		t.Run(envVar, func(t *testing.T) {
			// Set all required vars first
			setRequiredEnv(t)
			// Then unset the one we're testing
			t.Setenv(envVar, "")

			_, err := config.Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), envVar)
		})
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	setRequiredEnv(t)

	// Override defaults with custom values
	t.Setenv("APP_PORT", "8080")
	t.Setenv("APP_LOG_LEVEL", "info")
	t.Setenv("APP_LOG_FORMAT", "text")
	t.Setenv("DATABASE_MAX_OPEN_CONNS", "50")
	t.Setenv("DATABASE_MAX_IDLE_CONNS", "10")
	t.Setenv("DATABASE_CONN_MAX_LIFETIME", "10m")
	t.Setenv("JWT_ACCESS_TOKEN_TTL", "30m")
	t.Setenv("JWT_REFRESH_TOKEN_TTL", "72h")
	t.Setenv("REMNAWAVE_WEBHOOK_SECRET", "webhook-secret-xyz")

	cfg, err := config.Load()
	require.NoError(t, err)

	// App
	assert.Equal(t, 8080, cfg.App.Port)
	assert.Equal(t, "info", cfg.App.LogLevel)
	assert.Equal(t, "text", cfg.App.LogFormat)

	// Database
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 10, cfg.Database.MaxIdleConns)
	assert.Equal(t, 10*time.Minute, cfg.Database.ConnMaxLifetime)

	// JWT
	assert.Equal(t, 30*time.Minute, cfg.JWT.AccessTokenTTL)
	assert.Equal(t, 72*time.Hour, cfg.JWT.RefreshTokenTTL)

	// Remnawave
	assert.Equal(t, "https://api.remnawave.example.com", cfg.Remnawave.URL)
	assert.Equal(t, "token-abc-123", cfg.Remnawave.APIToken)
	assert.Equal(t, "webhook-secret-xyz", cfg.Remnawave.WebhookSecret)

	// Valkey & NATS
	assert.Equal(t, "redis://localhost:6379", cfg.Valkey.URL)
	assert.Equal(t, "nats://localhost:4222", cfg.NATS.URL)
}
