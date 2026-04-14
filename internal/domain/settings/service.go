package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// weightMin is the minimum allowed value for a smart router weight.
const weightMin = 0.0

// weightMax is the maximum allowed value for a smart router weight.
const weightMax = 1.0

// Service manages runtime configuration overrides. It reads the current
// overrides from the database, merges incoming partial updates, persists
// the merged result, and applies it via the ConfigApplier port.
type Service struct {
	mu      sync.Mutex
	applier ConfigApplier
	repo    OverrideRepository
	logger  *slog.Logger
}

// NewService creates a Service that manages runtime overrides, persisted via
// repo and applied to the runtime configuration via applier.
func NewService(applier ConfigApplier, repo OverrideRepository, logger *slog.Logger) *Service {
	return &Service{
		applier: applier,
		repo:    repo,
		logger:  logger,
	}
}

// LoadOverrides reads persisted overrides from the database and applies them
// to the runtime configuration via the ConfigApplier. This should be called
// once at startup after the database connection is established.
func (s *Service) LoadOverrides(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := s.repo.GetOverrides(ctx)
	if err != nil {
		return fmt.Errorf("load settings overrides: %w", err)
	}

	// An empty blob or empty JSON object means no overrides.
	if len(raw) == 0 || string(raw) == "{}" {
		s.logger.Info("no runtime settings overrides found")
		return nil
	}

	var update SettingsUpdate
	if err := json.Unmarshal(raw, &update); err != nil {
		return fmt.Errorf("unmarshal settings overrides: %w", err)
	}

	if err := s.applier.ApplySettingsUpdate(&update); err != nil {
		return fmt.Errorf("apply loaded settings overrides: %w", err)
	}

	s.logger.Info("applied runtime settings overrides from database")

	return nil
}

