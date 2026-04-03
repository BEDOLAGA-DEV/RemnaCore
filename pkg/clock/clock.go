// Package clock provides a deterministic time abstraction for domain code.
// Production code uses Real; tests use Mock for reproducible timestamps.
package clock

import "time"

// Clock provides the current time.
type Clock interface {
	Now() time.Time
}

// Real returns system time.
type Real struct{}

// NewReal creates a production clock.
func NewReal() Clock { return Real{} }

// Now returns the current system time.
func (Real) Now() time.Time { return time.Now() }

// Mock returns a fixed time for deterministic testing.
type Mock struct {
	FixedTime time.Time
}

// NewMock creates a mock clock pinned to t.
func NewMock(t time.Time) *Mock { return &Mock{FixedTime: t} }

// Now returns the fixed time.
func (m *Mock) Now() time.Time { return m.FixedTime }

// Set updates the mock clock to a new fixed time.
func (m *Mock) Set(t time.Time) { m.FixedTime = t }

// Advance moves the mock clock forward by d.
func (m *Mock) Advance(d time.Duration) { m.FixedTime = m.FixedTime.Add(d) }
