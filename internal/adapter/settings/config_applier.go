// Package settings provides infrastructure adapters for the settings domain.
package settings

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	domainsettings "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/settings"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/circuitbreaker"
)

// RuntimeConfigApplier implements domainsettings.ConfigApplier by directly
// mutating the shared *config.Config. This is the infrastructure-side
// implementation that the domain layer does not know about.
type RuntimeConfigApplier struct {
	cfg *config.Config
}

// NewRuntimeConfigApplier creates a RuntimeConfigApplier that writes validated
// settings updates to the given shared config.
func NewRuntimeConfigApplier(cfg *config.Config) *RuntimeConfigApplier {
	return &RuntimeConfigApplier{cfg: cfg}
}

// ApplySettingsUpdate writes all non-nil override values from the update to the
// shared config struct.
func (a *RuntimeConfigApplier) ApplySettingsUpdate(update *domainsettings.SettingsUpdate) error {
	if update.Billing != nil {
		applyBilling(a.cfg, update.Billing)
	}
	if update.RateLimit != nil {
		applyRateLimit(a.cfg, update.RateLimit)
	}
	if update.FeatureFlags != nil {
		applyFeatureFlags(a.cfg, update.FeatureFlags)
	}
	if update.SmartRouter != nil {
		applySmartRouter(a.cfg, update.SmartRouter)
	}
	if update.SpeedTest != nil {
		applySpeedTest(a.cfg, update.SpeedTest)
	}
	if update.Plugins != nil {
		applyPlugins(a.cfg, update.Plugins)
	}
	if update.Outbox != nil {
		applyOutbox(a.cfg, update.Outbox)
	}
	if update.CORS != nil {
		applyCORS(a.cfg, update.CORS)
	}
	if update.CircuitBreaker != nil {
		applyCircuitBreaker(a.cfg, update.CircuitBreaker)
	}
	return nil
}

func applyBilling(cfg *config.Config, b *domainsettings.BillingUpdate) {
	if b.TrialDays != nil {
		cfg.Billing.TrialDays = *b.TrialDays
	}
}

func applyRateLimit(cfg *config.Config, rl *domainsettings.RateLimitUpdate) {
	if rl.CheckoutMaxPerHour != nil {
		cfg.RateLimit.CheckoutMaxPerHour = *rl.CheckoutMaxPerHour
	}
	if rl.SubscriptionMaxPerDay != nil {
		cfg.RateLimit.SubscriptionMaxPerDay = *rl.SubscriptionMaxPerDay
	}
	if rl.LoginMaxPerWindow != nil {
		cfg.RateLimit.LoginMaxPerWindow = *rl.LoginMaxPerWindow
	}
	if rl.LoginWindowMinutes != nil {
		cfg.RateLimit.LoginWindowMinutes = *rl.LoginWindowMinutes
	}
	if rl.ForgotPwdMaxPerWindow != nil {
		cfg.RateLimit.ForgotPwdMaxPerWindow = *rl.ForgotPwdMaxPerWindow
	}
	if rl.ForgotPwdWindowMinutes != nil {
		cfg.RateLimit.ForgotPwdWindowMinutes = *rl.ForgotPwdWindowMinutes
	}
}

func applyFeatureFlags(cfg *config.Config, ff *domainsettings.FeatureFlagUpdate) {
	if ff.HooksSubscriptionEnabled != nil {
		cfg.FeatureFlags.HooksSubscriptionEnabled = *ff.HooksSubscriptionEnabled
	}
	if ff.HooksVPNProviderEnabled != nil {
		cfg.FeatureFlags.HooksVPNProviderEnabled = *ff.HooksVPNProviderEnabled
	}
}

