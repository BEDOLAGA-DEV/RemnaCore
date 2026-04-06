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
		require.NotNil(t, sub.PendingPlanID)
		assert.Equal(t, "plan-basic", *sub.PendingPlanID)
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
		assert.Nil(t, sub.PendingPlanID)
	})

	t.Run("downgrade to same plan returns ErrSamePlan", func(t *testing.T) {
		sub := newActiveSub(t, now)

		err := sub.Downgrade("plan-old", now)

		assert.ErrorIs(t, err, aggregate.ErrSamePlan)
		assert.Nil(t, sub.PendingPlanID)
	})
}

func TestSubscription_RenewWithPendingPlanID(t *testing.T) {
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	t.Run("renew applies pending plan and clears it", func(t *testing.T) {
		sub := newActiveSub(t, now)
		require.NoError(t, sub.Downgrade("plan-basic", now))
		sub.DomainEvents() // drain downgrade event

		renewTime := sub.Period.End.Add(time.Second) // after period ends
		err := sub.Renew(renewTime)

		require.NoError(t, err)
		assert.Equal(t, "plan-basic", sub.PlanID, "plan must switch to pending plan")
		assert.Nil(t, sub.PendingPlanID, "pending plan must be cleared")

		events := sub.DomainEvents()
		require.Len(t, events, 1)
		assert.Equal(t, aggregate.EventSubRenewed, events[0].Type)
	})

	t.Run("renew without pending plan keeps current plan", func(t *testing.T) {
		sub := newActiveSub(t, now)

		renewTime := sub.Period.End.Add(time.Second)
		err := sub.Renew(renewTime)

		require.NoError(t, err)
		assert.Equal(t, "plan-old", sub.PlanID, "plan must remain unchanged")
		assert.Nil(t, sub.PendingPlanID)
	})
}
