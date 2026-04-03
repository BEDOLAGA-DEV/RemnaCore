package clock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReal_Now_ReturnsCurrentTime(t *testing.T) {
	c := NewReal()

	before := time.Now()
	got := c.Now()
	after := time.Now()

	assert.False(t, got.Before(before), "Real.Now must not be before time.Now")
	assert.False(t, got.After(after), "Real.Now must not be after time.Now")
}

func TestMock_Now_ReturnsFixedTime(t *testing.T) {
	fixed := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	c := NewMock(fixed)

	assert.Equal(t, fixed, c.Now())
	assert.Equal(t, fixed, c.Now(), "consecutive calls must return the same time")
}

func TestMock_Set(t *testing.T) {
	original := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	c := NewMock(original)

	c.Set(updated)

	assert.Equal(t, updated, c.Now())
}

func TestMock_Advance(t *testing.T) {
	fixed := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	c := NewMock(fixed)

	c.Advance(2 * time.Hour)

	expected := fixed.Add(2 * time.Hour)
	assert.Equal(t, expected, c.Now())
}

func TestReal_SatisfiesClock(t *testing.T) {
	var c Clock = NewReal()
	assert.NotZero(t, c.Now())
}

func TestMock_SatisfiesClock(t *testing.T) {
	var c Clock = NewMock(time.Now())
	assert.NotZero(t, c.Now())
}