func applySmartRouter(cfg *config.Config, sr *domainsettings.SmartRouterUpdate) {
	if sr.WeightGeo != nil {
		cfg.SmartRouter.WeightGeo = *sr.WeightGeo
	}
	if sr.WeightLatency != nil {
		cfg.SmartRouter.WeightLatency = *sr.WeightLatency
	}
	if sr.WeightLoad != nil {
		cfg.SmartRouter.WeightLoad = *sr.WeightLoad
	}
	if sr.WeightGamingGeo != nil {
		cfg.SmartRouter.WeightGamingGeo = *sr.WeightGamingGeo
	}
	if sr.WeightGamingLatency != nil {
		cfg.SmartRouter.WeightGamingLatency = *sr.WeightGamingLatency
	}
	if sr.WeightGamingLoad != nil {
		cfg.SmartRouter.WeightGamingLoad = *sr.WeightGamingLoad
	}
	if sr.WeightStreamingGeo != nil {
		cfg.SmartRouter.WeightStreamingGeo = *sr.WeightStreamingGeo
	}
	if sr.WeightStreamingLatency != nil {
		cfg.SmartRouter.WeightStreamingLatency = *sr.WeightStreamingLatency
	}
	if sr.WeightStreamingLoad != nil {
		cfg.SmartRouter.WeightStreamingLoad = *sr.WeightStreamingLoad
	}
}

func applySpeedTest(cfg *config.Config, st *domainsettings.SpeedTestUpdate) {
	if st.MaxConcurrent != nil {
		cfg.SpeedTest.MaxConcurrent = *st.MaxConcurrent
	}
	if st.PerIPRateLimit != nil {
		cfg.SpeedTest.PerIPRateLimit = *st.PerIPRateLimit
	}
	if st.MaxUploadBytes != nil {
		cfg.SpeedTest.MaxUploadBytes = *st.MaxUploadBytes
	}
}

func applyPlugins(cfg *config.Config, p *domainsettings.PluginUpdate) {
	if p.MaxPlugins != nil {
		cfg.Plugin.MaxPlugins = *p.MaxPlugins
	}
	if p.EnableHotReload != nil {
		cfg.Plugin.EnableHotReload = *p.EnableHotReload
	}
}

func applyOutbox(cfg *config.Config, o *domainsettings.OutboxUpdate) {
	if o.RelayWorkers != nil {
		cfg.Outbox.RelayWorkers = *o.RelayWorkers
	}
	if o.PartitionLookahead != nil {
		cfg.Outbox.PartitionLookahead = *o.PartitionLookahead
	}
	if o.RetentionDays != nil {
		cfg.Outbox.RetentionDays = *o.RetentionDays
	}
}

func applyCORS(cfg *config.Config, c *domainsettings.CORSUpdate) {
	if c.AllowedOrigins != nil {
		cfg.CORS.AllowedOrigins = *c.AllowedOrigins
	}
}

func applyCircuitBreaker(cfg *config.Config, cb *domainsettings.CircuitBreakerUpdate) {
	if cb.Remnawave != nil {
		applyCBComponent(&cfg.CircuitBreaker.Remnawave, cb.Remnawave)
	}
	if cb.OutboxNATS != nil {
		applyCBComponent(&cfg.CircuitBreaker.OutboxNATS, cb.OutboxNATS)
	}
	if cb.Valkey != nil {
		applyCBComponent(&cfg.CircuitBreaker.Valkey, cb.Valkey)
	}
	if cb.VPNProvider != nil {
		applyCBComponent(&cfg.CircuitBreaker.VPNProvider, cb.VPNProvider)
	}
}

func applyCBComponent(cfg *circuitbreaker.Config, update *domainsettings.CircuitBreakerComponentUpdate) {
	if update.MaxFailures != nil {
		cfg.MaxFailures = *update.MaxFailures
	}
	if update.Timeout != nil {
		cfg.Timeout = *update.Timeout
	}
	if update.MaxRequests != nil {
		cfg.MaxRequests = *update.MaxRequests
	}
	if update.Interval != nil {
		cfg.Interval = *update.Interval
	}
}
