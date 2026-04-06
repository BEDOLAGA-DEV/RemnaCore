package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// --- checkout test infrastructure ---

type checkoutTestPublisher struct {
	events []domainevent.Event
}

func (p *checkoutTestPublisher) Publish(_ context.Context, event domainevent.Event) error {
	p.events = append(p.events, event)
	return nil
}

func checkoutLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func sampleDraftInvoice(subID string) *aggregate.Invoice {
	now := time.Now()
	return &aggregate.Invoice{
		ID:             "inv-1",
		SubscriptionID: subID,
		UserID:         "user-1",
		LineItems:      []vo.LineItem{vo.NewLineItem("Premium VPN", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1)},
		Subtotal:       vo.NewMoney(999, vo.CurrencyUSD),
		TotalDiscount:  vo.Zero(vo.CurrencyUSD),
		Total:          vo.NewMoney(999, vo.CurrencyUSD),
		Status:         aggregate.InvoiceDraft,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// noPricingModifier returns a MockPricingModifier that applies no modification.
func noPricingModifier() *billingtest.MockPricingModifier {
	pm := &billingtest.MockPricingModifier{}
	pm.On("BeginFlow", mock.Anything).Return()
	pm.On("ModifyPricing", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	return pm
}

// --- Tests ---

func TestStartCheckout_Success(t *testing.T) {
	// Set up billing service with mocks.
	billingSvc, plans, subs, invoices, _, billingPub := newTestBillingService()

	plan := samplePlan()
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	subs.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).Return(nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	billingPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	// Set up payment gateway mock (billing-owned ACL interface).
	paymentGW := &billingtest.MockPaymentGateway{}
	paymentGW.On("CreateCharge", mock.Anything, mock.AnythingOfType("billing.CreateChargeRequest")).
		Return(&billing.CreateChargeResult{
			Provider:    "stripe",
			ExternalID:  "pi_456",
			CheckoutURL: "https://checkout.stripe.com/session/456",
			Status:      "pending",
		}, nil)

	// Pricing modifier — no modification.
	pricingMod := noPricingModifier()

	// Create checkout service with billing-owned PaymentGateway.
	checkoutPub := &billingtest.MockEventPublisher{}
	checkoutPub.On("Publish", mock.Anything, mock.Anything).Return(nil).Maybe()
	checkoutSvc := NewCheckoutService(billingSvc, paymentGW, pricingMod, checkoutPub, checkoutLogger(), billing.AlwaysAllowRateLimiter{}, clock.NewReal())

	result, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		UserID:    "user-1",
		UserEmail: "user@example.com",
		PlanID:    "plan-premium",
		ReturnURL: "https://example.com/success",
		CancelURL: "https://example.com/cancel",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.SubscriptionID)
	assert.NotEmpty(t, result.InvoiceID)
	assert.Equal(t, "https://checkout.stripe.com/session/456", result.CheckoutURL)
	assert.Equal(t, "stripe", result.Provider)

	plans.AssertExpectations(t)
	subs.AssertExpectations(t)
	invoices.AssertExpectations(t)
	paymentGW.AssertExpectations(t)
	pricingMod.AssertExpectations(t)
}

func TestCompleteCheckout_Success(t *testing.T) {
	svc, _, subs, invoices, _, publisher := newTestBillingService()

	sub := trialSub("user-1", "plan-premium")
	inv := sampleDraftInvoice("sub-1")

	invoices.On("GetByIDForUpdate", mock.Anything, "inv-1").Return(inv, nil)
	invoices.On("Update", mock.Anything, inv).Return(nil)
	subs.On("GetByIDForUpdate", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	// Payment gateway and pricing modifier are not needed for CompleteCheckout.
	checkoutSvc := NewCheckoutService(svc, nil, nil, publisher, checkoutLogger(), billing.AlwaysAllowRateLimiter{}, clock.NewReal())

	err := checkoutSvc.CompleteCheckout(context.Background(), "inv-1")

	require.NoError(t, err)

	invoices.AssertExpectations(t)
	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestStartCheckout_MissingUserID(t *testing.T) {
	checkoutSvc := NewCheckoutService(nil, nil, nil, nil, checkoutLogger(), billing.AlwaysAllowRateLimiter{}, clock.NewReal())

	_, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		PlanID: "plan-premium",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user ID is required")
}

func TestStartCheckout_MissingPlanID(t *testing.T) {
	checkoutSvc := NewCheckoutService(nil, nil, nil, nil, checkoutLogger(), billing.AlwaysAllowRateLimiter{}, clock.NewReal())

	_, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		UserID: "user-1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan ID is required")
}

func TestCompleteCheckout_MissingInvoiceID(t *testing.T) {
	checkoutSvc := NewCheckoutService(nil, nil, nil, nil, checkoutLogger(), billing.AlwaysAllowRateLimiter{}, clock.NewReal())

	err := checkoutSvc.CompleteCheckout(context.Background(), "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invoice ID is required")
}

func TestStartCheckout_RateLimited(t *testing.T) {
	rateLimiter := &billingtest.MockDomainRateLimiter{}
	rateLimiter.On("AllowCheckout", mock.Anything, "user-1").Return(false, nil)

	checkoutSvc := NewCheckoutService(nil, nil, nil, nil, checkoutLogger(), rateLimiter, clock.NewReal())

	_, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		UserID: "user-1",
		PlanID: "plan-premium",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrCheckoutRateLimited)

	rateLimiter.AssertExpectations(t)
}

func TestStartCheckout_RateLimiterError_FailsOpen(t *testing.T) {
	billingSvc, plans, subs, invoices, _, billingPub := newTestBillingService()

	plan := samplePlan()
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	subs.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).Return(nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	billingPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	paymentGW := &billingtest.MockPaymentGateway{}
	paymentGW.On("CreateCharge", mock.Anything, mock.AnythingOfType("billing.CreateChargeRequest")).
		Return(&billing.CreateChargeResult{
			Provider:    "stripe",
			ExternalID:  "pi_789",
			CheckoutURL: "https://checkout.stripe.com/session/789",
			Status:      "pending",
		}, nil)

	pricingMod := noPricingModifier()

	rateLimiter := &billingtest.MockDomainRateLimiter{}
	rateLimiter.On("AllowCheckout", mock.Anything, "user-1").
		Return(false, errors.New("valkey unavailable"))

	checkoutPub := &billingtest.MockEventPublisher{}
	checkoutPub.On("Publish", mock.Anything, mock.Anything).Return(nil).Maybe()
	checkoutSvc := NewCheckoutService(billingSvc, paymentGW, pricingMod, checkoutPub, checkoutLogger(), rateLimiter, clock.NewReal())

	result, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		UserID:    "user-1",
		UserEmail: "user@example.com",
		PlanID:    "plan-premium",
		ReturnURL: "https://example.com/success",
		CancelURL: "https://example.com/cancel",
	})

	// Should succeed because rate limiter errors fail open
	require.NoError(t, err)
	assert.NotEmpty(t, result.SubscriptionID)
	assert.Equal(t, "stripe", result.Provider)

	rateLimiter.AssertExpectations(t)
}

