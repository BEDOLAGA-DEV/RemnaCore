package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// pluginBlockedCheckoutReason is the default reason used when a plugin blocks
// checkout but does not provide a custom reason.
const pluginBlockedCheckoutReason = "checkout blocked by plugin"

// CheckoutService orchestrates the full checkout flow: subscription creation,
// invoice generation, and payment charge initiation via the payment gateway.
//
// # Architectural rationale
//
// CheckoutService delegates pricing modifications to a billing.PricingModifier
// port. The adapter (internal/app/pricing_modifier_adapter.go) handles the
// WASM plugin wire protocol (JSON marshal/unmarshal, hookdispatch), keeping
// the billing domain free of encoding and plugin concerns for the pricing hook.
//
// Checkout lifecycle hooks (checkout.validating, subscription.creating,
// checkout.completed) are dispatched via injected SyncHookFn / AsyncHookFn
// callbacks, keeping the billing domain free of hookdispatch and sdk imports.
// The concrete implementations are wired in internal/app/wiring_billing.go.
//
// The pricing hook is best-effort and optional: if no plugin is registered or
// the hook errors, the checkout proceeds with the original price.
//
// Plugin version pinning (BeginFlow) is called at the start of the checkout
// flow to guarantee consistency across multiple hook calls within the flow.
// This is exposed through the PricingModifier port so that the checkout
// service does not depend on the hookdispatch package for version pinning.
type CheckoutService struct {
	billing       *BillingService
	invoiceReader billing.InvoiceReader
	subReader     billing.SubscriptionReader
	payment       billing.PaymentGateway
	pricing       billing.PricingModifier
	publisher     domainevent.Publisher
	logger        *slog.Logger
	rateLimiter   billing.DomainRateLimiter
	clock         clock.Clock
	syncHook      SyncHookFn  // nil-safe; all dispatches guarded
	asyncHook     AsyncHookFn // nil-safe; all dispatches guarded
}

// CheckoutServiceOption configures optional dependencies on CheckoutService.
type CheckoutServiceOption func(*CheckoutService)

// WithCheckoutSyncHook sets the synchronous hook dispatch function for checkout
// lifecycle hooks (e.g., checkout.validating, subscription.creating).
func WithCheckoutSyncHook(fn SyncHookFn) CheckoutServiceOption {
	return func(cs *CheckoutService) { cs.syncHook = fn }
}

// WithCheckoutAsyncHook sets the asynchronous hook dispatch function for
// post-checkout notifications (e.g., checkout.completed).
func WithCheckoutAsyncHook(fn AsyncHookFn) CheckoutServiceOption {
	return func(cs *CheckoutService) { cs.asyncHook = fn }
}

