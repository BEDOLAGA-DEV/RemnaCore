package aggregate_test

import (
	"testing"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newActiveSub creates a subscription in StatusActive for test convenience.
func newActiveSub(t *testing.T, now time.Time) *aggregate.Subscription {
	t.Helper()
	sub, err := aggregate.NewSubscription("user-1", "plan-old", vo.IntervalMonth, nil, now)
	require.NoError(t, err)
	require.NoError(t, sub.Activate(now))
	// drain creation + activation events so tests only see events they care about
	sub.DomainEvents()
	return sub
}

func TestAllSubscriptionStatusesHaveTransitions(t *testing.T) {
	err := aggregate.ValidateSubscriptionTransitions()
	require.NoError(t, err)
}

func TestSubscription_Upgrade(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	newPeriod := vo.NewBillingPeriod(now, vo.IntervalYear)

	t.Run("active subscription upgrades immediately", func(t *testing.T) {
		sub := newActiveSub(t, now)

		err := sub.Upgrade("plan-premium", newPeriod, now)

		require.NoError(t, err)
		assert.Equal(t, "plan-premium", sub.PlanID)
		assert.Equal(t, newPeriod, sub.Period)
		assert.Equal(t, now, sub.UpdatedAt)

		events := sub.DomainEvents()
		require.Len(t, events, 1)
		assert.Equal(t, aggregate.EventSubUpgraded, events[0].Type)

		payload, ok := events[0].Data.(aggregate.SubUpgradedPayload)
		require.True(t, ok)
		assert.Equal(t, "plan-old", payload.FromPlanID)
		assert.Equal(t, "plan-premium", payload.ToPlanID)
		assert.Equal(t, sub.ID, payload.SubscriptionID)
		assert.Equal(t, "user-1", payload.UserID)
	})

	t.Run("inactive subscription cannot upgrade", func(t *testing.T) {
		sub, err := aggregate.NewSubscription("user-1", "plan-old", vo.IntervalMonth, nil, now)
		require.NoError(t, err)
		// still in trial — not active
		sub.DomainEvents()

		err = sub.Upgrade("plan-premium", newPeriod, now)

		assert.ErrorIs(t, err, aggregate.ErrInvalidTransition)
		assert.Equal(t, "plan-old", sub.PlanID, "plan must not change on failure")
	})

	t.Run("upgrade to same plan returns ErrSamePlan", func(t *testing.T) {
		sub := newActiveSub(t, now)

		err := sub.Upgrade("plan-old", newPeriod, now)

		assert.ErrorIs(t, err, aggregate.ErrSamePlan)
	})
}

func TestSubscription_Downgrade(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	t.Run("active subscription schedules downgrade", func(t *testing.T) {
		sub := newActiveSub(t, now)
		downgradeTime := now.Add(time.Hour)

		err := sub.Downgrade("plan-basic", downgradeTime)

		require.NoError(t, err)
		assert.False(t, sub.PendingChange.IsZero())
		assert.Equal(t, "plan-basic", sub.PendingChange.PlanID)
		assert.Equal(t, "plan-old", sub.PendingChange.OriginalPlanID)
		assert.Equal(t, downgradeTime, sub.PendingChange.RequestedAt)
		assert.Equal(t, "plan-old", sub.PlanID, "current plan must not change yet")
		assert.Equal(t, downgradeTime, sub.UpdatedAt)

		events := sub.DomainEvents()
		require.Len(t, events, 1)
		assert.Equal(t, aggregate.EventSubDowngraded, events[0].Type)

		payload, ok := events[0].Data.(aggregate.SubDowngradedPayload)
		require.True(t, ok)
		assert.Equal(t, "plan-old", payload.FromPlanID)
		assert.Equal(t, "plan-basic", payload.ToPlanID)
	})

	t.Run("inactive subscription cannot downgrade", func(t *testing.T) {
		sub, err := aggregate.NewSubscription("user-1", "plan-old", vo.IntervalMonth, nil, now)
		require.NoError(t, err)
		sub.DomainEvents()

		err = sub.Downgrade("plan-basic", now)

		assert.ErrorIs(t, err, aggregate.ErrInvalidTransition)
		assert.True(t, sub.PendingChange.IsZero())
	})

	t.Run("downgrade to same plan returns ErrSamePlan", func(t *testing.T) {
		sub := newActiveSub(t, now)

		err := sub.Downgrade("plan-old", now)

		assert.ErrorIs(t, err, aggregate.ErrSamePlan)
		assert.True(t, sub.PendingChange.IsZero())
	})
}

func TestSubscription_RenewWithPendingChange(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	t.Run("renew applies pending plan and clears it", func(t *testing.T) {
		sub := newActiveSub(t, now)
		require.NoError(t, sub.Downgrade("plan-basic", now))
		sub.DomainEvents() // drain downgrade event

		renewTime := sub.Period.End.Add(time.Second) // after period ends
		err := sub.Renew(renewTime, true)

		require.NoError(t, err)
		assert.Equal(t, "plan-basic", sub.PlanID, "plan must switch to pending plan")
		assert.True(t, sub.PendingChange.IsZero(), "pending change must be cleared")

		events := sub.DomainEvents()
		require.Len(t, events, 1)
		assert.Equal(t, aggregate.EventSubRenewed, events[0].Type)
	})

	t.Run("renew without pending plan keeps current plan", func(t *testing.T) {
		sub := newActiveSub(t, now)

		renewTime := sub.Period.End.Add(time.Second)
		err := sub.Renew(renewTime, true)

		require.NoError(t, err)
		assert.Equal(t, "plan-old", sub.PlanID, "plan must remain unchanged")
		assert.True(t, sub.PendingChange.IsZero())
	})

	t.Run("renew with inactive pending plan returns ErrPendingPlanInactive", func(t *testing.T) {
		sub := newActiveSub(t, now)
		require.NoError(t, sub.Downgrade("plan-basic", now))
		sub.DomainEvents() // drain downgrade event

		renewTime := sub.Period.End.Add(time.Second)
		err := sub.Renew(renewTime, false)

		assert.ErrorIs(t, err, aggregate.ErrPendingPlanInactive)
		assert.Equal(t, "plan-old", sub.PlanID, "plan must not change on failure")
		assert.False(t, sub.PendingChange.IsZero(), "pending change must not be cleared on failure")
		assert.Equal(t, "plan-basic", sub.PendingChange.PlanID)
	})
}

func TestSubscription_AddAddon(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	// permissiveSpec returns an AddonSpec that allows the given addon IDs with no max limit.
	permissiveSpec := func(addonIDs ...string) aggregate.AddonSpec {
		return aggregate.AddonSpec{
			AvailableAddonIDs: addonIDs,
		}
	}

	t.Run("active subscription adds addon successfully", func(t *testing.T) {
		sub := newActiveSub(t, now)
		spec := permissiveSpec("addon-1", "addon-2")

		err := sub.AddAddon("addon-1", spec, now)

		require.NoError(t, err)
		assert.Equal(t, []string{"addon-1"}, sub.AddonIDs)
		assert.Equal(t, now, sub.UpdatedAt)

		events := sub.DomainEvents()
		require.Len(t, events, 1)
		assert.Equal(t, aggregate.EventSubUpdated, events[0].Type)
	})

	t.Run("trial subscription adds addon successfully", func(t *testing.T) {
		sub, err := aggregate.NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, now)
		require.NoError(t, err)
		sub.DomainEvents() // drain creation event
		spec := permissiveSpec("addon-1")

		err = sub.AddAddon("addon-1", spec, now)

		require.NoError(t, err)
		assert.Equal(t, []string{"addon-1"}, sub.AddonIDs)
	})

	t.Run("cancelled subscription cannot add addon", func(t *testing.T) {
		sub := newActiveSub(t, now)
		require.NoError(t, sub.Cancel(now))
		sub.DomainEvents()
		spec := permissiveSpec("addon-1")

		err := sub.AddAddon("addon-1", spec, now)

		require.Error(t, err)
		assert.ErrorIs(t, err, aggregate.ErrSubscriptionNotActiveForAddon)
	})

	t.Run("expired subscription cannot add addon", func(t *testing.T) {
		sub := newActiveSub(t, now)
		require.NoError(t, sub.Expire(now))
		sub.DomainEvents()
		spec := permissiveSpec("addon-1")

		err := sub.AddAddon("addon-1", spec, now)

		require.Error(t, err)
		assert.ErrorIs(t, err, aggregate.ErrSubscriptionNotActiveForAddon)
	})

	t.Run("paused subscription cannot add addon", func(t *testing.T) {
		sub := newActiveSub(t, now)
		require.NoError(t, sub.Pause(now))
		sub.DomainEvents()
		spec := permissiveSpec("addon-1")

		err := sub.AddAddon("addon-1", spec, now)

		require.Error(t, err)
		assert.ErrorIs(t, err, aggregate.ErrSubscriptionNotActiveForAddon)
	})

	t.Run("empty addon ID returns ErrEmptyAddonID", func(t *testing.T) {
		sub := newActiveSub(t, now)
		spec := permissiveSpec("addon-1")

		err := sub.AddAddon("", spec, now)

		require.Error(t, err)
		assert.ErrorIs(t, err, aggregate.ErrEmptyAddonID)
	})

	t.Run("duplicate addon returns ErrAddonAlreadyOnSubscription", func(t *testing.T) {
		sub := newActiveSub(t, now)
		spec := permissiveSpec("addon-1")
		require.NoError(t, sub.AddAddon("addon-1", spec, now))
		sub.DomainEvents()

		err := sub.AddAddon("addon-1", spec, now)

		require.Error(t, err)
		assert.ErrorIs(t, err, aggregate.ErrAddonAlreadyOnSubscription)
	})

	t.Run("addon not on plan returns ErrAddonNotAvailableOnPlan", func(t *testing.T) {
		sub := newActiveSub(t, now)
		spec := permissiveSpec("addon-1", "addon-2")

		err := sub.AddAddon("addon-3", spec, now)

		require.Error(t, err)
		assert.ErrorIs(t, err, aggregate.ErrAddonNotAvailableOnPlan)
	})

	t.Run("max addons exceeded returns ErrMaxAddonsExceeded", func(t *testing.T) {
		sub := newActiveSub(t, now)
		spec := aggregate.AddonSpec{
			AvailableAddonIDs: []string{"addon-1", "addon-2", "addon-3"},
			MaxAddons:         2,
		}
		require.NoError(t, sub.AddAddon("addon-1", spec, now))
		require.NoError(t, sub.AddAddon("addon-2", spec, now))
		sub.DomainEvents()

		err := sub.AddAddon("addon-3", spec, now)

		require.Error(t, err)
		assert.ErrorIs(t, err, aggregate.ErrMaxAddonsExceeded)
		assert.Len(t, sub.AddonIDs, 2, "addon must not be added on failure")
	})
}

