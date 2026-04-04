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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch/hookdispatchtest"
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

	// Pricing hook dispatcher.
	dispatcher := &hookdispatchtest.MockDispatcher{}
	dispatcher.On("DispatchSync", mock.Anything, "pricing.calculate", mock.AnythingOfType("json.RawMessage")).
		Return(nil, nil)

	// Create checkout service with billing-owned PaymentGateway.
	checkoutPub := &billingtest.MockEventPublisher{}
	checkoutPub.On("Publish", mock.Anything, mock.Anything).Return(nil).Maybe()
	checkoutSvc := NewCheckoutService(billingSvc, paymentGW, dispatcher, checkoutPub, checkoutLogger(), nil)

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
	dispatcher.AssertExpectations(t)
}

func TestCompleteCheckout_Success(t *testing.T) {
	svc, _, subs, invoices, _, publisher := newTestBillingService()

	sub := trialSub("user-1", "plan-premium")
	inv := sampleDraftInvoice("sub-1")

	invoices.On("GetByID", mock.Anything, "inv-1").Return(inv, nil)
	invoices.On("Update", mock.Anything, inv).Return(nil)
	subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil)
	subs.On("Update", mock.Anything, sub).Return(nil)
	publisher.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	// Payment gateway is not needed for CompleteCheckout; only billing service is used.
	checkoutSvc := NewCheckoutService(svc, nil, nil, publisher, checkoutLogger(), nil)

	err := checkoutSvc.CompleteCheckout(context.Background(), "inv-1")

	require.NoError(t, err)

	invoices.AssertExpectations(t)
	subs.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestStartCheckout_MissingUserID(t *testing.T) {
	checkoutSvc := NewCheckoutService(nil, nil, nil, nil, checkoutLogger(), nil)

	_, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		PlanID: "plan-premium",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user ID is required")
}

func TestStartCheckout_MissingPlanID(t *testing.T) {
	checkoutSvc := NewCheckoutService(nil, nil, nil, nil, checkoutLogger(), nil)

	_, err := checkoutSvc.StartCheckout(context.Background(), CheckoutRequest{
		UserID: "user-1",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan ID is required")
}

func TestCompleteCheckout_MissingInvoiceID(t *testing.T) {
	checkoutSvc := NewCheckoutService(nil, nil, nil, nil, checkoutLogger(), nil)

	err := checkoutSvc.CompleteCheckout(context.Background(), "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invoice ID is required")
}

func TestStartCheckout_RateLimited(t *testing.T) {
	rateLimiter := &billingtest.MockDomainRateLimiter{}
	rateLimiter.On("AllowCheckout", mock.Anything, "user-1").Return(false, nil)

	checkoutSvc := NewCheckoutService(nil, nil, nil, nil, checkoutLogger(), rateLimiter)

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

	dispatcher := &hookdispatchtest.MockDispatcher{}
	dispatcher.On("DispatchSync", mock.Anything, "pricing.calculate", mock.AnythingOfType("json.RawMessage")).
		Return(nil, nil)

	rateLimiter := &billingtest.MockDomainRateLimiter{}
	rateLimiter.On("AllowCheckout", mock.Anything, "user-1").
		Return(false, errors.New("valkey unavailable"))

	checkoutPub := &billingtest.MockEventPublisher{}
	checkoutPub.On("Publish", mock.Anything, mock.Anything).Return(nil).Maybe()
	checkoutSvc := NewCheckoutService(billingSvc, paymentGW, dispatcher, checkoutPub, checkoutLogger(), rateLimiter)

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
