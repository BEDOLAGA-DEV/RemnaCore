package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// ──────────────────────────────────────────────────────────────────────────────
// Subscription stubs
// ──────────────────────────────────────────────────────────────────────────────

// stubSubscriptionFetcher implements the narrow subscriptionFetcher interface.
type stubSubscriptionFetcher struct {
	active []*aggregate.Subscription
	byID   *aggregate.Subscription
	err    error
}

func (s *stubSubscriptionFetcher) GetByID(_ context.Context, _ string) (*aggregate.Subscription, error) {
	return s.byID, s.err
}

func (s *stubSubscriptionFetcher) GetActiveByUserID(_ context.Context, _ string) ([]*aggregate.Subscription, error) {
	return s.active, s.err
}

// ──────────────────────────────────────────────────────────────────────────────
// SubscriptionReaderAdapter tests
// ──────────────────────────────────────────────────────────────────────────────

func TestSubscriptionReaderAdapter_ActiveByUser_MapsAllFields(t *testing.T) {
	end := time.Date(2025, 3, 1, 12, 0, 0, 0, time.UTC)
	sub := &aggregate.Subscription{
		ID:     "sub-1",
		UserID: "user-1",
		PlanID: "plan-1",
		Status: aggregate.StatusActive,
		Period: vo.BillingPeriod{End: end},
	}
	adapter := &subscriptionReaderAdapter{subs: &stubSubscriptionFetcher{active: []*aggregate.Subscription{sub}}}

	subs, err := adapter.ActiveByUser(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, subs, 1)

	assert.Equal(t, "sub-1", subs[0].ID)
	assert.Equal(t, "plan-1", subs[0].PlanID)
	assert.Equal(t, "active", subs[0].Status)
	assert.Equal(t, end.Format(time.RFC3339), subs[0].ExpiresAt)
}

func TestSubscriptionReaderAdapter_ActiveByUser_ZeroEnd_EmptyExpiresAt(t *testing.T) {
	sub := &aggregate.Subscription{
		ID:     "sub-2",
		UserID: "user-2",
		PlanID: "plan-2",
		Status: aggregate.StatusTrial,
		Period: vo.BillingPeriod{}, // zero time.Time End
	}
	adapter := &subscriptionReaderAdapter{subs: &stubSubscriptionFetcher{active: []*aggregate.Subscription{sub}}}

	subs, err := adapter.ActiveByUser(context.Background(), "user-2")
	require.NoError(t, err)
	require.Len(t, subs, 1)
	assert.Empty(t, subs[0].ExpiresAt, "ExpiresAt must be empty for zero end time")
}

func TestSubscriptionReaderAdapter_Get_ReturnsViewAndOwnerUserID(t *testing.T) {
	end := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	sub := &aggregate.Subscription{
		ID:     "sub-3",
		UserID: "owner-user",
		PlanID: "plan-3",
		Status: aggregate.StatusActive,
		Period: vo.BillingPeriod{End: end},
	}
	adapter := &subscriptionReaderAdapter{subs: &stubSubscriptionFetcher{byID: sub}}

	view, ownerUserID, err := adapter.Get(context.Background(), "sub-3")
	require.NoError(t, err)
	require.NotNil(t, view)

	assert.Equal(t, "sub-3", view.ID)
	assert.Equal(t, "plan-3", view.PlanID)
	assert.Equal(t, "active", view.Status)
	assert.Equal(t, end.Format(time.RFC3339), view.ExpiresAt)
	assert.Equal(t, "owner-user", ownerUserID)
}

func TestSubscriptionReaderAdapter_Get_NotFound_ReturnsNilNoError(t *testing.T) {
	adapter := &subscriptionReaderAdapter{subs: &stubSubscriptionFetcher{byID: nil}}

	view, ownerUserID, err := adapter.Get(context.Background(), "not-exist")
	require.NoError(t, err)
	assert.Nil(t, view)
	assert.Empty(t, ownerUserID)
}

// Compile-time check: subscriptionReaderAdapter satisfies bothost.SubscriptionReader.
var _ bothost.SubscriptionReader = (*subscriptionReaderAdapter)(nil)

