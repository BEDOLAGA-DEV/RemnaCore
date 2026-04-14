package settings

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// --- mock config applier ---

type mockConfigApplier struct {
	lastUpdate *SettingsUpdate
	callCount  int
	err        error
}

func (m *mockConfigApplier) ApplySettingsUpdate(update *SettingsUpdate) error {
	m.callCount++
	m.lastUpdate = update
	return m.err
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
	applier := &mockConfigApplier{}
	logger := slog.Default()
	svc := NewService(applier, repo, logger)

	ctx := context.Background()

	// Apply an update.
	update := SettingsUpdate{
		Billing: &BillingUpdate{TrialDays: intPtr(30)},
	}
	err := svc.ApplyOverrides(ctx, update)
	require.NoError(t, err)

	// Applier was called with the merged update.
	require.Equal(t, 1, applier.callCount)
	require.NotNil(t, applier.lastUpdate)
	require.NotNil(t, applier.lastUpdate.Billing)
	assert.Equal(t, 30, *applier.lastUpdate.Billing.TrialDays)

	// Overrides were persisted.
	require.NotNil(t, repo.stored)

	// Apply a second, independent update — billing should be preserved in merge.
	update2 := SettingsUpdate{
		FeatureFlags: &FeatureFlagUpdate{
			HooksSubscriptionEnabled: boolPtr(true),
		},
	}
	err = svc.ApplyOverrides(ctx, update2)
	require.NoError(t, err)

	// Applier called a second time with both changes merged.
	require.Equal(t, 2, applier.callCount)
	require.NotNil(t, applier.lastUpdate.Billing)
	assert.Equal(t, 30, *applier.lastUpdate.Billing.TrialDays)
	require.NotNil(t, applier.lastUpdate.FeatureFlags)
	assert.True(t, *applier.lastUpdate.FeatureFlags.HooksSubscriptionEnabled)
}

func TestLoadOverrides_EmptyBlob(t *testing.T) {
	repo := &mockOverrideRepo{stored: json.RawMessage(`{}`)}
	applier := &mockConfigApplier{}
	logger := slog.Default()
	svc := NewService(applier, repo, logger)

	err := svc.LoadOverrides(context.Background())
	require.NoError(t, err)

	// Applier was NOT called — no overrides to apply.
	assert.Equal(t, 0, applier.callCount)
}

func TestLoadOverrides_WithPersistedValues(t *testing.T) {
	stored := json.RawMessage(`{"billing":{"trial_days":21}}`)
	repo := &mockOverrideRepo{stored: stored}
	applier := &mockConfigApplier{}
	logger := slog.Default()
	svc := NewService(applier, repo, logger)

	err := svc.LoadOverrides(context.Background())
	require.NoError(t, err)

	// Applier was called with the deserialized update.
	require.Equal(t, 1, applier.callCount)
	require.NotNil(t, applier.lastUpdate)
	require.NotNil(t, applier.lastUpdate.Billing)
	assert.Equal(t, 21, *applier.lastUpdate.Billing.TrialDays)
}

func TestApplyOverrides_ValidationRejectsInvalid(t *testing.T) {
	repo := &mockOverrideRepo{}
	applier := &mockConfigApplier{}
	logger := slog.Default()
	svc := NewService(applier, repo, logger)

	update := SettingsUpdate{
		Billing: &BillingUpdate{TrialDays: intPtr(-5)},
	}
	err := svc.ApplyOverrides(context.Background(), update)
	require.Error(t, err)

	// Applier was NOT called — validation failed before apply.
	assert.Equal(t, 0, applier.callCount)

	// Nothing was persisted.
	assert.Nil(t, repo.stored)
}