func TestSubscription_Upgrade_ClearsPendingChange(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	newPeriod := vo.NewBillingPeriod(now, vo.IntervalYear)

	t.Run("downgrade then upgrade clears pending and renew keeps upgraded plan", func(t *testing.T) {
		sub := newActiveSub(t, now)

		// Schedule a deferred downgrade.
		require.NoError(t, sub.Downgrade("plan-basic", now))
		assert.False(t, sub.PendingChange.IsZero())
		assert.Equal(t, "plan-basic", sub.PendingChange.PlanID)
		sub.DomainEvents() // drain

		// Upgrade overrides the pending downgrade.
		upgradeTime := now.Add(time.Hour)
		require.NoError(t, sub.Upgrade("plan-premium", newPeriod, upgradeTime))
		assert.True(t, sub.PendingChange.IsZero(), "upgrade must clear pending change")
		assert.Equal(t, "plan-premium", sub.PlanID)
		sub.DomainEvents() // drain

		// Renew should keep the upgraded plan, not apply the old downgrade.
		renewTime := sub.Period.End.Add(time.Second)
		require.NoError(t, sub.Renew(renewTime, true))

		assert.Equal(t, "plan-premium", sub.PlanID, "plan must remain upgraded after renew")
		assert.True(t, sub.PendingChange.IsZero())
	})
}

