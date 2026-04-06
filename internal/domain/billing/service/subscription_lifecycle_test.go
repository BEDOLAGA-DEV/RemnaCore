package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/billingtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// --- Mock dispatcher ---

type mockDispatcher struct {
	mock.Mock
}

func (m *mockDispatcher) DispatchSync(ctx context.Context, hookName string, payload json.RawMessage) (json.RawMessage, error) {
	args := m.Called(ctx, hookName, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(json.RawMessage), args.Error(1)
}

func (m *mockDispatcher) DispatchSyncVersioned(ctx context.Context, hookName string, currentVersion int, payload json.RawMessage) (json.RawMessage, error) {
	args := m.Called(ctx, hookName, currentVersion, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(json.RawMessage), args.Error(1)
}

func (m *mockDispatcher) DispatchSyncSafe(ctx context.Context, hookName string, payload json.RawMessage) *hookdispatch.ChainResult {
	args := m.Called(ctx, hookName, payload)
	return args.Get(0).(*hookdispatch.ChainResult)
}

func (m *mockDispatcher) DispatchAsync(ctx context.Context, hookName string, payload json.RawMessage) {
	m.Called(ctx, hookName, payload)
}

func (m *mockDispatcher) BeginFlow(ctx context.Context) context.Context {
	m.Called(ctx)
	return ctx
}

// --- Helpers ---

// newTestServiceWithHooks creates a BillingService with hooks enabled and a mock dispatcher.
func newTestServiceWithHooks() (
	*BillingService,
	*billingtest.MockPlanRepo,
	*billingtest.MockSubscriptionRepo,
	*billingtest.MockInvoiceRepo,
	*billingtest.MockFamilyRepo,
	*billingtest.MockEventPublisher,
	*mockDispatcher,
) {
	plans := &billingtest.MockPlanRepo{}
	subs := &billingtest.MockSubscriptionRepo{}
	invoices := &billingtest.MockInvoiceRepo{}
	families := &billingtest.MockFamilyRepo{}
	publisher := &billingtest.MockEventPublisher{}
	prorate := NewProrateCalculator()
	trial := NewTrialManager(DefaultTrialDays)
	txRunner := billingtest.NoopTxRunner{}
	clk := clock.NewReal()
	dispatcher := &mockDispatcher{}

	svc := NewBillingService(
		plans, subs, invoices, families, publisher, prorate, trial, txRunner, clk, slog.Default(),
		WithDispatcher(dispatcher),
		WithHooksEnabled(true),
	)
	return svc, plans, subs, invoices, families, publisher, dispatcher
}

// expiredActiveSub returns an active subscription whose billing period has already ended.
func expiredActiveSub(userID, planID string) *aggregate.Subscription {
	pastStart := time.Now().AddDate(0, -2, 0)
	return &aggregate.Subscription{
		ID:        "sub-1",
		UserID:    userID,
		PlanID:    planID,
		Status:    aggregate.StatusActive,
		Period:    vo.NewBillingPeriod(pastStart, vo.IntervalMonth),
		CreatedAt: pastStart,
		UpdatedAt: pastStart,
	}
}

func pausedSubscription(userID, planID string) *aggregate.Subscription {
	now := time.Now()
	pausedAt := now
	return &aggregate.Subscription{
		ID:        "sub-1",
		UserID:    userID,
		PlanID:    planID,
		Status:    aggregate.StatusPaused,
		Period:    vo.NewBillingPeriod(now, vo.IntervalMonth),
		PausedAt:  &pausedAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// --- PauseSubscription ---

func TestPauseSubscription_Success(t *testing.T) {
	svc, _, subs, _, _, publisher := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.PauseSubscription(ctx, "sub-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.StatusPaused, sub.Status)
	assert.NotNil(t, sub.PausedAt)

	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestPauseSubscription_AlreadyPaused(t *testing.T) {
	svc, _, subs, _, _, _ := newTestBillingService()
	ctx := context.Background()
	sub := pausedSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)

	err := svc.PauseSubscription(ctx, "sub-1")

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrInvalidTransition)

	subs.AssertExpectations(t)
}

func TestPauseSubscription_FiresAsyncHook(t *testing.T) {
	svc, _, subs, _, _, publisher, dispatcher := newTestServiceWithHooks()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)
	dispatcher.On("DispatchAsync", mock.Anything, HookSubPausedPost, mock.AnythingOfType("json.RawMessage")).Once()

	err := svc.PauseSubscription(ctx, "sub-1")

	require.NoError(t, err)
	dispatcher.AssertExpectations(t)
}

// --- ResumeSubscription ---

func TestResumeSubscription_Success(t *testing.T) {
	svc, _, subs, _, _, publisher := newTestBillingService()
	ctx := context.Background()
	sub := pausedSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.ResumeSubscription(ctx, "sub-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.StatusActive, sub.Status)
	assert.Nil(t, sub.PausedAt)

	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestResumeSubscription_NotPaused(t *testing.T) {
	svc, _, subs, _, _, _ := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)

	err := svc.ResumeSubscription(ctx, "sub-1")

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrInvalidTransition)

	subs.AssertExpectations(t)
}

// --- RenewSubscription ---

func TestRenewSubscription_Success(t *testing.T) {
	svc, plans, subs, invoices, _, publisher := newTestBillingService()
	ctx := context.Background()
	sub := expiredActiveSub("user-1", "plan-premium")
	plan := samplePlan()

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.RenewSubscription(ctx, "sub-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.StatusActive, sub.Status)

	subs.AssertExpectations(t)
	plans.AssertExpectations(t)
	invoices.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestRenewSubscription_NotActive(t *testing.T) {
	svc, _, subs, _, _, _ := newTestBillingService()
	ctx := context.Background()
	sub := pausedSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)

	err := svc.RenewSubscription(ctx, "sub-1")

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrSubscriptionNotActiveForRenewal)

	subs.AssertExpectations(t)
}

func TestRenewSubscription_PeriodNotElapsed(t *testing.T) {
	svc, _, subs, _, _, _ := newTestBillingService()
	ctx := context.Background()
	// Active subscription with future billing period end
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)

	err := svc.RenewSubscription(ctx, "sub-1")

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrPeriodNotElapsed)

	subs.AssertExpectations(t)
}

func TestRenewSubscription_WithRenewingHook(t *testing.T) {
	svc, plans, subs, invoices, _, publisher, dispatcher := newTestServiceWithHooks()
	ctx := context.Background()
	sub := expiredActiveSub("user-1", "plan-premium")
	plan := samplePlan()

	// Hook returns a price override.
	hookResp := sdk.SubRenewingResponse{PriceOverride: ptr(int64(799))}
	hookRespBytes, _ := json.Marshal(hookResp)

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	dispatcher.On("DispatchSyncSafe", mock.Anything, HookSubRenewing, mock.Anything).Return(&hookdispatch.ChainResult{
		Payload: hookRespBytes,
	})
	subs.On("Update", mock.Anything, sub).Return(nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)
	dispatcher.On("DispatchAsync", mock.Anything, HookSubRenewedPost, mock.AnythingOfType("json.RawMessage")).Once()

	err := svc.RenewSubscription(ctx, "sub-1")

	require.NoError(t, err)
	dispatcher.AssertExpectations(t)
}

// --- UpgradeSubscription ---

func TestUpgradeSubscription_Success(t *testing.T) {
	svc, plans, subs, invoices, _, publisher := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-basic")
	sub.PlanID = "plan-basic"

	basicPlan := &aggregate.Plan{
		ID:        "plan-basic",
		Name:      "Basic VPN",
		BasePrice: vo.NewMoney(499, vo.CurrencyUSD),
		Interval:  vo.IntervalMonth,
		IsActive:  true,
	}
	premiumPlan := samplePlan()

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	plans.On("GetByID", mock.Anything, "plan-basic").Return(basicPlan, nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(premiumPlan, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.UpgradeSubscription(ctx, "sub-1", "plan-premium")

	require.NoError(t, err)
	assert.Equal(t, "plan-premium", sub.PlanID)
	assert.Nil(t, sub.PendingPlanID)

	subs.AssertExpectations(t)
	plans.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestUpgradeSubscription_SamePlan(t *testing.T) {
	svc, plans, subs, _, _, _ := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	currentPlan := samplePlan()

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(currentPlan, nil).Once()
	// newPlanID == current planID -> same plan
	plans.On("GetByID", mock.Anything, "plan-premium").Return(currentPlan, nil).Once()

	err := svc.UpgradeSubscription(ctx, "sub-1", "plan-premium")

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrSamePlan)

	subs.AssertExpectations(t)
}

func TestUpgradeSubscription_WithCreditOverrideHook(t *testing.T) {
	svc, plans, subs, invoices, _, publisher, dispatcher := newTestServiceWithHooks()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-basic")
	sub.PlanID = "plan-basic"

	basicPlan := &aggregate.Plan{
		ID:        "plan-basic",
		Name:      "Basic VPN",
		BasePrice: vo.NewMoney(499, vo.CurrencyUSD),
		Interval:  vo.IntervalMonth,
		IsActive:  true,
	}
	premiumPlan := samplePlan()

	// Hook overrides credit to zero.
	hookResp := sdk.SubUpgradingResponse{CreditOverride: ptr(int64(0))}
	hookRespBytes, _ := json.Marshal(hookResp)

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	plans.On("GetByID", mock.Anything, "plan-basic").Return(basicPlan, nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(premiumPlan, nil)
	dispatcher.On("DispatchSyncSafe", mock.Anything, HookSubUpgrading, mock.Anything).Return(&hookdispatch.ChainResult{
		Payload: hookRespBytes,
	})
	subs.On("Update", mock.Anything, sub).Return(nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)
	dispatcher.On("DispatchAsync", mock.Anything, HookSubUpgradedPost, mock.AnythingOfType("json.RawMessage")).Once()

	err := svc.UpgradeSubscription(ctx, "sub-1", "plan-premium")

	require.NoError(t, err)
	dispatcher.AssertExpectations(t)
}

// --- DowngradeSubscription ---

func TestDowngradeSubscription_Success(t *testing.T) {
	svc, _, subs, _, _, publisher := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.DowngradeSubscription(ctx, "sub-1", "plan-basic")

	require.NoError(t, err)
	require.NotNil(t, sub.PendingPlanID)
	assert.Equal(t, "plan-basic", *sub.PendingPlanID)
	// PlanID unchanged until next renewal
	assert.Equal(t, "plan-premium", sub.PlanID)

	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestDowngradeSubscription_SamePlan(t *testing.T) {
	svc, _, subs, _, _, _ := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)

	err := svc.DowngradeSubscription(ctx, "sub-1", "plan-premium")

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrSamePlan)

	subs.AssertExpectations(t)
}

// --- ExpireSubscription ---

func TestExpireSubscription_Success(t *testing.T) {
	svc, _, subs, _, _, publisher := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.ExpireSubscription(ctx, "sub-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.StatusExpired, sub.Status)

	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestExpireSubscription_AlreadyExpired(t *testing.T) {
	svc, _, subs, _, _, _ := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")
	sub.Status = aggregate.StatusExpired

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)

	err := svc.ExpireSubscription(ctx, "sub-1")

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrInvalidTransition)

	subs.AssertExpectations(t)
}

func TestExpireSubscription_FiresAsyncHook(t *testing.T) {
	svc, _, subs, _, _, publisher, dispatcher := newTestServiceWithHooks()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)
	dispatcher.On("DispatchAsync", mock.Anything, HookSubExpiredPost, mock.AnythingOfType("json.RawMessage")).Once()

	err := svc.ExpireSubscription(ctx, "sub-1")

	require.NoError(t, err)
	dispatcher.AssertExpectations(t)
}

// --- CancelSubscription with hooks ---

func TestCancelSubscription_WithHook_Block(t *testing.T) {
	svc, _, subs, _, _, _, dispatcher := newTestServiceWithHooks()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	hookResp := sdk.SubCancellingResponse{Block: true}
	hookRespBytes, _ := json.Marshal(hookResp)

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	dispatcher.On("DispatchSyncSafe", mock.Anything, HookSubCancelling, mock.Anything).Return(&hookdispatch.ChainResult{
		Payload: hookRespBytes,
	})

	err := svc.CancelSubscription(ctx, "sub-1", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrCancellationBlocked)
	// Subscription should NOT be cancelled.
	assert.Equal(t, aggregate.StatusActive, sub.Status)

	subs.AssertExpectations(t)
	dispatcher.AssertExpectations(t)
}

func TestCancelSubscription_WithHook_Allowed(t *testing.T) {
	svc, _, subs, _, _, publisher, dispatcher := newTestServiceWithHooks()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	hookResp := sdk.SubCancellingResponse{Block: false}
	hookRespBytes, _ := json.Marshal(hookResp)

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	dispatcher.On("DispatchSyncSafe", mock.Anything, HookSubCancelling, mock.Anything).Return(&hookdispatch.ChainResult{
		Payload: hookRespBytes,
	})
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)
	dispatcher.On("DispatchAsync", mock.Anything, HookSubCancelledPost, mock.AnythingOfType("json.RawMessage")).Once()

	err := svc.CancelSubscription(ctx, "sub-1", nil)

	require.NoError(t, err)
	assert.Equal(t, aggregate.StatusCancelled, sub.Status)

	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
	dispatcher.AssertExpectations(t)
}

// --- Helpers ---

func ptr[T any](v T) *T {
	return &v
}
