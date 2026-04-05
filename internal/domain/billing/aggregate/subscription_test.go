package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

// mustNewSubscription is a test helper that creates a valid subscription or
// fails the test immediately. It keeps existing test code concise.
func mustNewSubscription(t *testing.T, userID, planID string, interval vo.BillingInterval, addonIDs []string, now time.Time) *Subscription {
	t.Helper()
	sub, err := NewSubscription(userID, planID, interval, addonIDs, now)
	require.NoError(t, err)
	return sub
}

func TestNewSubscription(t *testing.T) {
	sub, err := NewSubscription("user-1", "plan-1", vo.IntervalMonth, []string{"addon-1"}, time.Now())

	require.NoError(t, err)
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

func TestNewSubscription_EmptyUserID(t *testing.T) {
	sub, err := NewSubscription("", "plan-1", vo.IntervalMonth, nil, time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyUserID)
	assert.Nil(t, sub)
}

func TestNewSubscription_EmptyPlanID(t *testing.T) {
	sub, err := NewSubscription("user-1", "", vo.IntervalMonth, nil, time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrEmptyPlanID)
	assert.Nil(t, sub)
}

func TestSubscription_TrialToActive(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	assert.Equal(t, StatusTrial, sub.Status)

	err := sub.Activate(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
}

func TestSubscription_TrialToPaused_Invalid(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())

	err := sub.Pause(time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidTransition)
	assert.Equal(t, StatusTrial, sub.Status)
}

func TestSubscription_TrialToCancelled(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())

	err := sub.Cancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
	assert.NotNil(t, sub.CancelledAt)
}

func TestSubscription_TrialToExpired(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())

	err := sub.Expire(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_ActiveToPastDue(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	err := sub.MarkPastDue(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusPastDue, sub.Status)
}

func TestSubscription_ActiveToPaused(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	err := sub.Pause(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusPaused, sub.Status)
	assert.NotNil(t, sub.PausedAt)
}

func TestSubscription_ActiveToCancelled(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	err := sub.Cancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
	assert.NotNil(t, sub.CancelledAt)
}

func TestSubscription_ActiveToExpired(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	err := sub.Expire(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_PastDueToActive(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.MarkPastDue(time.Now()))

	err := sub.Activate(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
}

func TestSubscription_PastDueToCancelled(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.MarkPastDue(time.Now()))

	err := sub.Cancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
}

func TestSubscription_PastDueToExpired(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.MarkPastDue(time.Now()))

	err := sub.Expire(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_PausedToActive_Resume(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.Pause(time.Now()))

	err := sub.Resume(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
	assert.Nil(t, sub.PausedAt)
}

func TestSubscription_PausedToCancelled(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.Pause(time.Now()))

	err := sub.Cancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusCancelled, sub.Status)
}

func TestSubscription_PausedToExpired(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.Pause(time.Now()))

	err := sub.Expire(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusExpired, sub.Status)
}

func TestSubscription_CancelledToActive_Invalid(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
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
			sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
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
			sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
			require.NoError(t, sub.Expire(time.Now()))

			err := tt.fn(sub)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidTransition)
		})
	}
}

func TestSubscription_Renew(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	oldEnd := sub.Period.End
	expectedNext := vo.NewBillingPeriod(oldEnd, vo.IntervalMonth)

	err := sub.Renew(time.Now())

	require.NoError(t, err)
	assert.Equal(t, StatusActive, sub.Status)
	assert.Equal(t, expectedNext.Start, sub.Period.Start)
	assert.Equal(t, expectedNext.End, sub.Period.End)
}

func TestSubscription_Renew_NotActive(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	// Still in trial

	err := sub.Renew(time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "active")
}

func TestSubscription_Renew_PreservesInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval vo.BillingInterval
	}{
		{"monthly", vo.IntervalMonth},
		{"quarterly", vo.IntervalQuarter},
		{"yearly", vo.IntervalYear},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := mustNewSubscription(t, "user-1", "plan-1", tt.interval, nil, time.Now())
			require.NoError(t, sub.Activate(time.Now()))

			originalEnd := sub.Period.End

			err := sub.Renew(time.Now())

			require.NoError(t, err)
			assert.Equal(t, originalEnd, sub.Period.Start)
			assert.Equal(t, tt.interval, sub.Period.Interval)
		})
	}
}

