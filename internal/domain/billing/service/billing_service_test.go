package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/billingtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

func newTestBillingService() (
	*BillingService,
	*billingtest.MockPlanRepo,
	*billingtest.MockSubscriptionRepo,
	*billingtest.MockInvoiceRepo,
	*billingtest.MockFamilyRepo,
	*billingtest.MockEventPublisher,
) {
	plans := &billingtest.MockPlanRepo{}
	subs := &billingtest.MockSubscriptionRepo{}
	invoices := &billingtest.MockInvoiceRepo{}
	families := &billingtest.MockFamilyRepo{}
	publisher := &billingtest.MockEventPublisher{}
	prorate := NewProrateCalculator()
	trial := NewTrialManager(DefaultTrialDays)

	svc := NewBillingService(plans, subs, invoices, families, publisher, prorate, trial)
	return svc, plans, subs, invoices, families, publisher
}

func samplePlan() *aggregate.Plan {
	return &aggregate.Plan{
		ID:               "plan-premium",
		Name:             "Premium VPN",
		BasePrice:        vo.NewMoney(999, vo.CurrencyUSD),
		Interval:         vo.IntervalMonth,
		Tier:             aggregate.TierPremium,
		FamilyEnabled:    true,
		MaxFamilyMembers: 5,
		IsActive:         true,
		AvailableAddons: []aggregate.Addon{
			{
				ID:    "addon-traffic",
				Name:  "Extra Traffic",
				Price: vo.NewMoney(299, vo.CurrencyUSD),
				Type:  aggregate.AddonTraffic,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func activeSubscription(userID, planID string) *aggregate.Subscription {
	now := time.Now()
	return &aggregate.Subscription{
		ID:        "sub-1",
		UserID:    userID,
		PlanID:    planID,
		Status:    aggregate.StatusActive,
		Period:    vo.NewBillingPeriod(now, vo.IntervalMonth),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func trialSub(userID, planID string) *aggregate.Subscription {
	now := time.Now()
	return &aggregate.Subscription{
		ID:        "sub-1",
		UserID:    userID,
		PlanID:    planID,
		Status:    aggregate.StatusTrial,
		Period:    vo.NewBillingPeriod(now, vo.IntervalMonth),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// --- CreateSubscription ---

func TestCreateSubscription_Success(t *testing.T) {
	svc, plans, subs, invoices, _, publisher := newTestBillingService()
	ctx := context.Background()
	plan := samplePlan()

	plans.On("GetByID", ctx, "plan-premium").Return(plan, nil)
	subs.On("Create", ctx, mock.AnythingOfType("*aggregate.Subscription")).Return(nil)
	invoices.On("Create", ctx, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	publisher.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	cmd := CreateSubscriptionCmd{
		UserID: "user-1",
		PlanID: "plan-premium",
	}

	sub, inv, err := svc.CreateSubscription(ctx, cmd)

	require.NoError(t, err)
	assert.Equal(t, "user-1", sub.UserID)
	assert.Equal(t, "plan-premium", sub.PlanID)
	assert.Equal(t, aggregate.StatusTrial, sub.Status)
	assert.NotNil(t, inv)
	assert.Equal(t, sub.ID, inv.SubscriptionID)
	assert.Equal(t, "user-1", inv.UserID)

	plans.AssertExpectations(t)
	subs.AssertExpectations(t)
	invoices.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestCreateSubscription_PlanNotFound(t *testing.T) {
	svc, plans, _, _, _, _ := newTestBillingService()
	ctx := context.Background()

	plans.On("GetByID", ctx, "nonexistent").Return(nil, billing.ErrPlanNotFound)

	cmd := CreateSubscriptionCmd{
		UserID: "user-1",
		PlanID: "nonexistent",
	}

	sub, inv, err := svc.CreateSubscription(ctx, cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrPlanNotFound)
	assert.Nil(t, sub)
	assert.Nil(t, inv)

	plans.AssertExpectations(t)
}

func TestCreateSubscription_WithAddons(t *testing.T) {
	svc, plans, subs, invoices, _, publisher := newTestBillingService()
	ctx := context.Background()
	plan := samplePlan()

	plans.On("GetByID", ctx, "plan-premium").Return(plan, nil)
	subs.On("Create", ctx, mock.AnythingOfType("*aggregate.Subscription")).Return(nil)
	invoices.On("Create", ctx, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	publisher.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	cmd := CreateSubscriptionCmd{
		UserID:   "user-1",
		PlanID:   "plan-premium",
		AddonIDs: []string{"addon-traffic"},
	}

	sub, inv, err := svc.CreateSubscription(ctx, cmd)

	require.NoError(t, err)
	assert.Equal(t, []string{"addon-traffic"}, sub.AddonIDs)
	// Invoice should have 2 line items: plan + addon
	assert.Len(t, inv.LineItems, 2)

	plans.AssertExpectations(t)
	subs.AssertExpectations(t)
	invoices.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// --- CancelSubscription ---

func TestCancelSubscription_Success(t *testing.T) {
	svc, _, subs, _, _, publisher := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByID", ctx, "sub-1").Return(sub, nil)
	subs.On("Update", ctx, sub).Return(nil)
	publisher.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.CancelSubscription(ctx, "sub-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.StatusCancelled, sub.Status)
	assert.NotNil(t, sub.CancelledAt)

	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestCancelSubscription_AlreadyCancelled(t *testing.T) {
	svc, _, subs, _, _, _ := newTestBillingService()
	ctx := context.Background()
	sub := activeSubscription("user-1", "plan-premium")
	sub.Status = aggregate.StatusCancelled

	subs.On("GetByID", ctx, "sub-1").Return(sub, nil)

	err := svc.CancelSubscription(ctx, "sub-1")

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrInvalidTransition)

	subs.AssertExpectations(t)
}

// --- PayInvoice ---

func TestPayInvoice_Success(t *testing.T) {
	svc, _, subs, invoices, _, publisher := newTestBillingService()
	ctx := context.Background()

	sub := trialSub("user-1", "plan-premium")

	inv := &aggregate.Invoice{
		ID:             "inv-1",
		SubscriptionID: "sub-1",
		UserID:         "user-1",
		LineItems:      []vo.LineItem{vo.NewLineItem("Premium VPN", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1)},
		Subtotal:       vo.NewMoney(999, vo.CurrencyUSD),
		TotalDiscount:  vo.Zero(vo.CurrencyUSD),
		Total:          vo.NewMoney(999, vo.CurrencyUSD),
		Status:         aggregate.InvoiceDraft,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	invoices.On("GetByID", ctx, "inv-1").Return(inv, nil)
	invoices.On("Update", ctx, inv).Return(nil)
	subs.On("GetByID", ctx, "sub-1").Return(sub, nil)
	subs.On("Update", ctx, sub).Return(nil)
	publisher.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.PayInvoice(ctx, "inv-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.InvoicePaid, inv.Status)
	assert.NotNil(t, inv.PaidAt)
	assert.Equal(t, aggregate.StatusActive, sub.Status)

	invoices.AssertExpectations(t)
	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestPayInvoice_AlreadyPaid(t *testing.T) {
	svc, _, _, invoices, _, _ := newTestBillingService()
	ctx := context.Background()

	paidAt := time.Now()
	inv := &aggregate.Invoice{
		ID:             "inv-1",
		SubscriptionID: "sub-1",
		UserID:         "user-1",
		Status:         aggregate.InvoicePaid,
		PaidAt:         &paidAt,
		Total:          vo.NewMoney(999, vo.CurrencyUSD),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	invoices.On("GetByID", ctx, "inv-1").Return(inv, nil)

	err := svc.PayInvoice(ctx, "inv-1")

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrInvoiceAlreadyPaid)

	invoices.AssertExpectations(t)
}

func TestPayInvoice_ActiveSubscription_NoActivation(t *testing.T) {
	svc, _, subs, invoices, _, publisher := newTestBillingService()
	ctx := context.Background()

	sub := activeSubscription("user-1", "plan-premium")

	inv := &aggregate.Invoice{
		ID:             "inv-1",
		SubscriptionID: "sub-1",
		UserID:         "user-1",
		LineItems:      []vo.LineItem{vo.NewLineItem("Premium VPN", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1)},
		Subtotal:       vo.NewMoney(999, vo.CurrencyUSD),
		TotalDiscount:  vo.Zero(vo.CurrencyUSD),
		Total:          vo.NewMoney(999, vo.CurrencyUSD),
		Status:         aggregate.InvoiceDraft,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	invoices.On("GetByID", ctx, "inv-1").Return(inv, nil)
	invoices.On("Update", ctx, inv).Return(nil)
	subs.On("GetByID", ctx, "sub-1").Return(sub, nil)
	// Only invoice.paid event, no subscription.activated
	publisher.On("Publish", ctx, mock.MatchedBy(func(e interface{}) bool {
		return true
	})).Return(nil)

	err := svc.PayInvoice(ctx, "inv-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.InvoicePaid, inv.Status)
	// Subscription stays active, not re-activated
	assert.Equal(t, aggregate.StatusActive, sub.Status)

	invoices.AssertExpectations(t)
	subs.AssertExpectations(t)
}

// --- AddFamilyMember ---

func TestAddFamilyMember_Success(t *testing.T) {
	svc, plans, subs, _, families, publisher := newTestBillingService()
	ctx := context.Background()

	plan := samplePlan()
	sub := activeSubscription("user-1", "plan-premium")

	fg := aggregate.NewFamilyGroup("user-1", 5)

	subs.On("GetByID", ctx, "sub-1").Return(sub, nil)
	plans.On("GetByID", ctx, "plan-premium").Return(plan, nil)
	families.On("GetByOwnerID", ctx, "user-1").Return(fg, nil)
	families.On("Update", ctx, fg).Return(nil)
	publisher.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.AddFamilyMember(ctx, "sub-1", "member-1", "Alice")

	require.NoError(t, err)
	assert.True(t, fg.HasMember("member-1"))

	subs.AssertExpectations(t)
	plans.AssertExpectations(t)
	families.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestAddFamilyMember_FamilyNotEnabled(t *testing.T) {
	svc, plans, subs, _, _, _ := newTestBillingService()
	ctx := context.Background()

	plan := samplePlan()
	plan.FamilyEnabled = false
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByID", ctx, "sub-1").Return(sub, nil)
	plans.On("GetByID", ctx, "plan-premium").Return(plan, nil)

	err := svc.AddFamilyMember(ctx, "sub-1", "member-1", "Alice")

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrFamilyNotEnabled)

	subs.AssertExpectations(t)
	plans.AssertExpectations(t)
}

func TestAddFamilyMember_CreatesGroupIfNotExists(t *testing.T) {
	svc, plans, subs, _, families, publisher := newTestBillingService()
	ctx := context.Background()

	plan := samplePlan()
	sub := activeSubscription("user-1", "plan-premium")

	subs.On("GetByID", ctx, "sub-1").Return(sub, nil)
	plans.On("GetByID", ctx, "plan-premium").Return(plan, nil)
	families.On("GetByOwnerID", ctx, "user-1").Return(nil, billing.ErrFamilyGroupNotFound)
	families.On("Create", ctx, mock.AnythingOfType("*aggregate.FamilyGroup")).Return(nil)
	families.On("Update", ctx, mock.AnythingOfType("*aggregate.FamilyGroup")).Return(nil)
	publisher.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.AddFamilyMember(ctx, "sub-1", "member-1", "Alice")

	require.NoError(t, err)

	families.AssertCalled(t, "Create", ctx, mock.AnythingOfType("*aggregate.FamilyGroup"))
	families.AssertExpectations(t)
}

// --- RemoveFamilyMember ---

func TestRemoveFamilyMember_Success(t *testing.T) {
	svc, _, subs, _, families, publisher := newTestBillingService()
	ctx := context.Background()

	sub := activeSubscription("user-1", "plan-premium")
	fg := aggregate.NewFamilyGroup("user-1", 5)
	require.NoError(t, fg.AddMember("member-1", "Alice"))

	subs.On("GetByID", ctx, "sub-1").Return(sub, nil)
	families.On("GetByOwnerID", ctx, "user-1").Return(fg, nil)
	families.On("Update", ctx, fg).Return(nil)
	publisher.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.RemoveFamilyMember(ctx, "sub-1", "member-1")

	require.NoError(t, err)
	assert.False(t, fg.HasMember("member-1"))

	subs.AssertExpectations(t)
	families.AssertExpectations(t)
	publisher.AssertExpectations(t)
}
