package telegram

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// ══════════════════════════════════════════════════════════════════════════════
// Subscription adapter
// ══════════════════════════════════════════════════════════════════════════════

// subscriptionFetcher is the narrow callable surface the subscription adapter
// requires from the billing layer. billing.SubscriptionReader satisfies this;
// the narrow type is declared here so tests can inject a stub without needing
// the full billing.SubscriptionReader method set.
type subscriptionFetcher interface {
	GetByID(ctx context.Context, id string) (*aggregate.Subscription, error)
	GetActiveByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error)
}

type subscriptionReaderAdapter struct {
	subs subscriptionFetcher
}

// NewSubscriptionReaderAdapter returns a bothost.SubscriptionReader backed by r.
// In production, pass a billing.SubscriptionReader directly — it satisfies the
// narrow subscriptionFetcher interface used internally.
//
// Tenant scoping is the caller's responsibility: the ctx passed to each method
// must carry the tenant GUC (set via RunInTx + WithTenantID); this adapter
// must NOT set the GUC itself.
func NewSubscriptionReaderAdapter(r billing.SubscriptionReader) bothost.SubscriptionReader {
	return &subscriptionReaderAdapter{subs: r}
}

// ActiveByUser implements bothost.SubscriptionReader.
func (a *subscriptionReaderAdapter) ActiveByUser(ctx context.Context, userID string) ([]bothost.Subscription, error) {
	aggs, err := a.subs.GetActiveByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("subscriptions.active_by_user: %w", err)
	}
	views := make([]bothost.Subscription, 0, len(aggs))
	for _, s := range aggs {
		views = append(views, mapSubscription(s))
	}
	return views, nil
}

// Get implements bothost.SubscriptionReader.
func (a *subscriptionReaderAdapter) Get(ctx context.Context, id string) (*bothost.Subscription, string, error) {
	s, err := a.subs.GetByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if s == nil {
		return nil, "", nil
	}
	v := mapSubscription(s)
	return &v, s.UserID, nil
}

// mapSubscription converts a billing subscription aggregate to the bot-plugin
// view. ExpiresAt is formatted as RFC3339; it is left empty for open-ended
// subscriptions whose Period.End is a zero time.Time.
func mapSubscription(s *aggregate.Subscription) bothost.Subscription {
	expiresAt := ""
	if !s.Period.End.IsZero() {
		expiresAt = s.Period.End.Format(time.RFC3339)
	}
	return bothost.Subscription{
		ID:        s.ID,
		PlanID:    s.PlanID,
		Status:    string(s.Status),
		ExpiresAt: expiresAt,
	}
}

// Compile-time assertion: subscriptionReaderAdapter must implement bothost.SubscriptionReader.
var _ bothost.SubscriptionReader = (*subscriptionReaderAdapter)(nil)

// ══════════════════════════════════════════════════════════════════════════════
// Invoice adapter
// ══════════════════════════════════════════════════════════════════════════════

// invoiceFetcher is the narrow callable surface the invoice adapter requires
// from the billing layer. billing.InvoiceReader satisfies this.
type invoiceFetcher interface {
	GetPendingByUserID(ctx context.Context, userID string) ([]*aggregate.Invoice, error)
}

type invoiceReaderAdapter struct {
	invoices invoiceFetcher
}

// NewInvoiceReaderAdapter returns a bothost.InvoiceReader backed by r.
// In production, pass a billing.InvoiceReader directly — it satisfies the
// narrow invoiceFetcher interface used internally.
//
// Tenant scoping is the caller's responsibility: the ctx passed to each method
// must carry the tenant GUC (set via RunInTx + WithTenantID); this adapter
// must NOT set the GUC itself.
func NewInvoiceReaderAdapter(r billing.InvoiceReader) bothost.InvoiceReader {
	return &invoiceReaderAdapter{invoices: r}
}

// PendingByUser implements bothost.InvoiceReader.
func (a *invoiceReaderAdapter) PendingByUser(ctx context.Context, userID string) ([]bothost.Invoice, error) {
	aggs, err := a.invoices.GetPendingByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("invoices.pending_by_user: %w", err)
	}
	views := make([]bothost.Invoice, 0, len(aggs))
	for _, inv := range aggs {
		views = append(views, bothost.Invoice{
			ID:       inv.ID,
			Status:   string(inv.Status),
			Amount:   inv.Total.Amount,
			Currency: string(inv.Total.Currency),
		})
	}
	return views, nil
}

// Compile-time assertion: invoiceReaderAdapter must implement bothost.InvoiceReader.
var _ bothost.InvoiceReader = (*invoiceReaderAdapter)(nil)

// ══════════════════════════════════════════════════════════════════════════════
// Checkout adapter
// ══════════════════════════════════════════════════════════════════════════════

// checkoutStarter is the narrow callable surface the checkout adapter requires
// from CheckoutService. *billingservice.CheckoutService satisfies this; the
// narrow type is declared here so tests can inject a stub.
type checkoutStarter interface {
	StartCheckout(ctx context.Context, req billingservice.CheckoutRequest) (*billingservice.CheckoutResult, error)
}

type checkoutStarterAdapter struct {
	svc checkoutStarter
}

// NewCheckoutStarterAdapter returns a bothost.CheckoutStarter backed by svc.
//
// Note: CheckoutService.StartCheckout self-wraps its own RunInTx internally.
// Callers must NOT wrap Start inside another RunInTx.
func NewCheckoutStarterAdapter(svc *billingservice.CheckoutService) bothost.CheckoutStarter {
	return &checkoutStarterAdapter{svc: svc}
}

// Start implements bothost.CheckoutStarter.
func (a *checkoutStarterAdapter) Start(ctx context.Context, in bothost.CheckoutInput) (bothost.CheckoutResult, error) {
	res, err := a.svc.StartCheckout(ctx, billingservice.CheckoutRequest{
		UserID:      in.UserID,
		PlanID:      in.PlanID,
		UserEmail:   in.UserEmail,
		UserCountry: in.UserCountry,
	})
	if err != nil {
		return bothost.CheckoutResult{}, fmt.Errorf("checkout.start: %w", err)
	}
	if res == nil {
		return bothost.CheckoutResult{}, errors.New("checkout.start: nil result without error")
	}
	return bothost.CheckoutResult{
		CheckoutURL:    res.CheckoutURL,
		SubscriptionID: res.SubscriptionID,
		InvoiceID:      res.InvoiceID,
		Provider:       res.Provider,
	}, nil
}

// Compile-time assertion: checkoutStarterAdapter must implement bothost.CheckoutStarter.
var _ bothost.CheckoutStarter = (*checkoutStarterAdapter)(nil)