// ──────────────────────────────────────────────────────────────────────────────
// Invoice stubs
// ──────────────────────────────────────────────────────────────────────────────

// stubInvoiceFetcher implements the narrow invoiceFetcher interface.
type stubInvoiceFetcher struct {
	pending []*aggregate.Invoice
	err     error
}

func (s *stubInvoiceFetcher) GetPendingByUserID(_ context.Context, _ string) ([]*aggregate.Invoice, error) {
	return s.pending, s.err
}

// ──────────────────────────────────────────────────────────────────────────────
// InvoiceReaderAdapter tests
// ──────────────────────────────────────────────────────────────────────────────

func TestInvoiceReaderAdapter_PendingByUser_MapsAllFields(t *testing.T) {
	inv := &aggregate.Invoice{
		ID:     "inv-1",
		UserID: "user-1",
		Status: aggregate.InvoicePending,
		Total:  vo.NewMoney(1500, vo.CurrencyUSD),
	}
	adapter := &invoiceReaderAdapter{invoices: &stubInvoiceFetcher{pending: []*aggregate.Invoice{inv}}}

	invoices, err := adapter.PendingByUser(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, invoices, 1)

	assert.Equal(t, "inv-1", invoices[0].ID)
	assert.Equal(t, "pending", invoices[0].Status)
	assert.Equal(t, int64(1500), invoices[0].Amount)
	assert.Equal(t, "usd", invoices[0].Currency)
}

func TestInvoiceReaderAdapter_PendingByUser_EmptyList(t *testing.T) {
	adapter := &invoiceReaderAdapter{invoices: &stubInvoiceFetcher{pending: nil}}

	invoices, err := adapter.PendingByUser(context.Background(), "user-no-invoices")
	require.NoError(t, err)
	assert.Empty(t, invoices)
}

// Compile-time check: invoiceReaderAdapter satisfies bothost.InvoiceReader.
var _ bothost.InvoiceReader = (*invoiceReaderAdapter)(nil)

// ──────────────────────────────────────────────────────────────────────────────
// Checkout stubs
// ──────────────────────────────────────────────────────────────────────────────

// stubCheckoutSvc implements the narrow checkoutStarter interface.
type stubCheckoutSvc struct {
	capturedReq *billingservice.CheckoutRequest
	result      *billingservice.CheckoutResult
	err         error
}

func (s *stubCheckoutSvc) StartCheckout(_ context.Context, req billingservice.CheckoutRequest) (*billingservice.CheckoutResult, error) {
	s.capturedReq = &req
	return s.result, s.err
}

// ──────────────────────────────────────────────────────────────────────────────
// CheckoutStarterAdapter tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckoutStarterAdapter_Start_ThreadsInputAndMapsOutput(t *testing.T) {
	stub := &stubCheckoutSvc{
		result: &billingservice.CheckoutResult{
			CheckoutURL:    "https://pay.example.com/1",
			SubscriptionID: "sub-1",
			InvoiceID:      "inv-1",
			Provider:       "stripe",
		},
	}
	adapter := &checkoutStarterAdapter{svc: stub}

	in := bothost.CheckoutInput{
		UserID:      "user-1",
		PlanID:      "plan-1",
		UserEmail:   "user@example.com",
		UserCountry: "US",
	}
	result, err := adapter.Start(context.Background(), in)
	require.NoError(t, err)

	// Verify all input fields are threaded through to the underlying request.
	require.NotNil(t, stub.capturedReq)
	assert.Equal(t, "user-1", stub.capturedReq.UserID)
	assert.Equal(t, "plan-1", stub.capturedReq.PlanID)
	assert.Equal(t, "user@example.com", stub.capturedReq.UserEmail)
	assert.Equal(t, "US", stub.capturedReq.UserCountry)

	// Verify output field mapping.
	assert.Equal(t, "https://pay.example.com/1", result.CheckoutURL)
	assert.Equal(t, "sub-1", result.SubscriptionID)
	assert.Equal(t, "inv-1", result.InvoiceID)
	assert.Equal(t, "stripe", result.Provider)
}

// Compile-time check: checkoutStarterAdapter satisfies bothost.CheckoutStarter.
var _ bothost.CheckoutStarter = (*checkoutStarterAdapter)(nil)
