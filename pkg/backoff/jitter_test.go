package backoff

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithJitter_BoundsCheck(t *testing.T) {
	const iterations = 10000
	base := 1 * time.Second
	minExpected := time.Duration(float64(base) * (1 - JitterFraction))
	maxExpected := time.Duration(float64(base) * (1 + JitterFraction))

	for range iterations {
		result := WithJitter(base)
		assert.GreaterOrEqual(t, result, minExpected,
			"jittered duration %v should be >= %v", result, minExpected)
		assert.Less(t, result, maxExpected,
			"jittered duration %v should be < %v", result, maxExpected)
	}
}

func TestWithJitter_ZeroDuration(t *testing.T) {
	assert.Equal(t, time.Duration(0), WithJitter(0))
}

func TestWithJitter_NegativeDuration(t *testing.T) {
	assert.Equal(t, time.Duration(0), WithJitter(-1*time.Second))
}

func TestWithJitter_VerySmallDuration(t *testing.T) {
	// Durations so small that jitterRange rounds to 0 should return unchanged.
	result := WithJitter(1 * time.Nanosecond)
	assert.Equal(t, 1*time.Nanosecond, result)
}

func TestWithJitter_NotConstant(t *testing.T) {
	// Verify that WithJitter produces varying results (not always the same).
	base := 10 * time.Second
	seen := make(map[time.Duration]struct{})

	const iterations = 100
	for range iterations {
		seen[WithJitter(base)] = struct{}{}
	}

	assert.Greater(t, len(seen), 1,
		"WithJitter should produce varying results over %d calls", iterations)
}