func TestSubscription_CanTransitionTo(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())

	assert.True(t, sub.CanTransitionTo(StatusActive))
	assert.True(t, sub.CanTransitionTo(StatusCancelled))
	assert.True(t, sub.CanTransitionTo(StatusExpired))
	assert.False(t, sub.CanTransitionTo(StatusPaused))
	assert.False(t, sub.CanTransitionTo(StatusPastDue))
}

func TestSubscription_Cancel_SetsCancelledAt(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	before := time.Now()
	err := sub.Cancel(time.Now())
	require.NoError(t, err)

	assert.NotNil(t, sub.CancelledAt)
	assert.False(t, sub.CancelledAt.Before(before))
}

func TestSubscription_Pause_SetsPausedAt(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))

	before := time.Now()
	err := sub.Pause(time.Now())
	require.NoError(t, err)

	assert.NotNil(t, sub.PausedAt)
	assert.False(t, sub.PausedAt.Before(before))
}

func TestSubscription_Resume_ClearsPausedAt(t *testing.T) {
	sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, nil, time.Now())
	require.NoError(t, sub.Activate(time.Now()))
	require.NoError(t, sub.Pause(time.Now()))
	assert.NotNil(t, sub.PausedAt)

	err := sub.Resume(time.Now())
	require.NoError(t, err)

	assert.Nil(t, sub.PausedAt)
}

func TestSubscription_AddAddon(t *testing.T) {
	tests := []struct {
		name      string
		initial   []string
		addonID   string
		wantErr   error
		wantAddons []string
	}{
		{
			name:       "adds addon to empty list",
			initial:    nil,
			addonID:    "addon-1",
			wantAddons: []string{"addon-1"},
		},
		{
			name:       "adds addon to existing list",
			initial:    []string{"addon-1"},
			addonID:    "addon-2",
			wantAddons: []string{"addon-1", "addon-2"},
		},
		{
			name:    "rejects duplicate addon",
			initial: []string{"addon-1"},
			addonID: "addon-1",
			wantErr: ErrAddonAlreadyOnSubscription,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, tt.initial, now)
			// Drain creation events so we can verify addon events in isolation.
			sub.DomainEvents()

			addonTime := now.Add(time.Second)
			err := sub.AddAddon(tt.addonID, addonTime)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantAddons, sub.AddonIDs)
			assert.Equal(t, addonTime, sub.UpdatedAt)

			events := sub.DomainEvents()
			require.Len(t, events, 1)
			assert.Equal(t, EventSubUpdated, events[0].Type)
		})
	}
}

func TestSubscription_RemoveAddon(t *testing.T) {
	tests := []struct {
		name       string
		initial    []string
		addonID    string
		wantErr    error
		wantAddons []string
	}{
		{
			name:       "removes existing addon",
			initial:    []string{"addon-1", "addon-2"},
			addonID:    "addon-1",
			wantAddons: []string{"addon-2"},
		},
		{
			name:       "removes last addon",
			initial:    []string{"addon-1"},
			addonID:    "addon-1",
			wantAddons: []string{},
		},
		{
			name:    "rejects missing addon",
			initial: []string{"addon-1"},
			addonID: "addon-99",
			wantErr: ErrAddonNotOnSubscription,
		},
		{
			name:    "rejects on empty list",
			initial: nil,
			addonID: "addon-1",
			wantErr: ErrAddonNotOnSubscription,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			sub := mustNewSubscription(t, "user-1", "plan-1", vo.IntervalMonth, tt.initial, now)
			// Drain creation events so we can verify addon events in isolation.
			sub.DomainEvents()

			removeTime := now.Add(time.Second)
			err := sub.RemoveAddon(tt.addonID, removeTime)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantAddons, sub.AddonIDs)
			assert.Equal(t, removeTime, sub.UpdatedAt)

			events := sub.DomainEvents()
			require.Len(t, events, 1)
			assert.Equal(t, EventSubUpdated, events[0].Type)
		})
	}
}
