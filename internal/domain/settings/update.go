package settings

import "time"

// SettingsUpdate holds partial updates for editable runtime settings.
// Only non-nil fields are applied; nil fields are left unchanged.
type SettingsUpdate struct {
	Billing        *BillingUpdate        `json:"billing,omitempty"`
	RateLimit      *RateLimitUpdate      `json:"rate_limit,omitempty"`
	FeatureFlags   *FeatureFlagUpdate    `json:"feature_flags,omitempty"`
	SmartRouter    *SmartRouterUpdate    `json:"smart_router,omitempty"`
	SpeedTest      *SpeedTestUpdate      `json:"speed_test,omitempty"`
	Plugins        *PluginUpdate         `json:"plugins,omitempty"`
	Outbox         *OutboxUpdate         `json:"outbox,omitempty"`
	CORS           *CORSUpdate           `json:"cors,omitempty"`
	CircuitBreaker *CircuitBreakerUpdate `json:"circuit_breaker,omitempty"`
}

// BillingUpdate holds editable billing fields.
type BillingUpdate struct {
	TrialDays *int `json:"trial_days,omitempty"`
}

// RateLimitUpdate holds editable rate limit fields.
type RateLimitUpdate struct {
	CheckoutMaxPerHour     *int `json:"checkout_max_per_hour,omitempty"`
	SubscriptionMaxPerDay  *int `json:"subscription_max_per_day,omitempty"`
	LoginMaxPerWindow      *int `json:"login_max_per_window,omitempty"`
	LoginWindowMinutes     *int `json:"login_window_minutes,omitempty"`
	ForgotPwdMaxPerWindow  *int `json:"forgot_pwd_max_per_window,omitempty"`
	ForgotPwdWindowMinutes *int `json:"forgot_pwd_window_minutes,omitempty"`
}

// FeatureFlagUpdate holds editable feature flag fields.
type FeatureFlagUpdate struct {
	HooksSubscriptionEnabled *bool `json:"hooks_subscription_enabled,omitempty"`
	HooksVPNProviderEnabled  *bool `json:"hooks_vpn_provider_enabled,omitempty"`
}

// SmartRouterUpdate holds editable smart router weight fields.
type SmartRouterUpdate struct {
	WeightGeo     *float64 `json:"weight_geo,omitempty"`
	WeightLatency *float64 `json:"weight_latency,omitempty"`
	WeightLoad    *float64 `json:"weight_load,omitempty"`

	WeightGamingGeo     *float64 `json:"weight_gaming_geo,omitempty"`
	WeightGamingLatency *float64 `json:"weight_gaming_latency,omitempty"`
	WeightGamingLoad    *float64 `json:"weight_gaming_load,omitempty"`

	WeightStreamingGeo     *float64 `json:"weight_streaming_geo,omitempty"`
	WeightStreamingLatency *float64 `json:"weight_streaming_latency,omitempty"`
	WeightStreamingLoad    *float64 `json:"weight_streaming_load,omitempty"`
}

// SpeedTestUpdate holds editable speed test fields.
type SpeedTestUpdate struct {
	MaxConcurrent  *int `json:"max_concurrent,omitempty"`
	PerIPRateLimit *int `json:"per_ip_rate_limit,omitempty"`
	MaxUploadBytes *int `json:"max_upload_bytes,omitempty"`
}

// PluginUpdate holds editable plugin fields.
// Note: plugins_dir is NOT editable at runtime.
type PluginUpdate struct {
	MaxPlugins      *int  `json:"max_plugins,omitempty"`
	EnableHotReload *bool `json:"enable_hot_reload,omitempty"`
}

// OutboxUpdate holds editable outbox fields.
type OutboxUpdate struct {
	RelayWorkers       *int `json:"relay_workers,omitempty"`
	PartitionLookahead *int `json:"partition_lookahead,omitempty"`
	RetentionDays      *int `json:"retention_days,omitempty"`
}

// CORSUpdate holds editable CORS fields.
type CORSUpdate struct {
	AllowedOrigins *[]string `json:"allowed_origins,omitempty"`
}

// CircuitBreakerComponentUpdate holds editable circuit breaker fields
// for a single component.
type CircuitBreakerComponentUpdate struct {
	MaxFailures *uint32        `json:"max_failures,omitempty"`
	Timeout     *time.Duration `json:"timeout,omitempty"`
	MaxRequests *uint32        `json:"max_requests,omitempty"`
	Interval    *time.Duration `json:"interval,omitempty"`
}

// CircuitBreakerUpdate holds editable circuit breaker fields per component.
type CircuitBreakerUpdate struct {
	Remnawave   *CircuitBreakerComponentUpdate `json:"remnawave,omitempty"`
	OutboxNATS  *CircuitBreakerComponentUpdate `json:"outbox_nats,omitempty"`
	Valkey      *CircuitBreakerComponentUpdate `json:"valkey,omitempty"`
	VPNProvider *CircuitBreakerComponentUpdate `json:"vpn_provider,omitempty"`
}