// ApplyOverrides validates the incoming partial update, merges it with the
// existing persisted overrides, saves the merged result to the database,
// and applies the values via the ConfigApplier.
func (s *Service) ApplyOverrides(ctx context.Context, partial SettingsUpdate) error {
	if err := validate(&partial); err != nil {
		return fmt.Errorf("validate settings update: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Read current full overrides from DB to preserve unrelated changes.
	raw, err := s.repo.GetOverrides(ctx)
	if err != nil {
		return fmt.Errorf("read existing overrides: %w", err)
	}

	var existing SettingsUpdate
	if len(raw) > 0 && string(raw) != "{}" {
		if err := json.Unmarshal(raw, &existing); err != nil {
			return fmt.Errorf("unmarshal existing overrides: %w", err)
		}
	}

	// Merge the incoming partial update onto the existing overrides.
	merged := mergeUpdates(&existing, &partial)

	// Serialize and persist.
	mergedRaw, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("marshal merged overrides: %w", err)
	}

	if err := s.repo.SaveOverrides(ctx, mergedRaw); err != nil {
		return fmt.Errorf("save merged overrides: %w", err)
	}

	// Apply the full merged overrides to the runtime configuration.
	if err := s.applier.ApplySettingsUpdate(merged); err != nil {
		return fmt.Errorf("apply merged settings overrides: %w", err)
	}

	s.logger.Info("applied runtime settings update")

	return nil
}

// validate checks that all provided values in the update are within
// acceptable bounds.
func validate(u *SettingsUpdate) error {
	if u.Billing != nil {
		if err := validateBilling(u.Billing); err != nil {
			return err
		}
	}
	if u.RateLimit != nil {
		if err := validateRateLimit(u.RateLimit); err != nil {
			return err
		}
	}
	if u.SmartRouter != nil {
		if err := validateSmartRouter(u.SmartRouter); err != nil {
			return err
		}
	}
	if u.SpeedTest != nil {
		if err := validateSpeedTest(u.SpeedTest); err != nil {
			return err
		}
	}
	if u.Plugins != nil {
		if err := validatePlugins(u.Plugins); err != nil {
			return err
		}
	}
	if u.Outbox != nil {
		if err := validateOutbox(u.Outbox); err != nil {
			return err
		}
	}
	if u.CORS != nil {
		if err := validateCORS(u.CORS); err != nil {
			return err
		}
	}
	if u.CircuitBreaker != nil {
		if err := validateCircuitBreaker(u.CircuitBreaker); err != nil {
			return err
		}
	}
	return nil
}

func validateBilling(b *BillingUpdate) error {
	if b.TrialDays != nil && *b.TrialDays <= 0 {
		return ErrInvalidTrialDays
	}
	return nil
}

func validateRateLimit(rl *RateLimitUpdate) error {
	positiveChecks := []*int{
		rl.CheckoutMaxPerHour,
		rl.SubscriptionMaxPerDay,
		rl.LoginMaxPerWindow,
		rl.LoginWindowMinutes,
		rl.ForgotPwdMaxPerWindow,
		rl.ForgotPwdWindowMinutes,
	}
	for _, v := range positiveChecks {
		if v != nil && *v <= 0 {
			return ErrInvalidRateLimit
		}
	}
	return nil
}

func validateSmartRouter(sr *SmartRouterUpdate) error {
	weightChecks := []*float64{
		sr.WeightGeo, sr.WeightLatency, sr.WeightLoad,
		sr.WeightGamingGeo, sr.WeightGamingLatency, sr.WeightGamingLoad,
		sr.WeightStreamingGeo, sr.WeightStreamingLatency, sr.WeightStreamingLoad,
	}
	for _, v := range weightChecks {
		if v != nil && (*v < weightMin || *v > weightMax) {
			return ErrInvalidWeight
		}
	}
	return nil
}

func validateSpeedTest(st *SpeedTestUpdate) error {
	if st.MaxConcurrent != nil && *st.MaxConcurrent <= 0 {
		return ErrInvalidMaxConcurrent
	}
	if st.PerIPRateLimit != nil && *st.PerIPRateLimit <= 0 {
		return ErrInvalidPerIPRateLimit
	}
	if st.MaxUploadBytes != nil && *st.MaxUploadBytes <= 0 {
		return ErrInvalidMaxUploadBytes
	}
	return nil
}

func validatePlugins(p *PluginUpdate) error {
	if p.MaxPlugins != nil && *p.MaxPlugins <= 0 {
		return ErrInvalidMaxPlugins
	}
	return nil
}

func validateOutbox(o *OutboxUpdate) error {
	if o.RelayWorkers != nil && *o.RelayWorkers <= 0 {
		return ErrInvalidRelayWorkers
	}
	if o.PartitionLookahead != nil && *o.PartitionLookahead < 0 {
		return ErrInvalidPartitionLookahead
	}
	if o.RetentionDays != nil && *o.RetentionDays < 0 {
		return ErrInvalidRetentionDays
	}
	return nil
}

func validateCORS(c *CORSUpdate) error {
	if c.AllowedOrigins != nil && len(*c.AllowedOrigins) == 0 {
		return ErrEmptyAllowedOrigins
	}
	return nil
}

func validateCircuitBreaker(cb *CircuitBreakerUpdate) error {
	components := []*CircuitBreakerComponentUpdate{
		cb.Remnawave, cb.OutboxNATS, cb.Valkey, cb.VPNProvider,
	}
	for _, comp := range components {
		if comp == nil {
			continue
		}
		if comp.MaxFailures != nil && *comp.MaxFailures == 0 {
			return ErrInvalidCBMaxFailures
		}
		if comp.Timeout != nil && *comp.Timeout <= 0 {
			return ErrInvalidCBTimeout
		}
	}
	return nil
}

// mergeUpdates creates a new SettingsUpdate by overlaying partial onto
// existing. Non-nil fields in partial replace the corresponding fields
// in existing.
func mergeUpdates(existing, partial *SettingsUpdate) *SettingsUpdate {
	merged := *existing

	if partial.Billing != nil {
		merged.Billing = mergeBilling(existing.Billing, partial.Billing)
	}
	if partial.RateLimit != nil {
		merged.RateLimit = mergeRateLimit(existing.RateLimit, partial.RateLimit)
	}
	if partial.FeatureFlags != nil {
		merged.FeatureFlags = mergeFeatureFlags(existing.FeatureFlags, partial.FeatureFlags)
	}
	if partial.SmartRouter != nil {
		merged.SmartRouter = mergeSmartRouter(existing.SmartRouter, partial.SmartRouter)
	}
	if partial.SpeedTest != nil {
		merged.SpeedTest = mergeSpeedTest(existing.SpeedTest, partial.SpeedTest)
	}
	if partial.Plugins != nil {
		merged.Plugins = mergePlugins(existing.Plugins, partial.Plugins)
	}
	if partial.Outbox != nil {
		merged.Outbox = mergeOutbox(existing.Outbox, partial.Outbox)
	}
	if partial.CORS != nil {
		merged.CORS = partial.CORS // CORS is replaced entirely
	}
	if partial.CircuitBreaker != nil {
		merged.CircuitBreaker = mergeCircuitBreaker(existing.CircuitBreaker, partial.CircuitBreaker)
	}

	return &merged
}

func mergeBilling(existing, partial *BillingUpdate) *BillingUpdate {
	out := &BillingUpdate{}
	if existing != nil {
		*out = *existing
	}
	if partial.TrialDays != nil {
		out.TrialDays = partial.TrialDays
	}
	return out
}

func mergeRateLimit(existing, partial *RateLimitUpdate) *RateLimitUpdate {
	out := &RateLimitUpdate{}
	if existing != nil {
		*out = *existing
	}
	mergeIntPtr(&out.CheckoutMaxPerHour, partial.CheckoutMaxPerHour)
	mergeIntPtr(&out.SubscriptionMaxPerDay, partial.SubscriptionMaxPerDay)
	mergeIntPtr(&out.LoginMaxPerWindow, partial.LoginMaxPerWindow)
	mergeIntPtr(&out.LoginWindowMinutes, partial.LoginWindowMinutes)
	mergeIntPtr(&out.ForgotPwdMaxPerWindow, partial.ForgotPwdMaxPerWindow)
	mergeIntPtr(&out.ForgotPwdWindowMinutes, partial.ForgotPwdWindowMinutes)
	return out
}

func mergeFeatureFlags(existing, partial *FeatureFlagUpdate) *FeatureFlagUpdate {
	out := &FeatureFlagUpdate{}
	if existing != nil {
		*out = *existing
	}
	mergeBoolPtr(&out.HooksSubscriptionEnabled, partial.HooksSubscriptionEnabled)
	mergeBoolPtr(&out.HooksVPNProviderEnabled, partial.HooksVPNProviderEnabled)
	return out
}

func mergeSmartRouter(existing, partial *SmartRouterUpdate) *SmartRouterUpdate {
	out := &SmartRouterUpdate{}
	if existing != nil {
		*out = *existing
	}
	mergeFloat64Ptr(&out.WeightGeo, partial.WeightGeo)
	mergeFloat64Ptr(&out.WeightLatency, partial.WeightLatency)
	mergeFloat64Ptr(&out.WeightLoad, partial.WeightLoad)
	mergeFloat64Ptr(&out.WeightGamingGeo, partial.WeightGamingGeo)
	mergeFloat64Ptr(&out.WeightGamingLatency, partial.WeightGamingLatency)
	mergeFloat64Ptr(&out.WeightGamingLoad, partial.WeightGamingLoad)
	mergeFloat64Ptr(&out.WeightStreamingGeo, partial.WeightStreamingGeo)
	mergeFloat64Ptr(&out.WeightStreamingLatency, partial.WeightStreamingLatency)
	mergeFloat64Ptr(&out.WeightStreamingLoad, partial.WeightStreamingLoad)
	return out
}

func mergeSpeedTest(existing, partial *SpeedTestUpdate) *SpeedTestUpdate {
	out := &SpeedTestUpdate{}
	if existing != nil {
		*out = *existing
	}
	mergeIntPtr(&out.MaxConcurrent, partial.MaxConcurrent)
	mergeIntPtr(&out.PerIPRateLimit, partial.PerIPRateLimit)
	mergeIntPtr(&out.MaxUploadBytes, partial.MaxUploadBytes)
	return out
}

func mergePlugins(existing, partial *PluginUpdate) *PluginUpdate {
	out := &PluginUpdate{}
	if existing != nil {
		*out = *existing
	}
	mergeIntPtr(&out.MaxPlugins, partial.MaxPlugins)
	mergeBoolPtr(&out.EnableHotReload, partial.EnableHotReload)
	return out
}

func mergeOutbox(existing, partial *OutboxUpdate) *OutboxUpdate {
	out := &OutboxUpdate{}
	if existing != nil {
		*out = *existing
	}
	mergeIntPtr(&out.RelayWorkers, partial.RelayWorkers)
	mergeIntPtr(&out.PartitionLookahead, partial.PartitionLookahead)
	mergeIntPtr(&out.RetentionDays, partial.RetentionDays)
	return out
}

func mergeCircuitBreaker(existing, partial *CircuitBreakerUpdate) *CircuitBreakerUpdate {
	out := &CircuitBreakerUpdate{}
	if existing != nil {
		*out = *existing
	}
	if partial.Remnawave != nil {
		out.Remnawave = mergeCBComponent(existing.cbComponent(func(u *CircuitBreakerUpdate) *CircuitBreakerComponentUpdate { return u.Remnawave }), partial.Remnawave)
	}
	if partial.OutboxNATS != nil {
		out.OutboxNATS = mergeCBComponent(existing.cbComponent(func(u *CircuitBreakerUpdate) *CircuitBreakerComponentUpdate { return u.OutboxNATS }), partial.OutboxNATS)
	}
	if partial.Valkey != nil {
		out.Valkey = mergeCBComponent(existing.cbComponent(func(u *CircuitBreakerUpdate) *CircuitBreakerComponentUpdate { return u.Valkey }), partial.Valkey)
	}
	if partial.VPNProvider != nil {
		out.VPNProvider = mergeCBComponent(existing.cbComponent(func(u *CircuitBreakerUpdate) *CircuitBreakerComponentUpdate { return u.VPNProvider }), partial.VPNProvider)
	}
	return out
}

// cbComponent safely extracts a component from a potentially nil CircuitBreakerUpdate.
func (u *CircuitBreakerUpdate) cbComponent(fn func(*CircuitBreakerUpdate) *CircuitBreakerComponentUpdate) *CircuitBreakerComponentUpdate {
	if u == nil {
		return nil
	}
	return fn(u)
}

func mergeCBComponent(existing, partial *CircuitBreakerComponentUpdate) *CircuitBreakerComponentUpdate {
	out := &CircuitBreakerComponentUpdate{}
	if existing != nil {
		*out = *existing
	}
	mergeUint32Ptr(&out.MaxFailures, partial.MaxFailures)
	mergeDurationPtr(&out.Timeout, partial.Timeout)
	mergeUint32Ptr(&out.MaxRequests, partial.MaxRequests)
	mergeDurationPtr(&out.Interval, partial.Interval)
	return out
}

// --- pointer merge helpers ---

func mergeIntPtr(dst **int, src *int) {
	if src != nil {
		*dst = src
	}
}

func mergeBoolPtr(dst **bool, src *bool) {
	if src != nil {
		*dst = src
	}
}

func mergeFloat64Ptr(dst **float64, src *float64) {
	if src != nil {
		*dst = src
	}
}

func mergeUint32Ptr(dst **uint32, src *uint32) {
	if src != nil {
		*dst = src
	}
}

func mergeDurationPtr(dst **time.Duration, src *time.Duration) {
	if src != nil {
		*dst = src
	}
}
