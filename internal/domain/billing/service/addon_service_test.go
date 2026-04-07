package service

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/billingtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

func newTestAddonService() (
	*AddonService,
	*billingtest.MockSubscriptionRepo,
	*billingtest.MockPlanRepo,
	*billingtest.MockEventPublisher,
) {
	subs := &billingtest.MockSubscriptionRepo{}
	plans := &billingtest.MockPlanRepo{}
	publisher := &billingtest.MockEventPublisher{}
	txRunner := billingtest.NoopTxRunner{}
	clk := clock.NewReal()

	svc := NewAddonService(subs, plans, publisher, txRunner, clk, slog.Default())
	return svc, subs, plans, publisher
}

// --- AddSubscriptionAddon ---

func TestAddonService_AddSubscriptionAddon_Success(t *testing.T) {
	svc, subs, plans, publisher := newTestAddonService()
	ctx := context.Background()

	sub := activeSubscription("user-1", "plan-premium")
	plan := samplePlan()

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.AddSubscriptionAddon(ctx, "sub-1", "addon-traffic")

	require.NoError(t, err)
	assert.Contains(t, sub.AddonIDs, "addon-traffic")

	subs.AssertExpectations(t)
	plans.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestAddonService_AddSubscriptionAddon_NotFound(t *testing.T) {
	svc, subs, _, _ := newTestAddonService()
	ctx := context.Background()

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(nil, billing.ErrSubscriptionNotFound)

	err := svc.AddSubscriptionAddon(ctx, "sub-1", "addon-traffic")

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrSubscriptionNotFound)

	subs.AssertExpectations(t)
}

// --- RemoveSubscriptionAddon ---

func TestAddonService_RemoveSubscriptionAddon_Success(t *testing.T) {
	svc, subs, _, publisher := newTestAddonService()
	ctx := context.Background()

	sub := activeSubscription("user-1", "plan-premium")
	sub.AddonIDs = []string{"addon-traffic"}

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.RemoveSubscriptionAddon(ctx, "sub-1", "addon-traffic")

	require.NoError(t, err)
	assert.NotContains(t, sub.AddonIDs, "addon-traffic")

	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestAddonService_RemoveSubscriptionAddon_NotOnSubscription(t *testing.T) {
	svc, subs, _, _ := newTestAddonService()
	ctx := context.Background()

	sub := activeSubscription("user-1", "plan-premium")
	// No addons on subscription.

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)

	err := svc.RemoveSubscriptionAddon(ctx, "sub-1", "addon-traffic")

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrAddonNotOnSubscription)

	subs.AssertExpectations(t)
}
