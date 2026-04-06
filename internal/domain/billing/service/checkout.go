package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
)

// HookPricingCalculate is the plugin hook dispatched to modify invoice pricing.
const HookPricingCalculate = "pricing.calculate"

// PricingHookResult is the expected response from the pricing.calculate hook.
// Pointer fields distinguish "not set" from "zero".
type PricingHookResult struct {
	Subtotal *int64 `json:"subtotal,omitempty"`
	Discount *int64 `json:"discount,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// CheckoutService orchestrates the full checkout flow: subscription creation,
// invoice generation, and payment charge initiation via the payment gateway.
//
// # Architectural rationale
//
// CheckoutService depends directly on hookdispatch.Dispatcher for the
// pricing.calculate hook. This is a pragmatic trade-off:
//
//   - hookdispatch.Dispatcher is a shared kernel package (pkg/hookdispatch), not
//     an infrastructure adapter. It defines a pure Go interface with no external
//     dependencies. Importing it from a domain service package follows the same
//     dependency direction as importing stdlib or pkg/domainevent.
//
//   - The pricing hook is best-effort and optional: if no plugin is registered or
//     the hook errors, the checkout proceeds with the original price. Extracting a
//     PricingPort interface would add a file that delegates to the same dispatcher
//     method with identical semantics.
//
//   - Plugin version pinning (BeginFlow) is called at the start of the checkout
//     flow to guarantee consistency across multiple hook calls. This is a
//     dispatcher concern that would leak into the adapter regardless.
//
// Rationale: in a modular monolith compiled as a single binary, a
// PricingDispatcher port + adapter would be pure delegation with no decoupling
// benefit. The trade-off is documented rather than abstracted.
//
// When to split: if pricing logic grows beyond a single hook dispatch (e.g.,
// tiered pricing engine, A/B testing, coupon validation), extract a
// PricingService that encapsulates that complexity and inject it here.
type CheckoutService struct {
	billing     *BillingService
	payment     billing.PaymentGateway
	dispatcher  hookdispatch.Dispatcher
	publisher   domainevent.Publisher
	logger      *slog.Logger
	rateLimiter billing.DomainRateLimiter
	clock       clock.Clock
}

// NewCheckoutService creates a CheckoutService with the given dependencies.
func NewCheckoutService(
	billingSvc *BillingService,
	paymentGateway billing.PaymentGateway,
	dispatcher hookdispatch.Dispatcher,
	publisher domainevent.Publisher,
	logger *slog.Logger,
	rateLimiter billing.DomainRateLimiter,
	clk clock.Clock,
) *CheckoutService {
	return &CheckoutService{
		billing:     billingSvc,
		payment:     paymentGateway,
		dispatcher:  dispatcher,
		publisher:   publisher,
		logger:      logger,
		rateLimiter: rateLimiter,
		clock:       clk,
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
		output, dispatchErr := cs.dispatcher.DispatchSync(ctx, HookPricingCalculate, pricingPayload)
		if dispatchErr != nil {
			cs.logger.Warn("pricing.calculate hook failed, using original price",
				slog.String("invoice_id", inv.ID),
				slog.Any("error", dispatchErr),
			)
		} else if output != nil {
			var pricingResult PricingHookResult
			if unmarshalErr := json.Unmarshal(output, &pricingResult); unmarshalErr != nil {
				cs.logger.Warn("pricing.calculate returned invalid JSON, using original price",
					slog.String("invoice_id", inv.ID),
					slog.Any("error", unmarshalErr),
				)
			} else {
				if pricingErr := inv.ApplyPricingModification(pricingResult.Subtotal, pricingResult.Discount, pricingResult.Reason, cs.clock.Now()); pricingErr != nil {
					cs.logger.Warn("pricing.calculate modification rejected",
						slog.String("invoice_id", inv.ID),
						slog.Any("error", pricingErr),
					)
				}
			}
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
