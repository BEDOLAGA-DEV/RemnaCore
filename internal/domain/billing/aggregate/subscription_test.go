package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

func TestNewSubscription(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, []string{"addon-1"})

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
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	assert.Equal(t, StatusTrial, sub.Status)

	err := sub.Activate()

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
}

func TestSubscription_TrialToPaused_Invalid(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)

	err := sub.Pause()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.Equal(t, StatusTrial, sub.Status)
}

func TestSubscription_TrialToCancelled(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)

	err := sub.Cancel()

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
	assert.NotNil(t, sub.CancelledAt)
}

func TestSubscription_TrialToExpired(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)

	err := sub.Expire()

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_ActiveToPastDue(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())

	err := sub.MarkPastDue()

	require.NoError(t, err)
	assert.Equal(t, StatusPastDue, sub.Status)
}

func TestSubscription_ActiveToPaused(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())

	err := sub.Pause()

	require.NoError(t, err)
	assert.Equal(t, StatusPaused, sub.Status)
	assert.NotNil(t, sub.PausedAt)
}

func TestSubscription_ActiveToCancelled(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())

	err := sub.Cancel()

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
	assert.NotNil(t, sub.CancelledAt)
}

func TestSubscription_ActiveToExpired(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())

	err := sub.Expire()

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_PastDueToActive(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())
	require.NoError(t, sub.MarkPastDue())

	err := sub.Activate()

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
}

func TestSubscription_PastDueToCancelled(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())
	require.NoError(t, sub.MarkPastDue())

	err := sub.Cancel()

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
}

func TestSubscription_PastDueToExpired(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())
	require.NoError(t, sub.MarkPastDue())

	err := sub.Expire()

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_PausedToActive_Resume(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())
	require.NoError(t, sub.Pause())

	err := sub.Resume()

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
	assert.Nil(t, sub.PausedAt)
}

func TestSubscription_PausedToCancelled(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())
	require.NoError(t, sub.Pause())

	err := sub.Cancel()

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
}

func TestSubscription_PausedToExpired(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())
	require.NoError(t, sub.Pause())

	err := sub.Expire()

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_CancelledToActive_Invalid(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Cancel())

	err := sub.Activate()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.Equal(t, StatusCancelled, sub.Status)
}

func TestSubscription_CancelledToAny_Terminal(t *testing.T) {
	tests := []struct {
		name string
		fn   func(s *Subscription) error
	}{
		{"Activate", func(s *Subscription) error { return s.Activate() }},
		{"MarkPastDue", func(s *Subscription) error { return s.MarkPastDue() }},
		{"Pause", func(s *Subscription) error { return s.Pause() }},
		{"Resume", func(s *Subscription) error { return s.Resume() }},
		{"Expire", func(s *Subscription) error { return s.Expire() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
			require.NoError(t, sub.Cancel())

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
		{"Activate", func(s *Subscription) error { return s.Activate() }},
		{"MarkPastDue", func(s *Subscription) error { return s.MarkPastDue() }},
		{"Cancel", func(s *Subscription) error { return s.Cancel() }},
		{"Pause", func(s *Subscription) error { return s.Pause() }},
		{"Resume", func(s *Subscription) error { return s.Resume() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
			require.NoError(t, sub.Expire())

			err := tt.fn(sub)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidTransition)
		})
	}
}

func TestSubscription_Renew(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())

	oldEnd := sub.Period.End
	newPeriod := vo.NewBillingPeriod(oldEnd, vo.IntervalMonth)

	err := sub.Renew(newPeriod)

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
	assert.Equal(t, newPeriod.Start, sub.Period.Start)
	assert.Equal(t, newPeriod.End, sub.Period.End)
}

func TestSubscription_Renew_NotActive(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	// Still in trial
	newPeriod := vo.NewBillingPeriod(time.Now(), vo.IntervalMonth)

	err := sub.Renew(newPeriod)

	require.Error(t, err)
	assert.ErrorContains(t, err, "active")
}

func TestSubscription_CanTransitionTo(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)

	assert.True(t, sub.CanTransitionTo(StatusActive))
	assert.True(t, sub.CanTransitionTo(StatusCancelled))
	assert.True(t, sub.CanTransitionTo(StatusExpired))
	assert.False(t, sub.CanTransitionTo(StatusPaused))
	assert.False(t, sub.CanTransitionTo(StatusPastDue))
}

func TestSubscription_Cancel_SetsCancelledAt(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())

	before := time.Now()
	err := sub.Cancel()
	require.NoError(t, err)

	assert.NotNil(t, sub.CancelledAt)
	assert.False(t, sub.CancelledAt.Before(before))
}

func TestSubscription_Pause_SetsPausedAt(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())

	before := time.Now()
	err := sub.Pause()
	require.NoError(t, err)

	assert.NotNil(t, sub.PausedAt)
	assert.False(t, sub.PausedAt.Before(before))
}

func TestSubscription_Resume_ClearsPausedAt(t *testing.T) {
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil)
	require.NoError(t, sub.Activate())
	require.NoError(t, sub.Pause())
	assert.NotNil(t, sub.PausedAt)

	err := sub.Resume()
	require.NoError(t, err)

	assert.Nil(t, sub.PausedAt)
}
