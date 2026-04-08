package settings

import "errors"

// Sentinel errors for the settings domain.
var (
	// ErrInvalidTrialDays is returned when trial_days is not a positive integer.
	ErrInvalidTrialDays = errors.New("trial_days must be greater than zero")

	// ErrInvalidRateLimit is returned when a rate limit value is not positive.
	ErrInvalidRateLimit = errors.New("rate limit values must be greater than zero")

	// ErrInvalidWeight is returned when a smart router weight is outside [0, 1].
	ErrInvalidWeight = errors.New("smart router weights must be between 0 and 1")

	// ErrInvalidMaxConcurrent is returned when max_concurrent is not positive.
	ErrInvalidMaxConcurrent = errors.New("max_concurrent must be greater than zero")

	// ErrInvalidPerIPRateLimit is returned when per_ip_rate_limit is not positive.
	ErrInvalidPerIPRateLimit = errors.New("per_ip_rate_limit must be greater than zero")

	// ErrInvalidMaxUploadBytes is returned when max_upload_bytes is not positive.
	ErrInvalidMaxUploadBytes = errors.New("max_upload_bytes must be greater than zero")

	// ErrInvalidMaxPlugins is returned when max_plugins is not positive.
	ErrInvalidMaxPlugins = errors.New("max_plugins must be greater than zero")

	// ErrInvalidRelayWorkers is returned when relay_workers is not positive.
	ErrInvalidRelayWorkers = errors.New("relay_workers must be greater than zero")

	// ErrInvalidPartitionLookahead is returned when partition_lookahead is negative.
	ErrInvalidPartitionLookahead = errors.New("partition_lookahead must be non-negative")

	// ErrInvalidRetentionDays is returned when retention_days is negative.
	ErrInvalidRetentionDays = errors.New("retention_days must be non-negative")

	// ErrInvalidCBMaxFailures is returned when circuit breaker max_failures is zero.
	ErrInvalidCBMaxFailures = errors.New("circuit breaker max_failures must be greater than zero")

	// ErrInvalidCBTimeout is returned when circuit breaker timeout is not positive.
	ErrInvalidCBTimeout = errors.New("circuit breaker timeout must be positive")

	// ErrEmptyAllowedOrigins is returned when CORS allowed_origins is explicitly set to an empty list.
	ErrEmptyAllowedOrigins = errors.New("cors allowed_origins must not be empty when provided")
)
