// Package backoff provides retry backoff utilities including jitter to prevent
// thundering herd effects when multiple clients retry simultaneously.
package backoff

import (
	"math/rand/v2"
	"time"
)

// JitterFraction is the proportion of the base duration used for jitter.
// A value of 0.25 means the returned duration will be in the range
// [base * 0.75, base * 1.25].
const JitterFraction = 0.25

// WithJitter adds +/-25% random jitter to a duration to prevent thundering
// herd effects. The result is always non-negative; if the computed value would
// go below zero (extremely short durations), zero is returned.
func WithJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	// jitterRange = d/2 (50% of d, centered at d => +/-25%)
	jitterRange := int64(d) / 2
	if jitterRange == 0 {
		return d
	}
	// offset is in [0, jitterRange), shift to [-d/4, +d/4)
	offset := rand.Int64N(jitterRange) - int64(d)/4
	result := int64(d) + offset
	if result < 0 {
		return 0
	}
	return time.Duration(result)
}