func TestSubscription_ValidateForInvoicing(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		status  aggregate.SubscriptionStatus
		wantErr error
	}{
		{"trial allows invoicing", aggregate.StatusTrial, nil},
		{"active allows invoicing", aggregate.StatusActive, nil},
		{"past_due allows invoicing", aggregate.StatusPastDue, nil},
		{"paused allows invoicing", aggregate.StatusPaused, nil},
		{"cancelled rejects invoicing", aggregate.StatusCancelled, aggregate.ErrSubscriptionNotActiveForInvoicing},
		{"expired rejects invoicing", aggregate.StatusExpired, aggregate.ErrSubscriptionNotActiveForInvoicing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, err := aggregate.NewSubscription("user-1", "plan-old", vo.IntervalMonth, nil, now)
			require.NoError(t, err)

			// Force status for test. Trial is the default from NewSubscription;
			// for other statuses we transition through the state machine.
			switch tt.status {
			case aggregate.StatusActive:
				require.NoError(t, sub.Activate(now))
			case aggregate.StatusPastDue:
				require.NoError(t, sub.Activate(now))
				require.NoError(t, sub.MarkPastDue(now))
			case aggregate.StatusPaused:
				require.NoError(t, sub.Activate(now))
				require.NoError(t, sub.Pause(now))
			case aggregate.StatusCancelled:
				require.NoError(t, sub.Activate(now))
				require.NoError(t, sub.Cancel(now))
			case aggregate.StatusExpired:
				require.NoError(t, sub.Activate(now))
				require.NoError(t, sub.Expire(now))
			}

			err = sub.ValidateForInvoicing()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
