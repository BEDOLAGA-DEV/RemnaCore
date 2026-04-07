package circuitbreaker

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errFake = errors.New("fake failure")

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name string
		got  any
		want any
	}{
		{name: "max_failures", got: cfg.MaxFailures, want: DefaultMaxFailures},
		{name: "timeout", got: cfg.Timeout, want: DefaultTimeout},
		{name: "max_requests", got: cfg.MaxRequests, want: DefaultMaxRequests},
		{name: "interval", got: cfg.Interval, want: DefaultInterval},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.got)
		})
	}
}

func TestDefaultConfigNoInterval_ZeroInterval(t *testing.T) {
	cfg := DefaultConfigNoInterval()

	assert.Equal(t, time.Duration(0), cfg.Interval, "interval must be zero")
	assert.Equal(t, DefaultMaxFailures, cfg.MaxFailures, "max_failures should match default")
}

func TestNewBreaker_TripsAfterMaxFailures(t *testing.T) {
	cfg := DefaultConfig()
	cb := NewBreaker[struct{}]("test-breaker", cfg, nil)

	for i := range cfg.MaxFailures {
		t.Run(fmt.Sprintf("failure_%d", i+1), func(t *testing.T) {
			_, err := cb.Execute(func() (struct{}, error) {
				return struct{}{}, errFake
			})
			require.Error(t, err)
		})
	}

	assert.Equal(t, gobreaker.StateOpen, cb.State(),
		"breaker should be open after %d consecutive failures", cfg.MaxFailures)

	functionCalled := false
	_, err := cb.Execute(func() (struct{}, error) {
		functionCalled = true
		return struct{}{}, nil
	})
	assert.Error(t, err)
	assert.False(t, functionCalled, "function must not be called when breaker is open")
}

func TestNewBreaker_OnStateChangeCallback(t *testing.T) {
	cfg := DefaultConfig()
	var transitions []gobreaker.State

	cb := NewBreaker[struct{}]("test-cb", cfg, func(_ string, _, to gobreaker.State) {
		transitions = append(transitions, to)
	})

	// Trip the breaker.
	for range cfg.MaxFailures {
		_, _ = cb.Execute(func() (struct{}, error) {
			return struct{}{}, errFake
		})
	}

	require.Len(t, transitions, 1)
	assert.Equal(t, gobreaker.StateOpen, transitions[0])
}

func TestSettings_ReadyToTrip(t *testing.T) {
	cfg := Config{
		MaxFailures: 3,
		Timeout:     5 * time.Second,
		MaxRequests: 1,
		Interval:    0,
	}

	s := Settings("test", cfg, nil)

	tests := []struct {
		name               string
		consecutiveFailures uint32
		want               bool
	}{
		{name: "below threshold", consecutiveFailures: 2, want: false},
		{name: "at threshold", consecutiveFailures: 3, want: true},
		{name: "above threshold", consecutiveFailures: 10, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.ReadyToTrip(gobreaker.Counts{ConsecutiveFailures: tt.consecutiveFailures})
			assert.Equal(t, tt.want, got)
		})
	}
}
