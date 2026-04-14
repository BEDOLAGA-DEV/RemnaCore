package settings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	domainsettings "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/settings"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

// testConfig returns a baseline config for testing.
func testConfig() *config.Config {
	return &config.Config{
		Billing: config.BillingConfig{TrialDays: 7},
		RateLimit: config.RateLimitConfig{
			CheckoutMaxPerHour:     10,
			SubscriptionMaxPerDay:  5,
			LoginMaxPerWindow:      20,
			LoginWindowMinutes:     15,
			ForgotPwdMaxPerWindow:  3,
			ForgotPwdWindowMinutes: 60,
		},
		SmartRouter: config.SmartRouterConfig{
			WeightGeo:     0.33,
			WeightLatency: 0.34,
			WeightLoad:    0.33,
		},
		SpeedTest: config.SpeedTestConfig{
			MaxConcurrent:  10,
			PerIPRateLimit: 3,
			MaxUploadBytes: 10 * 1024 * 1024,
		},
		Plugin: config.PluginConfig{
			PluginsDir:      "./plugins",
			MaxPlugins:      50,
			EnableHotReload: false,
		},
		Outbox: config.OutboxConfig{
			RelayWorkers:       1,
			PartitionLookahead: 2,
			RetentionDays:      90,
		},
		FeatureFlags: config.FeatureFlags{
			HooksSubscriptionEnabled: false,
			HooksVPNProviderEnabled:  false,
		},
		CORS: config.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
		},
		CircuitBreaker: config.CircuitBreakerConfig{
			Remnawave:   circuitbreaker.DefaultConfig(),
			OutboxNATS:  circuitbreaker.DefaultConfigNoInterval(),
			Valkey:      circuitbreaker.DefaultConfigNoInterval(),
			VPNProvider: circuitbreaker.DefaultConfig(),
		},
	}
}

func intPtr(v int) *int             { return &v }
func boolPtr(v bool) *bool          { return &v }
func float64Ptr(v float64) *float64 { return &v }
func uint32Ptr(v uint32) *uint32    { return &v }
func durPtr(v time.Duration) *time.Duration { return &v }

func TestApplySettingsUpdate_Billing(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)
	update := &domainsettings.SettingsUpdate{
		Billing: &domainsettings.BillingUpdate{TrialDays: intPtr(30)},
	}

	err := applier.ApplySettingsUpdate(update)

	assert.NoError(t, err)
	assert.Equal(t, 30, cfg.Billing.TrialDays)
}

func TestApplySettingsUpdate_RateLimit(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)
	update := &domainsettings.SettingsUpdate{
		RateLimit: &domainsettings.RateLimitUpdate{
			CheckoutMaxPerHour: intPtr(50),
			LoginWindowMinutes: intPtr(30),
		},
	}

	err := applier.ApplySettingsUpdate(update)

	assert.NoError(t, err)
	assert.Equal(t, 50, cfg.RateLimit.CheckoutMaxPerHour)
	assert.Equal(t, 30, cfg.RateLimit.LoginWindowMinutes)
	// Unchanged fields stay at original values.
	assert.Equal(t, 5, cfg.RateLimit.SubscriptionMaxPerDay)
	assert.Equal(t, 20, cfg.RateLimit.LoginMaxPerWindow)
}

func TestApplySettingsUpdate_FeatureFlags(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)
	update := &domainsettings.SettingsUpdate{
		FeatureFlags: &domainsettings.FeatureFlagUpdate{
			HooksSubscriptionEnabled: boolPtr(true),
		},
	}

	err := applier.ApplySettingsUpdate(update)

	assert.NoError(t, err)
	assert.True(t, cfg.FeatureFlags.HooksSubscriptionEnabled)
	// Unchanged flag stays at original.
	assert.False(t, cfg.FeatureFlags.HooksVPNProviderEnabled)
}

func TestApplySettingsUpdate_SmartRouter(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)
	update := &domainsettings.SettingsUpdate{
		SmartRouter: &domainsettings.SmartRouterUpdate{
			WeightGeo: float64Ptr(0.5),
		},
	}

	err := applier.ApplySettingsUpdate(update)

	assert.NoError(t, err)
	assert.InDelta(t, 0.5, cfg.SmartRouter.WeightGeo, 0.001)
	// Unchanged.
	assert.InDelta(t, 0.34, cfg.SmartRouter.WeightLatency, 0.001)
}

func TestApplySettingsUpdate_SpeedTest(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)
	update := &domainsettings.SettingsUpdate{
		SpeedTest: &domainsettings.SpeedTestUpdate{
			MaxConcurrent: intPtr(20),
		},
	}

	err := applier.ApplySettingsUpdate(update)

	assert.NoError(t, err)
	assert.Equal(t, 20, cfg.SpeedTest.MaxConcurrent)
	assert.Equal(t, 3, cfg.SpeedTest.PerIPRateLimit) // unchanged
}

func TestApplySettingsUpdate_Plugins(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)
	update := &domainsettings.SettingsUpdate{
		Plugins: &domainsettings.PluginUpdate{
			EnableHotReload: boolPtr(true),
		},
	}

	err := applier.ApplySettingsUpdate(update)

	assert.NoError(t, err)
	assert.True(t, cfg.Plugin.EnableHotReload)
	assert.Equal(t, 50, cfg.Plugin.MaxPlugins)       // unchanged
	assert.Equal(t, "./plugins", cfg.Plugin.PluginsDir) // never changed
}

func TestApplySettingsUpdate_Outbox(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)
	update := &domainsettings.SettingsUpdate{
		Outbox: &domainsettings.OutboxUpdate{
			RelayWorkers: intPtr(4),
		},
	}

	err := applier.ApplySettingsUpdate(update)

	assert.NoError(t, err)
	assert.Equal(t, 4, cfg.Outbox.RelayWorkers)
	assert.Equal(t, 2, cfg.Outbox.PartitionLookahead) // unchanged
}

func TestApplySettingsUpdate_CORS(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)
	newOrigins := []string{"https://app.example.com", "https://admin.example.com"}
	update := &domainsettings.SettingsUpdate{
		CORS: &domainsettings.CORSUpdate{
			AllowedOrigins: &newOrigins,
		},
	}

	err := applier.ApplySettingsUpdate(update)

	assert.NoError(t, err)
	assert.Equal(t, newOrigins, cfg.CORS.AllowedOrigins)
}

func TestApplySettingsUpdate_CircuitBreaker(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)
	update := &domainsettings.SettingsUpdate{
		CircuitBreaker: &domainsettings.CircuitBreakerUpdate{
			Remnawave: &domainsettings.CircuitBreakerComponentUpdate{
				MaxFailures: uint32Ptr(10),
			},
		},
	}

	err := applier.ApplySettingsUpdate(update)

	assert.NoError(t, err)
	assert.Equal(t, uint32(10), cfg.CircuitBreaker.Remnawave.MaxFailures)
	// Other components unchanged.
	assert.Equal(t, circuitbreaker.DefaultMaxFailures, cfg.CircuitBreaker.Valkey.MaxFailures)
}

func TestApplySettingsUpdate_NilSections(t *testing.T) {
	cfg := testConfig()
	applier := NewRuntimeConfigApplier(cfg)

	// An empty update should not change anything.
	err := applier.ApplySettingsUpdate(&domainsettings.SettingsUpdate{})

	assert.NoError(t, err)
	assert.Equal(t, 7, cfg.Billing.TrialDays)
	assert.Equal(t, 10, cfg.RateLimit.CheckoutMaxPerHour)
}
