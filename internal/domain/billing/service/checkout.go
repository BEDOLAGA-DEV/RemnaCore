package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
)

// HookPricingCalculate is the plugin hook dispatched to modify invoice pricing.
const HookPricingCalculate = "pricing.calculate"

// CheckoutService orchestrates the full checkout flow: subscription creation,
// invoice generation, and payment charge initiation via the payment gateway.
type CheckoutService struct {
	billing     *BillingService
	payment     billing.PaymentGateway
	dispatcher  hookdispatch.Dispatcher
	publisher   domainevent.Publisher
	logger      *slog.Logger
	rateLimiter billing.DomainRateLimiter
}

// NewCheckoutService creates a CheckoutService with the given dependencies.
func NewCheckoutService(
	billingSvc *BillingService,
	paymentGateway billing.PaymentGateway,
	dispatcher hookdispatch.Dispatcher,
	publisher domainevent.Publisher,
	logger *slog.Logger,
	rateLimiter billing.DomainRateLimiter,
) *CheckoutService {
	return &CheckoutService{
		billing:     billingSvc,
		payment:     paymentGateway,
		dispatcher:  dispatcher,
		publisher:   publisher,
		logger:      logger,
		rateLimiter: rateLimiter,
	}
}

// CheckoutRequest holds the parameters for starting a checkout flow.
type CheckoutRequest struct {
	UserID    string
	UserEmail string
	PlanID    string
	AddonIDs  []string
	ReturnURL string
	CancelURL string
}

// CheckoutResult holds the output of a started checkout flow.
type CheckoutResult struct {
	SubscriptionID string `json:"subscription_id"`
	InvoiceID      string `json:"invoice_id"`
	CheckoutURL    string `json:"checkout_url"`
	Provider       string `json:"provider"`
}

// StartCheckout creates a subscription and invoice, then initiates a payment
// charge through the payment gateway. Returns the checkout URL for the user to
// complete payment.
func (cs *CheckoutService) StartCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutResult, error) {
	if req.UserID == "" {
		return nil, fmt.Errorf("user ID is required")
	}
	if req.PlanID == "" {
		return nil, fmt.Errorf("plan ID is required")
	}

	// Rate limit check BEFORE any business logic. Fail open on errors so that
	// a transient rate limiter issue does not block legitimate checkouts.
	allowed, err := cs.rateLimiter.AllowCheckout(ctx, req.UserID)
	if err != nil {
		cs.logger.Warn("rate limit check failed, allowing",
			slog.String("user_id", req.UserID),
			slog.Any("error", err),
		)
	} else if !allowed {
		return nil, billing.ErrCheckoutRateLimited
	}

	// Pin plugin versions for the duration of this checkout flow so that all
	// hook calls within this flow use the same plugin version, even if a
	// plugin is hot-reloaded mid-flow.
	if cs.dispatcher != nil {
		ctx = cs.dispatcher.BeginFlow(ctx)
	}

	// 1. Create subscription + invoice via billing service.
	sub, inv, err := cs.billing.CreateSubscription(ctx, CreateSubscriptionCmd{
		UserID:   req.UserID,
		PlanID:   req.PlanID,
		AddonIDs: req.AddonIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}

	// 2. Run pricing plugins (can modify final price). Best-effort: if no
	//    handler is registered or the hook errors, proceed with the original price.
	if cs.dispatcher != nil {
		pricingPayload, _ := json.Marshal(map[string]any{
			"invoice_id": inv.ID,
			"user_id":    req.UserID,
			"plan_id":    req.PlanID,
			"subtotal":   inv.Subtotal.Amount,
			"currency":   string(inv.Total.Currency),
		})
		if _, dispatchErr := cs.dispatcher.DispatchSync(ctx, HookPricingCalculate, pricingPayload); dispatchErr != nil {
			cs.logger.Warn("pricing.calculate hook failed, using original price",
				slog.String("invoice_id", inv.ID),
				slog.Any("error", dispatchErr),
			)
		}
	}

	// 3. Initiate payment charge via the payment gateway (ACL boundary).
	chargeResult, err := cs.payment.CreateCharge(ctx, billing.CreateChargeRequest{
		InvoiceID: inv.ID,
		Amount:    inv.Total.Amount,
		Currency:  string(inv.Total.Currency),
		UserID:    req.UserID,
		UserEmail: req.UserEmail,
		PlanName:  sub.PlanID, // Plan name from subscription; caller can enrich later.
		ReturnURL: req.ReturnURL,
		CancelURL: req.CancelURL,
	})
	if err != nil {
		return nil, fmt.Errorf("create payment charge: %w", err)
	}

	cs.logger.Info("checkout started",
		slog.String("subscription_id", sub.ID),
		slog.String("invoice_id", inv.ID),
		slog.String("provider", chargeResult.Provider),
	)

	return &CheckoutResult{
		SubscriptionID: sub.ID,
		InvoiceID:      inv.ID,
		CheckoutURL:    chargeResult.CheckoutURL,
		Provider:       chargeResult.Provider,
	}, nil
}

// CompleteCheckout is called when a payment webhook confirms success. It marks
// the invoice as paid (which activates the subscription if in trial/past_due).
func (cs *CheckoutService) CompleteCheckout(ctx context.Context, invoiceID string) error {
	if invoiceID == "" {
		return fmt.Errorf("invoice ID is required")
	}

	if err := cs.billing.PayInvoice(ctx, invoiceID); err != nil {
		return fmt.Errorf("pay invoice: %w", err)
	}

	cs.logger.Info("checkout completed", slog.String("invoice_id", invoiceID))
	return nil
}