// --- Pricing plugin integration tests ---

func TestStartCheckout_PricingModifier_DiscountApplied(t *testing.T) {
	billingSvc, plans, subs, invoices, _, billingPub := newTestBillingService()

	plan := samplePlan()
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	subs.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).Return(nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	billingPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	paymentGW := &billingtest.MockPaymentGateway{}
	paymentGW.On("CreateCharge", mock.Anything, mock.MatchedBy(func(req billing.CreateChargeRequest) bool {
		return true
	})).
		Return(&billing.CreateChargeResult{
			Provider:    "stripe",
			ExternalID:  "pi_pricing",
			CheckoutURL: "https://checkout.stripe.com/session/pricing",
			Status:      "pending",
		}, nil)

	// Pricing modifier returns a discount of 200 cents.
	discountCents := int64(200)
	pricingMod := &billingtest.MockPricingModifier{}
	pricingMod.On("BeginFlow", mock.Anything).Return()
	pricingMod.On("ModifyPricing", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&billing.PricingModification{
			Discount: &discountCents,
			Reason:   "loyalty_10pct",
		}, nil)

	checkoutPub := &billingtest.MockEventPublisher{}
	checkoutPub.On("Publish", mock.Anything, mock.Anything).Return(nil).Maybe()
	checkoutSvc := NewCheckoutService(billingSvc, paymentGW, pricingMod, checkoutPub, checkoutLogger(), billing.AlwaysAllowRateLimiter{}, clock.NewReal())

	result, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		UserID:    "user-1",
		UserEmail: "user@example.com",
		PlanID:    "plan-premium",
		ReturnURL: "https://example.com/success",
		CancelURL: "https://example.com/cancel",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.SubscriptionID)

	// Verify the charge was created with the discounted amount (999 - 200 = 799).
	paymentGW.AssertCalled(t, "CreateCharge", mock.Anything, mock.MatchedBy(func(req billing.CreateChargeRequest) bool {
		return req.Amount == 799
	}))
	pricingMod.AssertExpectations(t)
}

