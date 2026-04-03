package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

func trialSubscription() *aggregate.Subscription {
	now := time.Now()
	return &aggregate.Subscription{
		ID:        "sub-1",
		UserID:    "user-1",
		PlanID:    "plan-1",
		Status:    aggregate.StatusTrial,
		Period:    vo.NewBillingPeriod(now, vo.IntervalMonth),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestNewTrialManager_DefaultDays(t *testing.T) {
	tm := NewTrialManager(0)

	assert.Equal(t, DefaultTrialDays, tm.TrialDays())
}

func TestNewTrialManager_CustomDays(t *testing.T) {
	tm := NewTrialManager(14)

	assert.Equal(t, 14, tm.TrialDays())
}

func TestTrialManager_StartTrial(t *testing.T) {
	tm := NewTrialManager(DefaultTrialDays)
	sub := trialSubscription()

	err := tm.StartTrial(sub)

	require.NoError(t, err)
	assert.Equal(t, aggregate.StatusTrial, sub.Status)

	// Period should span exactly trialDays
	expectedDuration := time.Duration(DefaultTrialDays) * hoursPerDay * time.Hour
	actualDuration := sub.Period.End.Sub(sub.Period.Start)
	assert.InDelta(t, expectedDuration.Hours(), actualDuration.Hours(), 1)
}

func TestTrialManager_StartTrial_NotTrialStatus(t *testing.T) {
	tm := NewTrialManager(DefaultTrialDays)
	sub := trialSubscription()
	sub.Status = aggregate.StatusActive

	err := tm.StartTrial(sub)

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrNotTrialStatus)
}

func TestTrialManager_IsTrialExpiring_True(t *testing.T) {
	tm := NewTrialManager(DefaultTrialDays)
	sub := trialSubscription()

	// Set period to end in 2 days
	now := time.Now()
	sub.Period = vo.BillingPeriod{
		Start:    now.AddDate(0, 0, -5),
		End:      now.AddDate(0, 0, 2),
		Interval: vo.IntervalMonth,
	}

	warningDays := 3
	assert.True(t, tm.IsTrialExpiring(sub, warningDays))
}

func TestTrialManager_IsTrialExpiring_False(t *testing.T) {
	tm := NewTrialManager(DefaultTrialDays)
	sub := trialSubscription()

	// Set period to end in 10 days
	now := time.Now()
	sub.Period = vo.BillingPeriod{
		Start:    now,
		End:      now.AddDate(0, 0, 10),
		Interval: vo.IntervalMonth,
	}

	warningDays := 3
	assert.False(t, tm.IsTrialExpiring(sub, warningDays))
}

func TestTrialManager_IsTrialExpiring_NotTrialStatus(t *testing.T) {
	tm := NewTrialManager(DefaultTrialDays)
	sub := trialSubscription()
	sub.Status = aggregate.StatusActive

	assert.False(t, tm.IsTrialExpiring(sub, 3))
}

func TestTrialManager_IsTrialExpired_True(t *testing.T) {
	tm := NewTrialManager(DefaultTrialDays)
	sub := trialSubscription()

	// Set period end to the past
	now := time.Now()
	sub.Period = vo.BillingPeriod{
		Start:    now.AddDate(0, 0, -10),
		End:      now.AddDate(0, 0, -1),
		Interval: vo.IntervalMonth,
	}

	assert.True(t, tm.IsTrialExpired(sub))
}

func TestTrialManager_IsTrialExpired_False(t *testing.T) {
	tm := NewTrialManager(DefaultTrialDays)
	sub := trialSubscription()

	// Period end is in the future
	now := time.Now()
	sub.Period = vo.BillingPeriod{
		Start:    now,
		End:      now.AddDate(0, 0, 5),
		Interval: vo.IntervalMonth,
	}

	assert.False(t, tm.IsTrialExpired(sub))
}

func TestTrialManager_IsTrialExpired_NotTrialStatus(t *testing.T) {
	tm := NewTrialManager(DefaultTrialDays)
	sub := trialSubscription()
	sub.Status = aggregate.StatusActive

	assert.False(t, tm.IsTrialExpired(sub))
}