// NewCheckoutService creates a CheckoutService with the given dependencies.
// Optional dependencies (hook functions) are configured via
// CheckoutServiceOption functions to keep the constructor backward-compatible.
func NewCheckoutService(
	billingSvc *BillingService,
	invoiceReader billing.InvoiceReader,
	subReader billing.SubscriptionReader,
	paymentGateway billing.PaymentGateway,
	pricing billing.PricingModifier,
	publisher domainevent.Publisher,
	logger *slog.Logger,
	rateLimiter billing.DomainRateLimiter,
	clk clock.Clock,
	opts ...CheckoutServiceOption,
) *CheckoutService {
	cs := &CheckoutService{
		billing:       billingSvc,
		invoiceReader: invoiceReader,
		subReader:     subReader,
		payment:       paymentGateway,
		pricing:       pricing,
		publisher:     publisher,
		logger:        logger,
		rateLimiter:   rateLimiter,
		clock:         clk,
	}
	for _, opt := range opts {
		opt(cs)
	}
	return cs
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
//
// Hook dispatch order:
//  1. checkout.validating (sync) -- can block the checkout
//  2. subscription.creating (sync) -- can modify addonIDs (best-effort)
//  3. pricing.calculate (sync, via PricingModifier) -- can modify price
func (cs *CheckoutService) StartCheckout(ctx context.Context, req CheckoutRequest) (*CheckoutResult, error) {
	if req.UserID == "" {
		return nil, aggregate.ErrEmptyUserID
	}
	if req.PlanID == "" {
		return nil, aggregate.ErrEmptyPlanID
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
	if cs.pricing != nil {
		ctx = cs.pricing.BeginFlow(ctx)
	}

	// Effective addon IDs may be modified by hooks below.
	addonIDs := req.AddonIDs

	// --- Hook: checkout.validating (sync, can block) ---
	if blocked, blockErr := cs.dispatchCheckoutValidating(ctx, req); blocked {
		return nil, blockErr
	}

	// --- Hook: subscription.creating (sync, best-effort) ---
	addonIDs = cs.dispatchSubscriptionCreating(ctx, req, addonIDs)

	// 1. Create subscription + invoice via billing service.
	sub, inv, err := cs.billing.CreateSubscription(ctx, CreateSubscriptionCmd{
		UserID:   req.UserID,
		PlanID:   req.PlanID,
		AddonIDs: addonIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("create subscription: %w", err)
	}

	// 2. Run pricing plugins (can modify final price). Best-effort: if no
	//    handler is registered or the hook errors, proceed with the original price.
	if cs.pricing != nil {
		cs.applyPricingModification(ctx, inv, req)
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

// applyPricingModification dispatches a pricing modification request to the
// pricing modifier and applies any resulting modification to the invoice. This
// is best-effort: errors are logged and the original price is preserved.
func (cs *CheckoutService) applyPricingModification(ctx context.Context, inv *aggregate.Invoice, req CheckoutRequest) {
	mod, err := cs.pricing.ModifyPricing(ctx, inv.ID, req.UserID, req.PlanID, inv.Subtotal.Amount, string(inv.Total.Currency))
	if err != nil {
		cs.logger.Warn("pricing modification failed, using original price",
			slog.String("invoice_id", inv.ID),
			slog.Any("error", err),
		)
		return
	}

	if mod == nil {
		return
	}

	if pricingErr := inv.ApplyPricingModification(mod.Subtotal, mod.Discount, mod.Reason, cs.clock.Now()); pricingErr != nil {
		cs.logger.Warn("pricing modification rejected",
			slog.String("invoice_id", inv.ID),
			slog.Any("error", pricingErr),
		)
	}
}

// CompleteCheckout is called when a payment webhook confirms success. It marks
// the invoice as paid (which activates the subscription if in trial/past_due),
// then fires the checkout.completed async hook for analytics/notifications.
func (cs *CheckoutService) CompleteCheckout(ctx context.Context, invoiceID string) error {
	if invoiceID == "" {
		return billing.ErrEmptyInvoiceID
	}

	if err := cs.billing.PayInvoice(ctx, invoiceID); err != nil {
		return fmt.Errorf("pay invoice: %w", err)
	}

	cs.logger.Info("checkout completed", slog.String("invoice_id", invoiceID))

	// --- Hook: checkout.completed (async, fire-and-forget) ---
	// Read invoice to populate notification payload. Best-effort: if the read
	// fails, the checkout is still considered successful -- only the async
	// notification is lost.
	cs.dispatchCheckoutCompleted(ctx, invoiceID)

	return nil
}

// dispatchCheckoutValidating dispatches the checkout.validating sync hook.
// Returns (true, error) if the plugin blocked the checkout, (false, nil)
// otherwise. Best-effort: dispatch errors are logged and the checkout proceeds.
func (cs *CheckoutService) dispatchCheckoutValidating(ctx context.Context, req CheckoutRequest) (bool, error) {
	if cs.syncHook == nil {
		return false, nil
	}

	hookResp, err := cs.syncHook(ctx, HookCheckoutValidating, CheckoutValidatingPayload{
		UserID:    req.UserID,
		PlanID:    req.PlanID,
		AddonIDs:  req.AddonIDs,
		UserEmail: req.UserEmail,
	})
	if err != nil {
		cs.logger.Warn("checkout.validating hook failed, proceeding",
			slog.Any("error", err),
		)
		return false, nil
	}

	if hookResp == nil {
		return false, nil
	}

	var resp CheckoutValidatingResponse
	if err := json.Unmarshal(hookResp, &resp); err != nil {
		cs.logger.Warn("failed to unmarshal checkout.validating response, proceeding",
			slog.Any("error", err),
		)
		return false, nil
	}

	if !resp.Allowed {
		reason := pluginBlockedCheckoutReason
		if resp.Reason != nil {
			reason = *resp.Reason
		}
		return true, fmt.Errorf("%s: %w", reason, billing.ErrCheckoutBlocked)
	}

	return false, nil
}

// dispatchSubscriptionCreating dispatches the subscription.creating sync hook.
// Returns possibly-modified addonIDs. Best-effort: dispatch errors fall back
// to the original addonIDs.
func (cs *CheckoutService) dispatchSubscriptionCreating(ctx context.Context, req CheckoutRequest, addonIDs []string) []string {
	if cs.syncHook == nil {
		return addonIDs
	}

	hookResp, err := cs.syncHook(ctx, HookSubscriptionCreating, SubCreatingPayload{
		UserID:   req.UserID,
		PlanID:   req.PlanID,
		AddonIDs: addonIDs,
	})
	if err != nil {
		cs.logger.Warn("subscription.creating hook failed, proceeding with defaults",
			slog.Any("error", err),
		)
		return addonIDs
	}

	if hookResp == nil {
		return addonIDs
	}

	var resp SubCreatingResponse
	if err := json.Unmarshal(hookResp, &resp); err != nil {
		cs.logger.Warn("failed to unmarshal subscription.creating response, proceeding with defaults",
			slog.Any("error", err),
		)
		return addonIDs
	}

	if len(resp.AddonIDs) > 0 {
		return resp.AddonIDs
	}

	return addonIDs
}

// dispatchCheckoutCompleted fires the checkout.completed async hook after a
// successful payment. Best-effort: if the invoice cannot be read or the hook
// fails to dispatch, the error is logged but the checkout remains successful.
func (cs *CheckoutService) dispatchCheckoutCompleted(ctx context.Context, invoiceID string) {
	if cs.asyncHook == nil {
		return
	}

	inv, err := cs.invoiceReader.GetByID(ctx, invoiceID)
	if err != nil {
		cs.logger.Warn("failed to read invoice for checkout.completed hook",
			slog.String("invoice_id", invoiceID),
			slog.Any("error", err),
		)
		return
	}

	sub, err := cs.subReader.GetByID(ctx, inv.SubscriptionID)
	if err != nil {
		cs.logger.Warn("failed to read subscription for checkout.completed hook",
			slog.String("subscription_id", inv.SubscriptionID),
			slog.Any("error", err),
		)
		return
	}

	cs.asyncHook(ctx, HookCheckoutCompleted, CheckoutCompletedPayload{
		SubscriptionID: sub.ID,
		InvoiceID:      inv.ID,
		UserID:         inv.UserID,
		PlanID:         sub.PlanID,
		AmountCents:    inv.Total.Amount,
		Currency:       string(inv.Total.Currency),
	})
}