func TestStartCheckout_PricingModifier_Error_OriginalPrice(t *testing.T) {
	billingSvc, plans, subs, invoices, _, billingPub := newTestBillingService()

	plan := samplePlan()
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	subs.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).Return(nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	billingPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	paymentGW := &billingtest.MockPaymentGateway{}
	paymentGW.On("CreateCharge", mock.Anything, mock.AnythingOfType("billing.CreateChargeRequest")).
		Return(&billing.CreateChargeResult{
			Provider:    "stripe",
			ExternalID:  "pi_err",
			CheckoutURL: "https://checkout.stripe.com/session/err",
			Status:      "pending",
		}, nil)

	// Pricing modifier returns an error -- price should remain unchanged.
	pricingMod := &billingtest.MockPricingModifier{}
	pricingMod.On("BeginFlow", mock.Anything).Return()
	pricingMod.On("ModifyPricing", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("plugin crashed"))

	checkoutPub := &billingtest.MockEventPublisher{}
	checkoutPub.On("Publish", mock.Anything, mock.Anything).Return(nil).Maybe()
	checkoutSvc := NewCheckoutService(billingSvc, paymentGW, pricingMod, checkoutPub, checkoutLogger(), billing.AlwaysAllowRateLimiter{}, clock.NewReal())

	result, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		UserID:    "user-1",
		UserEmail: "user@example.com",
		PlanID:    "plan-premium",
		ReturnURL: "https://example.com/success",
		CancelURL: "https://example.com/cancel",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.SubscriptionID)

	// Verify original price was used (999 cents for the premium plan).
	paymentGW.AssertCalled(t, "CreateCharge", mock.Anything, mock.MatchedBy(func(req billing.CreateChargeRequest) bool {
		return req.Amount == 999
	}))
}

func TestStartCheckout_PricingModifier_NilResult_OriginalPrice(t *testing.T) {
	billingSvc, plans, subs, invoices, _, billingPub := newTestBillingService()

	plan := samplePlan()
	plans.On("GetByID", mock.Anything, "plan-premium").Return(plan, nil)
	subs.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).Return(nil)
	invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
	billingPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	paymentGW := &billingtest.MockPaymentGateway{}
	paymentGW.On("CreateCharge", mock.Anything, mock.AnythingOfType("billing.CreateChargeRequest")).
		Return(&billing.CreateChargeResult{
			Provider:    "stripe",
			ExternalID:  "pi_nil",
			CheckoutURL: "https://checkout.stripe.com/session/nil",
			Status:      "pending",
		}, nil)

	// Pricing modifier returns nil (no plugin registered).
	pricingMod := noPricingModifier()

	checkoutPub := &billingtest.MockEventPublisher{}
	checkoutPub.On("Publish", mock.Anything, mock.Anything).Return(nil).Maybe()
	checkoutSvc := NewCheckoutService(billingSvc, paymentGW, pricingMod, checkoutPub, checkoutLogger(), billing.AlwaysAllowRateLimiter{}, clock.NewReal())

	result, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		UserID:    "user-1",
		UserEmail: "user@example.com",
		PlanID:    "plan-premium",
		ReturnURL: "https://example.com/success",
		CancelURL: "https://example.com/cancel",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.SubscriptionID)

	// Verify original price was used.
	paymentGW.AssertCalled(t, "CreateCharge", mock.Anything, mock.MatchedBy(func(req billing.CreateChargeRequest) bool {
		return req.Amount == 999
	}))
}
