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
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// hookRecorder captures typed sync and async hook dispatch calls for test assertions.
type hookRecorder struct {
	syncCalls  []hookCall
	asyncCalls []hookCall
	// cancelResp is returned by the CancelHookFn when non-nil.
	cancelResp *CancellingResponse
	// renewResp is returned by the RenewHookFn when non-nil.
	renewResp *RenewingResponse
	// upgradeResp is returned by the UpgradeHookFn when non-nil.
	upgradeResp *UpgradingResponse
}

type hookCall struct {
	hookName string
	payload  any
}

func (r *hookRecorder) cancelHook(_ context.Context, payload CancellingPayload) (*CancellingResponse, error) {
	r.syncCalls = append(r.syncCalls, hookCall{hookName: HookSubCancelling, payload: payload})
	return r.cancelResp, nil
}

func (r *hookRecorder) renewHook(_ context.Context, payload RenewingPayload) (*RenewingResponse, error) {
	r.syncCalls = append(r.syncCalls, hookCall{hookName: HookSubRenewing, payload: payload})
	return r.renewResp, nil
}

func (r *hookRecorder) upgradeHook(_ context.Context, payload UpgradingPayload) (*UpgradingResponse, error) {
	r.syncCalls = append(r.syncCalls, hookCall{hookName: HookSubUpgrading, payload: payload})
	return r.upgradeResp, nil
}

func (r *hookRecorder) asyncHook(_ context.Context, hookName string, payload any) {
	r.asyncCalls = append(r.asyncCalls, hookCall{hookName: hookName, payload: payload})
}

// --- Helpers ---

// newTestServiceWithHooks creates a BillingService with hooks enabled and a hook recorder.
func newTestServiceWithHooks() (
	*BillingService,
	*billingtest.MockPlanRepo,
	*billingtest.MockSubscriptionRepo,
	*billingtest.MockInvoiceRepo,
	*billingtest.MockFamilyRepo,
	*billingtest.MockEventPublisher,
	*hookRecorder,
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
	recorder := &hookRecorder{}

	svc := NewBillingService(
		plans, subs, invoices, families, publisher, prorate, trial, txRunner, clk, slog.Default(),
		WithCancelHook(recorder.cancelHook),
		WithRenewHook(recorder.renewHook),
		WithUpgradeHook(recorder.upgradeHook),
		WithAsyncHook(recorder.asyncHook),
	)
	return svc, plans, subs, invoices, families, publisher, recorder
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
	svc, _, subs, _, _, publisher, recorder := newTestServiceWithHooks()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.PauseSubscription(ctx, "sub-1")

	require.NoError(t, err)
	require.Len(t, recorder.asyncCalls, 1)
	assert.Equal(t, HookSubPausedPost, recorder.asyncCalls[0].hookName)
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
	svc, plans, subs, invoices, _, publisher, recorder := newTestServiceWithHooks()
	ctx := context.Background()
	sub := expiredActiveSub("user-1", "plan-premium")
	plan := samplePlan()

	// Hook returns a price override.
	recorder.renewResp = &RenewingResponse{PriceOverride: ptr(int64(799))}

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.RenewSubscription(ctx, "sub-1")

	require.NoError(t, err)
	require.Len(t, recorder.syncCalls, 1)
	assert.Equal(t, HookSubRenewing, recorder.syncCalls[0].hookName)
	require.Len(t, recorder.asyncCalls, 1)
	assert.Equal(t, HookSubRenewedPost, recorder.asyncCalls[0].hookName)
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
	assert.True(t, sub.PendingChange.IsZero())

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
	svc, plans, subs, invoices, _, publisher, recorder := newTestServiceWithHooks()
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
	recorder.upgradeResp = &UpgradingResponse{CreditOverride: ptr(int64(0))}

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	plans.On("GetByID", mock.Anything, "plan-basic").Return(basicPlan, nil)
	plans.On("GetByID", mock.Anything, "plan-premium").Return(premiumPlan, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.UpgradeSubscription(ctx, "sub-1", "plan-premium")

	require.NoError(t, err)
	require.Len(t, recorder.syncCalls, 1)
	assert.Equal(t, HookSubUpgrading, recorder.syncCalls[0].hookName)
	require.Len(t, recorder.asyncCalls, 1)
	assert.Equal(t, HookSubUpgradedPost, recorder.asyncCalls[0].hookName)
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
	assert.False(t, sub.PendingChange.IsZero())
	assert.Equal(t, "plan-basic", sub.PendingChange.PlanID)
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
	svc, _, subs, _, _, publisher, recorder := newTestServiceWithHooks()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.ExpireSubscription(ctx, "sub-1")

	require.NoError(t, err)
	require.Len(t, recorder.asyncCalls, 1)
	assert.Equal(t, HookSubExpiredPost, recorder.asyncCalls[0].hookName)
}

// --- CancelSubscription with hooks ---

func TestCancelSubscription_WithHook_Block(t *testing.T) {
	svc, _, subs, _, _, _, recorder := newTestServiceWithHooks()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	recorder.cancelResp = &CancellingResponse{Block: true}

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)

	err := svc.CancelSubscription(ctx, "sub-1", nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrCancellationBlocked)
	// Subscription should NOT be cancelled.
	assert.Equal(t, aggregate.StatusActive, sub.Status)

	subs.AssertExpectations(t)
	require.Len(t, recorder.syncCalls, 1)
	assert.Equal(t, HookSubCancelling, recorder.syncCalls[0].hookName)
}

func TestCancelSubscription_WithHook_Allowed(t *testing.T) {
	svc, _, subs, _, _, publisher, recorder := newTestServiceWithHooks()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	recorder.cancelResp = &CancellingResponse{Block: false}

	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.CancelSubscription(ctx, "sub-1", nil)

	require.NoError(t, err)
	assert.Equal(t, aggregate.StatusCancelled, sub.Status)

	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
	require.Len(t, recorder.syncCalls, 1)
	assert.Equal(t, HookSubCancelling, recorder.syncCalls[0].hookName)
	require.Len(t, recorder.asyncCalls, 1)
	assert.Equal(t, HookSubCancelledPost, recorder.asyncCalls[0].hookName)
}

// --- Helpers ---

func ptr[T any](v T) *T {
	return &v
}
