package service

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/billingtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

func newTestFamilyService() (
	*FamilyService,
	*billingtest.MockFamilyRepo,
	*billingtest.MockSubscriptionRepo,
	*billingtest.MockPlanRepo,
	*billingtest.MockEventPublisher,
) {
	families := &billingtest.MockFamilyRepo{}
	subs := &billingtest.MockSubscriptionRepo{}
	plans := &billingtest.MockPlanRepo{}
	publisher := &billingtest.MockEventPublisher{}
	txRunner := billingtest.NoopTxRunner{}
	clk := clock.NewReal()

	svc := NewFamilyService(families, subs, plans, publisher, txRunner, clk, slog.Default())
	return svc, families, subs, plans, publisher
}

// --- AddFamilyMember ---

func TestFamilyService_AddFamilyMember_Success(t *testing.T) {
	svc, families, subs, plans, publisher := newTestFamilyService()
	ctx := context.Background()

	plan := samplePlan()
	sub := activeSubscription("user-1", "plan-premium")

	fg, fgErr := aggregate.NewFamilyGroup("user-1", 5, time.Now())
	require.NoError(t, fgErr)

	subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	families.On("GetByOwnerIDForUpdate", mock.Anything, "user-1").Return(fg, nil)
	families.On("Update", mock.Anything, fg).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.AddFamilyMember(ctx, "sub-1", "member-1", "Alice")

	require.NoError(t, err)
	assert.True(t, fg.HasMember("member-1"))

	subs.AssertExpectations(t)
	plans.AssertExpectations(t)
	families.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestFamilyService_AddFamilyMember_FamilyNotEnabled(t *testing.T) {
	svc, families, subs, plans, _ := newTestFamilyService()
	ctx := context.Background()

	plan := samplePlan()
	plan.FamilyEnabled = false
	sub := activeSubscription("user-1", "plan-premium")

	fg, fgErr := aggregate.NewFamilyGroup("user-1", 5, time.Now())
	require.NoError(t, fgErr)

	subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	families.On("GetByOwnerIDForUpdate", mock.Anything, "user-1").Return(fg, nil)

	err := svc.AddFamilyMember(ctx, "sub-1", "member-1", "Alice")

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrFamilyNotEnabled)

	subs.AssertExpectations(t)
	plans.AssertExpectations(t)
	families.AssertExpectations(t)
}

func TestFamilyService_AddFamilyMember_CreatesGroupIfNotExists(t *testing.T) {
	svc, families, subs, plans, publisher := newTestFamilyService()
	ctx := context.Background()

	plan := samplePlan()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	families.On("GetByOwnerIDForUpdate", mock.Anything, "user-1").Return(nil, billing.ErrFamilyGroupNotFound)
	families.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.FamilyGroup")).Return(nil)
	families.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.FamilyGroup")).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.AddFamilyMember(ctx, "sub-1", "member-1", "Alice")

	require.NoError(t, err)

	families.AssertCalled(t, "Create", mock.Anything, mock.AnythingOfType("*aggregate.FamilyGroup"))
	families.AssertExpectations(t)
}

// --- RemoveFamilyMember ---

func TestFamilyService_RemoveFamilyMember_Success(t *testing.T) {
	svc, families, subs, _, publisher := newTestFamilyService()
	ctx := context.Background()

	sub := activeSubscription("user-1", "plan-premium")
	fg, fgErr := aggregate.NewFamilyGroup("user-1", 5, time.Now())
	require.NoError(t, fgErr)
	require.NoError(t, fg.AddMember("member-1", "Alice", time.Now()))

	subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil)
	families.On("GetByOwnerIDForUpdate", mock.Anything, "user-1").Return(fg, nil)
	families.On("Update", mock.Anything, fg).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.RemoveFamilyMember(ctx, "sub-1", "member-1")

	require.NoError(t, err)
	assert.False(t, fg.HasMember("member-1"))

	subs.AssertExpectations(t)
	families.AssertExpectations(t)
	publisher.AssertExpectations(t)
}
