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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// Hook names dispatched by CheckoutService during the checkout flow.
const (
	// HookCheckoutValidating is dispatched before CreateSubscription to allow
	// plugins to perform custom eligibility checks (e.g., geo-blocking,
	// anti-fraud). Sync hook: a plugin can block the checkout.
	HookCheckoutValidating = "checkout.validating"

	// HookSubscriptionCreating is dispatched before CreateSubscription to allow
	// plugins to modify subscription parameters (trial_days, addon_ids).
	// Sync hook: best-effort, errors fall back to defaults.
	HookSubscriptionCreating = "subscription.creating"

	// HookCheckoutCompleted is dispatched after a successful CompleteCheckout
	// transaction. Async hook: fire-and-forget for analytics/notifications.
	HookCheckoutCompleted = "checkout.completed"
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
// checkout.completed) dispatch directly via hookdispatch.Dispatcher from pkg/,
// which is a shared kernel dependency (same direction as stdlib imports).
// This follows the pattern established by BillingService.dispatchHook and
// PaymentFacade.
//
// The pricing hook is best-effort and optional: if no plugin is registered or
// the hook errors, the checkout proceeds with the original price.
//
// Plugin version pinning (BeginFlow) is called at the start of the checkout
// flow to guarantee consistency across multiple hook calls within the flow.
// This is exposed through the PricingModifier port so that the checkout
// service does not depend on the hookdispatch package for version pinning.
type CheckoutService struct {
	billing      *BillingService
	payment      billing.PaymentGateway
	pricing      billing.PricingModifier
	publisher    domainevent.Publisher
	logger       *slog.Logger
	rateLimiter  billing.DomainRateLimiter
	clock        clock.Clock
	dispatcher   hookdispatch.Dispatcher // nil-safe; all dispatches guarded
	hooksEnabled bool                    // checkout lifecycle hooks feature flag
}

// CheckoutServiceOption configures optional dependencies on CheckoutService.
type CheckoutServiceOption func(*CheckoutService)

// WithCheckoutDispatcher sets the WASM hook dispatcher for checkout lifecycle hooks.
func WithCheckoutDispatcher(d hookdispatch.Dispatcher) CheckoutServiceOption {
	return func(cs *CheckoutService) { cs.dispatcher = d }
}

// WithCheckoutHooksEnabled enables checkout lifecycle hook dispatch points.
func WithCheckoutHooksEnabled(enabled bool) CheckoutServiceOption {
	return func(cs *CheckoutService) { cs.hooksEnabled = enabled }
}

// NewCheckoutService creates a CheckoutService with the given dependencies.
// Optional dependencies (dispatcher, feature flags) are configured via
// CheckoutServiceOption functions to keep the constructor backward-compatible.
func NewCheckoutService(
	billingSvc *BillingService,
	paymentGateway billing.PaymentGateway,
	pricing billing.PricingModifier,
	publisher domainevent.Publisher,
	logger *slog.Logger,
	rateLimiter billing.DomainRateLimiter,
	clk clock.Clock,
	opts ...CheckoutServiceOption,
) *CheckoutService {
	cs := &CheckoutService{
		billing:     billingSvc,
		payment:     paymentGateway,
		pricing:     pricing,
		publisher:   publisher,
		logger:      logger,
		rateLimiter: rateLimiter,
		clock:       clk,
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
		return fmt.Errorf("invoice ID is required")
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
	if cs.dispatcher == nil || !cs.hooksEnabled {
		return false, nil
	}

	validReq := sdk.CheckoutValidatingRequest{
		UserID:    req.UserID,
		PlanID:    req.PlanID,
		AddonIDs:  req.AddonIDs,
		UserEmail: req.UserEmail,
	}

	data, err := json.Marshal(validReq)
	if err != nil {
		cs.logger.Warn("failed to marshal checkout.validating payload",
			slog.Any("error", err),
		)
		return false, nil
	}

	result := cs.dispatcher.DispatchSyncSafe(ctx, HookCheckoutValidating, data)
	if result.Err != nil {
		cs.logger.Warn("checkout.validating hook failed, proceeding",
			slog.Any("error", result.Err),
		)
		return false, nil
	}

	if result.Payload == nil {
		return false, nil
	}

	var resp sdk.CheckoutValidatingResponse
	if err := json.Unmarshal(result.Payload, &resp); err != nil {
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
	if cs.dispatcher == nil || !cs.hooksEnabled {
		return addonIDs
	}

	creatingReq := sdk.SubCreatingRequest{
		UserID:   req.UserID,
		PlanID:   req.PlanID,
		AddonIDs: addonIDs,
		// Interval is not available at this point in the checkout flow;
		// BillingService.CreateSubscription resolves it from the plan.
	}

	data, err := json.Marshal(creatingReq)
	if err != nil {
		cs.logger.Warn("failed to marshal subscription.creating payload",
			slog.Any("error", err),
		)
		return addonIDs
	}

	result := cs.dispatcher.DispatchSyncSafe(ctx, HookSubscriptionCreating, data)
	if result.Err != nil {
		cs.logger.Warn("subscription.creating hook failed, proceeding with defaults",
			slog.Any("error", result.Err),
		)
		return addonIDs
	}

	if result.Payload == nil {
		return addonIDs
	}

	var resp sdk.SubCreatingResponse
	if err := json.Unmarshal(result.Payload, &resp); err != nil {
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
	if cs.dispatcher == nil || !cs.hooksEnabled {
		return
	}

	inv, err := cs.billing.invoices.GetByID(ctx, invoiceID)
	if err != nil {
		cs.logger.Warn("failed to read invoice for checkout.completed hook",
			slog.String("invoice_id", invoiceID),
			slog.Any("error", err),
		)
		return
	}

	sub, err := cs.billing.subs.GetByID(ctx, inv.SubscriptionID)
	if err != nil {
		cs.logger.Warn("failed to read subscription for checkout.completed hook",
			slog.String("subscription_id", inv.SubscriptionID),
			slog.Any("error", err),
		)
		return
	}

	notification := sdk.CheckoutCompletedNotification{
		SubscriptionID: sub.ID,
		InvoiceID:      inv.ID,
		UserID:         inv.UserID,
		PlanID:         sub.PlanID,
		AmountCents:    inv.Total.Amount,
		Currency:       string(inv.Total.Currency),
	}

	data, err := json.Marshal(notification)
	if err != nil {
		cs.logger.Warn("failed to marshal checkout.completed payload",
			slog.Any("error", err),
		)
		return
	}

	cs.dispatcher.DispatchAsync(ctx, HookCheckoutCompleted, data)
}
