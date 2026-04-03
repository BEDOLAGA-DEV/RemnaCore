package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

func TestNewSubscription(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, []string{"addon-1"}, time.Now())

	assert.NotEmpty(t, sub.ID)
	assert.Equal(t, "user-1", sub.UserID)
	assert.Equal(t, "plan-1", sub.PlanID)
	assert.Equal(t, StatusTrial, sub.Status)
	assert.Equal(t, vo.IntervalMonth, sub.Period.Interval)
	assert.Equal(t, []string{"addon-1"}, sub.AddonIDs)
	assert.Nil(t, sub.CancelledAt)
	assert.Nil(t, sub.PausedAt)
	assert.False(t, sub.CreatedAt.IsZero())
	assert.False(t, sub.UpdatedAt.IsZero())
}

func TestSubscription_TrialToActive(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	assert.Equal(t, StatusTrial, sub.Status)

	err := sub.Activate(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
}

func TestSubscription_TrialToPaused_Invalid(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())

	err := sub.Pause(time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.Equal(t, StatusTrial, sub.Status)
}

func TestSubscription_TrialToCancelled(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())

	err := sub.Cancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
	assert.NotNil(t, sub.CancelledAt)
}

func TestSubscription_TrialToExpired(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())

	err := sub.Expire(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_ActiveToPastDue(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	err := sub.MarkPastDue(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusPastDue, sub.Status)
}

func TestSubscription_ActiveToPaused(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	err := sub.Pause(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusPaused, sub.Status)
	assert.NotNil(t, sub.PausedAt)
}

func TestSubscription_ActiveToCancelled(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	err := sub.Cancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
	assert.NotNil(t, sub.CancelledAt)
}

func TestSubscription_ActiveToExpired(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	err := sub.Expire(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_PastDueToActive(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.MarkPastDue(time.Now()))

	err := sub.Activate(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
}

func TestSubscription_PastDueToCancelled(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.MarkPastDue(time.Now()))

	err := sub.Cancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
}

func TestSubscription_PastDueToExpired(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.MarkPastDue(time.Now()))

	err := sub.Expire(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_PausedToActive_Resume(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.Pause(time.Now()))

	err := sub.Resume(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
	assert.Nil(t, sub.PausedAt)
}

func TestSubscription_PausedToCancelled(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.Pause(time.Now()))

	err := sub.Cancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
}

func TestSubscription_PausedToExpired(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.Pause(time.Now()))

	err := sub.Expire(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_CancelledToActive_Invalid(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Cancel(time.Now()))

	err := sub.Activate(time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.Equal(t, StatusCancelled, sub.Status)
}

func TestSubscription_CancelledToAny_Terminal(t *testing.T) {
	tests := []struct {
		name string
		fn   func(s *Subscription) error
	}{
		{"Activate", func(s *Subscription) error { return s.Activate(time.Now()) }},
		{"MarkPastDue", func(s *Subscription) error { return s.MarkPastDue(time.Now()) }},
		{"Pause", func(s *Subscription) error { return s.Pause(time.Now()) }},
		{"Resume", func(s *Subscription) error { return s.Resume(time.Now()) }},
		{"Expire", func(s *Subscription) error { return s.Expire(time.Now()) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
			require.NoError(t, sub.Cancel(time.Now()))

			err := tt.fn(sub)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidTransition)
		})
	}
}

func TestSubscription_ExpiredToAny_Terminal(t *testing.T) {
	tests := []struct {
		name string
		fn   func(s *Subscription) error
	}{
		{"Activate", func(s *Subscription) error { return s.Activate(time.Now()) }},
		{"MarkPastDue", func(s *Subscription) error { return s.MarkPastDue(time.Now()) }},
		{"Cancel", func(s *Subscription) error { return s.Cancel(time.Now()) }},
		{"Pause", func(s *Subscription) error { return s.Pause(time.Now()) }},
		{"Resume", func(s *Subscription) error { return s.Resume(time.Now()) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
			require.NoError(t, sub.Expire(time.Now()))

			err := tt.fn(sub)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidTransition)
		})
	}
}

func TestSubscription_Renew(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	oldEnd := sub.Period.End
	newPeriod := vo.NewBillingPeriod(oldEnd, vo.IntervalMonth)

	err := sub.Renew(newPeriod, time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
	assert.Equal(t, newPeriod.Start, sub.Period.Start)
	assert.Equal(t, newPeriod.End, sub.Period.End)
}

func TestSubscription_Renew_NotActive(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	// Still in trial
	newPeriod := vo.NewBillingPeriod(time.Now(), vo.IntervalMonth)

	err := sub.Renew(newPeriod, time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "active")
}

func TestSubscription_CanTransitionTo(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())

	assert.True(t, sub.CanTransitionTo(StatusActive))
	assert.True(t, sub.CanTransitionTo(StatusCancelled))
	assert.True(t, sub.CanTransitionTo(StatusExpired))
	assert.False(t, sub.CanTransitionTo(StatusPaused))
	assert.False(t, sub.CanTransitionTo(StatusPastDue))
}

func TestSubscription_Cancel_SetsCancelledAt(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	before := time.Now()
	err := sub.Cancel(time.Now())
	require.NoError(t, err)

	assert.NotNil(t, sub.CancelledAt)
	assert.False(t, sub.CancelledAt.Before(before))
}

func TestSubscription_Pause_SetsPausedAt(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	before := time.Now()
	err := sub.Pause(time.Now())
	require.NoError(t, err)

	assert.NotNil(t, sub.PausedAt)
	assert.False(t, sub.PausedAt.Before(before))
}

func TestSubscription_Resume_ClearsPausedAt(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.Pause(time.Now()))
	assert.NotNil(t, sub.PausedAt)

	err := sub.Resume(time.Now())
	require.NoError(t, err)

	assert.Nil(t, sub.PausedAt)
}
