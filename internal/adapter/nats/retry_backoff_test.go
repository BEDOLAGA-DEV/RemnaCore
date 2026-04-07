package nats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryBackoffDelay(t *testing.T) {
	tests := []struct {
		name       string
		attempt    int
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{
			name:    "attempt 1 uses RetryDelay1",
			attempt: 1,
			wantMin: RetryDelay1,
			wantMax: RetryDelay1 + time.Duration(float64(RetryDelay1)*retryJitterFraction),
		},
		{
			name:    "attempt 2 uses RetryDelay2",
			attempt: 2,
			wantMin: RetryDelay2,
			wantMax: RetryDelay2 + time.Duration(float64(RetryDelay2)*retryJitterFraction),
		},
		{
			name:    "attempt 3 uses RetryDelay3",
			attempt: 3,
			wantMin: RetryDelay3,
			wantMax: RetryDelay3 + time.Duration(float64(RetryDelay3)*retryJitterFraction),
		},
		{
			name:    "attempt beyond slice length caps at last delay",
			attempt: 10,
			wantMin: RetryDelay3,
			wantMax: RetryDelay3 + time.Duration(float64(RetryDelay3)*retryJitterFraction),
		},
		{
			name:    "attempt 0 clamps to first delay",
			attempt: 0,
			wantMin: RetryDelay1,
			wantMax: RetryDelay1 + time.Duration(float64(RetryDelay1)*retryJitterFraction),
		},
		{
			name:    "negative attempt clamps to first delay",
			attempt: -1,
			wantMin: RetryDelay1,
			wantMax: RetryDelay1 + time.Duration(float64(RetryDelay1)*retryJitterFraction),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay := retryBackoffDelay(tt.attempt)
			assert.GreaterOrEqual(t, delay, tt.wantMin, "delay should be >= base delay")
			assert.LessOrEqual(t, delay, tt.wantMax, "delay should be <= base + jitter")
		})
	}
}

func TestRetryDelaysAreStrictlyIncreasing(t *testing.T) {
	for i := 1; i < len(retryDelays); i++ {
		assert.Greater(t, retryDelays[i], retryDelays[i-1],
			"retryDelays[%d] should be > retryDelays[%d]", i, i-1)
	}
}
