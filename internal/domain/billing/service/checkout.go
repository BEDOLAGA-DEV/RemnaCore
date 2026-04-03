package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
)

// CheckoutService orchestrates the full checkout flow: subscription creation,
// invoice generation, and payment charge initiation via the payment facade.
type CheckoutService struct {
	billing    *BillingService
	payment    *payment.PaymentFacade
	dispatcher hookdispatch.Dispatcher
	publisher  domainevent.Publisher
	logger     *slog.Logger
}

// NewCheckoutService creates a CheckoutService with the given dependencies.
func NewCheckoutService(
	billing *BillingService,
	paymentFacade *payment.PaymentFacade,
	dispatcher hookdispatch.Dispatcher,
	publisher domainevent.Publisher,
	logger *slog.Logger,
) *CheckoutService {
	return &CheckoutService{
		billing:    billing,
		payment:    paymentFacade,
		dispatcher: dispatcher,
		publisher:  publisher,
		logger:     logger,
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
// charge through the payment facade. Returns the checkout URL for the user to
// complete payment.
func (cs *CheckoutService) StartCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutResult, error) {
	if req.UserID == "" {
		return nil, fmt.Errorf("user ID is required")
	}
	if req.PlanID == "" {
		return nil, fmt.Errorf("plan ID is required")
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
		pricingPayload, _ := json.Marshal(map[string]interface{}{
			"invoice_id": inv.ID,
			"user_id":    req.UserID,
			"plan_id":    req.PlanID,
			"subtotal":   inv.Subtotal.Amount,
			"currency":   string(inv.Total.Currency),
		})
		if _, dispatchErr := cs.dispatcher.DispatchSync(ctx, "pricing.calculate", pricingPayload); dispatchErr != nil {
			cs.logger.Warn("pricing.calculate hook failed, using original price",
				slog.String("invoice_id", inv.ID),
				slog.Any("error", dispatchErr),
			)
		}
	}

	// 3. Initiate payment charge via the payment facade (dispatches to plugin).
	chargeResult, err := cs.payment.CreateCharge(ctx, payment.CreateChargeRequest{
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
