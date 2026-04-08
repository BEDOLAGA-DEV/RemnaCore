package settings

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

// --- mock repository ---

type mockOverrideRepo struct {
	stored json.RawMessage
	err    error
}

func (m *mockOverrideRepo) GetOverrides(_ context.Context) (json.RawMessage, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.stored == nil {
		return json.RawMessage(`{}`), nil
	}
	return m.stored, nil
}

func (m *mockOverrideRepo) SaveOverrides(_ context.Context, overrides json.RawMessage) error {
	if m.err != nil {
		return m.err
	}
	m.stored = make(json.RawMessage, len(overrides))
	copy(m.stored, overrides)
	return nil
}

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

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		update  SettingsUpdate
		wantErr error
	}{
		{
			name:    "empty update is valid",
			update:  SettingsUpdate{},
			wantErr: nil,
		},
		{
			name: "valid billing trial days",
			update: SettingsUpdate{
				Billing: &BillingUpdate{TrialDays: intPtr(14)},
			},
			wantErr: nil,
		},
		{
			name: "zero trial days is invalid",
			update: SettingsUpdate{
				Billing: &BillingUpdate{TrialDays: intPtr(0)},
			},
			wantErr: ErrInvalidTrialDays,
		},
		{
			name: "negative trial days is invalid",
			update: SettingsUpdate{
				Billing: &BillingUpdate{TrialDays: intPtr(-1)},
			},
			wantErr: ErrInvalidTrialDays,
		},
		{
			name: "valid rate limits",
			update: SettingsUpdate{
				RateLimit: &RateLimitUpdate{
					CheckoutMaxPerHour:    intPtr(20),
					SubscriptionMaxPerDay: intPtr(10),
				},
			},
			wantErr: nil,
		},
		{
			name: "zero rate limit is invalid",
			update: SettingsUpdate{
				RateLimit: &RateLimitUpdate{
					CheckoutMaxPerHour: intPtr(0),
				},
			},
			wantErr: ErrInvalidRateLimit,
		},
		{
			name: "valid smart router weights",
			update: SettingsUpdate{
				SmartRouter: &SmartRouterUpdate{
					WeightGeo: float64Ptr(0.5),
				},
			},
			wantErr: nil,
		},
		{
			name: "weight above 1 is invalid",
			update: SettingsUpdate{
				SmartRouter: &SmartRouterUpdate{
					WeightGeo: float64Ptr(1.1),
				},
			},
			wantErr: ErrInvalidWeight,
		},
		{
			name: "negative weight is invalid",
			update: SettingsUpdate{
				SmartRouter: &SmartRouterUpdate{
					WeightLatency: float64Ptr(-0.1),
				},
			},
			wantErr: ErrInvalidWeight,
		},
		{
			name: "zero weight is valid (boundary)",
			update: SettingsUpdate{
				SmartRouter: &SmartRouterUpdate{
					WeightLoad: float64Ptr(0.0),
				},
			},
			wantErr: nil,
		},
		{
			name: "weight of 1 is valid (boundary)",
			update: SettingsUpdate{
				SmartRouter: &SmartRouterUpdate{
					WeightGeo: float64Ptr(1.0),
				},
			},
			wantErr: nil,
		},
		{
			name: "zero max concurrent is invalid",
			update: SettingsUpdate{
				SpeedTest: &SpeedTestUpdate{
					MaxConcurrent: intPtr(0),
				},
			},
			wantErr: ErrInvalidMaxConcurrent,
		},
		{
			name: "zero max plugins is invalid",
			update: SettingsUpdate{
				Plugins: &PluginUpdate{
					MaxPlugins: intPtr(0),
				},
			},
			wantErr: ErrInvalidMaxPlugins,
		},
		{
			name: "zero relay workers is invalid",
			update: SettingsUpdate{
				Outbox: &OutboxUpdate{
					RelayWorkers: intPtr(0),
				},
			},
			wantErr: ErrInvalidRelayWorkers,
		},
		{
			name: "negative partition lookahead is invalid",
			update: SettingsUpdate{
				Outbox: &OutboxUpdate{
					PartitionLookahead: intPtr(-1),
				},
			},
			wantErr: ErrInvalidPartitionLookahead,
		},
		{
			name: "zero retention days is valid",
			update: SettingsUpdate{
				Outbox: &OutboxUpdate{
					RetentionDays: intPtr(0),
				},
			},
			wantErr: nil,
		},
		{
			name: "empty CORS origins is invalid",
			update: SettingsUpdate{
				CORS: &CORSUpdate{
					AllowedOrigins: &[]string{},
				},
			},
			wantErr: ErrEmptyAllowedOrigins,
		},
		{
			name: "valid CORS origins",
			update: SettingsUpdate{
				CORS: &CORSUpdate{
					AllowedOrigins: &[]string{"https://example.com"},
				},
			},
			wantErr: nil,
		},
		{
			name: "zero CB max failures is invalid",
			update: SettingsUpdate{
				CircuitBreaker: &CircuitBreakerUpdate{
					Remnawave: &CircuitBreakerComponentUpdate{
						MaxFailures: uint32Ptr(0),
					},
				},
			},
			wantErr: ErrInvalidCBMaxFailures,
		},
		{
			name: "negative CB timeout is invalid",
			update: SettingsUpdate{
				CircuitBreaker: &CircuitBreakerUpdate{
					Valkey: &CircuitBreakerComponentUpdate{
						Timeout: durPtr(-1 * time.Second),
					},
				},
			},
			wantErr: ErrInvalidCBTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(&tt.update)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplyToConfig_Billing(t *testing.T) {
	cfg := testConfig()
	update := &SettingsUpdate{
		Billing: &BillingUpdate{TrialDays: intPtr(30)},
	}

	applyToConfig(cfg, update)

	assert.Equal(t, 30, cfg.Billing.TrialDays)
}

func TestApplyToConfig_RateLimit(t *testing.T) {
	cfg := testConfig()
	update := &SettingsUpdate{
		RateLimit: &RateLimitUpdate{
			CheckoutMaxPerHour:    intPtr(50),
			LoginWindowMinutes:    intPtr(30),
		},
	}

	applyToConfig(cfg, update)

	assert.Equal(t, 50, cfg.RateLimit.CheckoutMaxPerHour)
	assert.Equal(t, 30, cfg.RateLimit.LoginWindowMinutes)
	// Unchanged fields stay at original values.
	assert.Equal(t, 5, cfg.RateLimit.SubscriptionMaxPerDay)
	assert.Equal(t, 20, cfg.RateLimit.LoginMaxPerWindow)
}

func TestApplyToConfig_FeatureFlags(t *testing.T) {
	cfg := testConfig()
	update := &SettingsUpdate{
		FeatureFlags: &FeatureFlagUpdate{
			HooksSubscriptionEnabled: boolPtr(true),
		},
	}

	applyToConfig(cfg, update)

	assert.True(t, cfg.FeatureFlags.HooksSubscriptionEnabled)
	// Unchanged flag stays at original.
	assert.False(t, cfg.FeatureFlags.HooksVPNProviderEnabled)
}

func TestApplyToConfig_SmartRouter(t *testing.T) {
	cfg := testConfig()
	update := &SettingsUpdate{
		SmartRouter: &SmartRouterUpdate{
			WeightGeo: float64Ptr(0.5),
		},
	}

	applyToConfig(cfg, update)

	assert.InDelta(t, 0.5, cfg.SmartRouter.WeightGeo, 0.001)
	// Unchanged.
	assert.InDelta(t, 0.34, cfg.SmartRouter.WeightLatency, 0.001)
}

func TestApplyToConfig_SpeedTest(t *testing.T) {
	cfg := testConfig()
	update := &SettingsUpdate{
		SpeedTest: &SpeedTestUpdate{
			MaxConcurrent: intPtr(20),
		},
	}

	applyToConfig(cfg, update)

	assert.Equal(t, 20, cfg.SpeedTest.MaxConcurrent)
	assert.Equal(t, 3, cfg.SpeedTest.PerIPRateLimit) // unchanged
}

func TestApplyToConfig_Plugins(t *testing.T) {
	cfg := testConfig()
	update := &SettingsUpdate{
		Plugins: &PluginUpdate{
			EnableHotReload: boolPtr(true),
		},
	}

	applyToConfig(cfg, update)

	assert.True(t, cfg.Plugin.EnableHotReload)
	assert.Equal(t, 50, cfg.Plugin.MaxPlugins) // unchanged
	assert.Equal(t, "./plugins", cfg.Plugin.PluginsDir) // never changed
}

func TestApplyToConfig_Outbox(t *testing.T) {
	cfg := testConfig()
	update := &SettingsUpdate{
		Outbox: &OutboxUpdate{
			RelayWorkers: intPtr(4),
		},
	}

	applyToConfig(cfg, update)

	assert.Equal(t, 4, cfg.Outbox.RelayWorkers)
	assert.Equal(t, 2, cfg.Outbox.PartitionLookahead) // unchanged
}

func TestApplyToConfig_CORS(t *testing.T) {
	cfg := testConfig()
	newOrigins := []string{"https://app.example.com", "https://admin.example.com"}
	update := &SettingsUpdate{
		CORS: &CORSUpdate{
			AllowedOrigins: &newOrigins,
		},
	}

	applyToConfig(cfg, update)

	assert.Equal(t, newOrigins, cfg.CORS.AllowedOrigins)
}

func TestApplyToConfig_CircuitBreaker(t *testing.T) {
	cfg := testConfig()
	update := &SettingsUpdate{
		CircuitBreaker: &CircuitBreakerUpdate{
			Remnawave: &CircuitBreakerComponentUpdate{
				MaxFailures: uint32Ptr(10),
			},
		},
	}

	applyToConfig(cfg, update)

	assert.Equal(t, uint32(10), cfg.CircuitBreaker.Remnawave.MaxFailures)
	// Other components unchanged.
	assert.Equal(t, circuitbreaker.DefaultMaxFailures, cfg.CircuitBreaker.Valkey.MaxFailures)
}

func TestMergeUpdates_PartialOverlay(t *testing.T) {
	existing := &SettingsUpdate{
		Billing:   &BillingUpdate{TrialDays: intPtr(14)},
		RateLimit: &RateLimitUpdate{CheckoutMaxPerHour: intPtr(20)},
	}
	partial := &SettingsUpdate{
		Billing: &BillingUpdate{TrialDays: intPtr(30)},
	}

	merged := mergeUpdates(existing, partial)

	// Billing was overridden.
	require.NotNil(t, merged.Billing)
	assert.Equal(t, 30, *merged.Billing.TrialDays)

	// RateLimit preserved from existing.
	require.NotNil(t, merged.RateLimit)
	assert.Equal(t, 20, *merged.RateLimit.CheckoutMaxPerHour)
}

func TestMergeUpdates_EmptyExisting(t *testing.T) {
	existing := &SettingsUpdate{}
	partial := &SettingsUpdate{
		Billing: &BillingUpdate{TrialDays: intPtr(30)},
	}

	merged := mergeUpdates(existing, partial)

	require.NotNil(t, merged.Billing)
	assert.Equal(t, 30, *merged.Billing.TrialDays)
}

func TestApplyOverrides_EndToEnd(t *testing.T) {
	repo := &mockOverrideRepo{}
	cfg := testConfig()
	logger := slog.Default()
	svc := NewService(cfg, repo, logger)

	ctx := context.Background()

	// Apply an update.
	update := SettingsUpdate{
		Billing: &BillingUpdate{TrialDays: intPtr(30)},
	}
	err := svc.ApplyOverrides(ctx, update)
	require.NoError(t, err)

	// Config was updated in-memory.
	assert.Equal(t, 30, cfg.Billing.TrialDays)

	// Overrides were persisted.
	require.NotNil(t, repo.stored)

	// Apply a second, independent update — billing should be preserved.
	update2 := SettingsUpdate{
		FeatureFlags: &FeatureFlagUpdate{
			HooksSubscriptionEnabled: boolPtr(true),
		},
	}
	err = svc.ApplyOverrides(ctx, update2)
	require.NoError(t, err)

	// Both changes present.
	assert.Equal(t, 30, cfg.Billing.TrialDays)
	assert.True(t, cfg.FeatureFlags.HooksSubscriptionEnabled)
}

func TestLoadOverrides_EmptyBlob(t *testing.T) {
	repo := &mockOverrideRepo{stored: json.RawMessage(`{}`)}
	cfg := testConfig()
	logger := slog.Default()
	svc := NewService(cfg, repo, logger)

	err := svc.LoadOverrides(context.Background())
	require.NoError(t, err)

	// Config unchanged.
	assert.Equal(t, 7, cfg.Billing.TrialDays)
}

func TestLoadOverrides_WithPersistedValues(t *testing.T) {
	stored := json.RawMessage(`{"billing":{"trial_days":21}}`)
	repo := &mockOverrideRepo{stored: stored}
	cfg := testConfig()
	logger := slog.Default()
	svc := NewService(cfg, repo, logger)

	err := svc.LoadOverrides(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 21, cfg.Billing.TrialDays)
}

func TestApplyOverrides_ValidationRejectsInvalid(t *testing.T) {
	repo := &mockOverrideRepo{}
	cfg := testConfig()
	logger := slog.Default()
	svc := NewService(cfg, repo, logger)

	update := SettingsUpdate{
		Billing: &BillingUpdate{TrialDays: intPtr(-5)},
	}
	err := svc.ApplyOverrides(context.Background(), update)
	require.Error(t, err)

	// Config was NOT changed.
	assert.Equal(t, 7, cfg.Billing.TrialDays)

	// Nothing was persisted.
	assert.Nil(t, repo.stored)
}
