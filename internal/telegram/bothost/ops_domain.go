package bothost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// Sentinel errors for domain ops.
var (
	// ErrCapabilityUnavailable is returned when a required reader or runner port
	// is nil (not wired for this bot instance).
	ErrCapabilityUnavailable = errors.New("bothost: capability not wired for this bot")

	// ErrForbidden is returned when the resolved caller does not own the
	// requested resource.
	ErrForbidden = errors.New("bothost: caller does not own this resource")
)

// Domain op name constants — the public op-catalog ABI. Bot plugins MUST
// reference these exported consts; never duplicate the raw strings.
const (
	OpPlansList            = "plans.list"
	OpPlansGet             = "plans.get"
	OpSubscriptionsMine    = "subscriptions.mine"
	OpSubscriptionsGet     = "subscriptions.get"
	OpSubscriptionsCancel  = "subscriptions.cancel"
	OpSubscriptionsUpgrade = "subscriptions.upgrade"
	OpInvoicesPending      = "invoices.pending"
	OpBalanceGet           = "balance.get"
	OpCheckoutCreate       = "checkout.create"
)

// --- Argument structs ---

type plansListArgs struct {
	Channel string `json:"channel"`
	// Optional personalization: when set, the tariff reader runs the pricing
	// pipeline (geo/promo/loyalty) instead of returning list prices.
	TelegramID int64  `json:"telegram_id,omitempty"`
	Country    string `json:"country,omitempty"`
	Promo      string `json:"promo,omitempty"`
}

type plansGetArgs struct {
	PlanID     string `json:"plan_id"`
	TelegramID int64  `json:"telegram_id,omitempty"`
	Country    string `json:"country,omitempty"`
	Promo      string `json:"promo,omitempty"`
}

type subscriptionsMineArgs struct {
	TelegramID int64 `json:"telegram_id"`
}

type subscriptionsGetArgs struct {
	TelegramID int64  `json:"telegram_id"`
	ID         string `json:"id"`
}

type subscriptionsCancelArgs struct {
	TelegramID int64   `json:"telegram_id"`
	ID         string  `json:"id"`
	Reason     *string `json:"reason,omitempty"`
}

type subscriptionsUpgradeArgs struct {
	TelegramID int64  `json:"telegram_id"`
	ID         string `json:"id"`
	NewPlanID  string `json:"new_plan_id"`
}

type invoicesPendingArgs struct {
	TelegramID int64 `json:"telegram_id"`
}

type balanceGetArgs struct {
	TelegramID int64 `json:"telegram_id"`
}

type checkoutCreateArgs struct {
	TelegramID int64  `json:"telegram_id"`
	PlanID     string `json:"plan_id"`
	Email      string `json:"email,omitempty"`
	Country    string `json:"country,omitempty"`
}

// inTenantTx executes fn inside a transaction whose context carries the shop
// tenant GUC. ALL RLS-filtered reads (tariffs, subscriptions, invoices, wallets)
// must run through this helper so the GUC is active for the duration of the call.
// Returns ErrCapabilityUnavailable when TxRunner is nil.
func inTenantTx(ctx context.Context, oc *OpContext, fn func(ctx context.Context) error) error {
	if oc.TxRunner == nil {
		return ErrCapabilityUnavailable
	}
	return oc.TxRunner.RunInTx(tenantctx.WithTenantID(ctx, oc.TenantID), fn)
}

// resolveUser maps a Telegram user id to the shop-scoped platform user id.
// GetByTelegramID does not set the tenant GUC itself, so the lookup MUST run
// inside RunInTx(WithTenantID) or RLS would hide the shop's customers.
func resolveUser(ctx context.Context, oc *OpContext, telegramID int64) (string, error) {
	if oc.Identity == nil {
		return "", ErrCapabilityUnavailable
	}
	var userID string
	err := inTenantTx(ctx, oc, func(txCtx context.Context) error {
		u, err := oc.Identity.GetByTelegramID(txCtx, telegramID)
		if err != nil {
			return err
		}
		userID = u.ID
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("resolve user: %w", err)
	}
	return userID, nil
}

// RegisterDomainOps installs the seven domain host operations into r. Call once
// at startup after RegisterStdOps.
func RegisterDomainOps(r *Registry) {
	r.Register(OpPlansList, plugin.PermBillingRead, handlePlansList)
	r.Register(OpPlansGet, plugin.PermBillingRead, handlePlansGet)
	r.Register(OpSubscriptionsMine, plugin.PermBillingRead, handleSubscriptionsMine)
	r.Register(OpSubscriptionsGet, plugin.PermBillingRead, handleSubscriptionsGet)
	r.Register(OpSubscriptionsCancel, plugin.PermBillingWrite, handleSubscriptionsCancel)
	r.Register(OpSubscriptionsUpgrade, plugin.PermBillingWrite, handleSubscriptionsUpgrade)
	r.Register(OpInvoicesPending, plugin.PermBillingRead, handleInvoicesPending)
	r.Register(OpBalanceGet, plugin.PermBillingRead, handleBalanceGet)
	r.Register(OpCheckoutCreate, plugin.PermPaymentWrite, handleCheckoutCreate)
}

func handlePlansList(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a plansListArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%s: decode args: %w", OpPlansList, err)
	}
	if oc.Tariffs == nil {
		return nil, ErrCapabilityUnavailable
	}
	ch := a.Channel
	if ch == "" {
		ch = ChannelTelegram
	}
	ctx = withPriceHints(ctx, oc, a.TelegramID, a.Country, a.Promo)
	var offers []TariffOffer
	if err := inTenantTx(ctx, oc, func(txCtx context.Context) error {
		var err error
		offers, err = oc.Tariffs.ListVisible(txCtx, ch)
		return err
	}); err != nil {
		return nil, fmt.Errorf("%s: %w", OpPlansList, err)
	}
	return json.Marshal(offers)
}

// withPriceHints resolves the optional personalization inputs into a ctx-carried
// PriceHints the tariff reader consumes. telegramID>0 is resolved to a platform
// user id (best-effort; a resolution error just leaves UserID empty so geo/promo
// pricing still applies). Returns the original ctx when no hint is present.
func withPriceHints(ctx context.Context, oc *OpContext, telegramID int64, country, promo string) context.Context {
	hints := PriceHints{Country: country, PromoCode: promo}
	if telegramID != 0 {
		if userID, err := resolveUser(ctx, oc, telegramID); err == nil {
			hints.UserID = userID
		}
	}
	if hints.IsZero() {
		return ctx
	}
	return WithPriceHints(ctx, hints)
}

func handlePlansGet(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a plansGetArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%s: decode args: %w", OpPlansGet, err)
	}
	if oc.Tariffs == nil {
		return nil, ErrCapabilityUnavailable
	}
	ctx = withPriceHints(ctx, oc, a.TelegramID, a.Country, a.Promo)
	var offer *TariffOffer
	if err := inTenantTx(ctx, oc, func(txCtx context.Context) error {
		var err error
		offer, err = oc.Tariffs.Get(txCtx, a.PlanID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("%s: %w", OpPlansGet, err)
	}
	return json.Marshal(offer)
}

func handleSubscriptionsMine(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a subscriptionsMineArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%s: decode args: %w", OpSubscriptionsMine, err)
	}
	if oc.Subs == nil {
		return nil, ErrCapabilityUnavailable
	}
	userID, err := resolveUser(ctx, oc, a.TelegramID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", OpSubscriptionsMine, err)
	}
	var subs []Subscription
	if err := inTenantTx(ctx, oc, func(txCtx context.Context) error {
		var err error
		subs, err = oc.Subs.ActiveByUser(txCtx, userID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("%s: %w", OpSubscriptionsMine, err)
	}
	return json.Marshal(subs)
}

func handleSubscriptionsGet(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a subscriptionsGetArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%s: decode args: %w", OpSubscriptionsGet, err)
	}
	if oc.Subs == nil {
		return nil, ErrCapabilityUnavailable
	}
	userID, err := resolveUser(ctx, oc, a.TelegramID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", OpSubscriptionsGet, err)
	}
	var sub *Subscription
	var ownerID string
	if err := inTenantTx(ctx, oc, func(txCtx context.Context) error {
		var err error
		sub, ownerID, err = oc.Subs.Get(txCtx, a.ID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("%s: %w", OpSubscriptionsGet, err)
	}
	if ownerID != userID {
		return nil, ErrForbidden
	}
	return json.Marshal(sub)
}

// verifyOwnership resolves the caller and confirms they own subscription subID.
// The ownership read runs inside a tenant tx (Subs.Get needs the shop GUC).
// Returns ErrForbidden on mismatch. The subscription reader must be wired.
func verifyOwnership(ctx context.Context, oc *OpContext, telegramID int64, subID string) error {
	if oc.Subs == nil {
		return ErrCapabilityUnavailable
	}
	userID, err := resolveUser(ctx, oc, telegramID)
	if err != nil {
		return err
	}
	var ownerID string
	if err := inTenantTx(ctx, oc, func(txCtx context.Context) error {
		_, ownerID, err = oc.Subs.Get(txCtx, subID)
		return err
	}); err != nil {
		return err
	}
	if ownerID != userID {
		return ErrForbidden
	}
	return nil
}

func handleSubscriptionsCancel(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a subscriptionsCancelArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%s: decode args: %w", OpSubscriptionsCancel, err)
	}
	if oc.SubMutator == nil {
		return nil, ErrCapabilityUnavailable
	}
	if err := verifyOwnership(ctx, oc, a.TelegramID, a.ID); err != nil {
		return nil, fmt.Errorf("%s: %w", OpSubscriptionsCancel, err)
	}
	// Cancel self-wraps its own RunInTx; pass a tenant-annotated ctx (so its
	// writes stamp the shop tenant) but do NOT wrap in an outer RunInTx.
	if err := oc.SubMutator.Cancel(tenantctx.WithTenantID(ctx, oc.TenantID), a.ID, a.Reason); err != nil {
		return nil, fmt.Errorf("%s: %w", OpSubscriptionsCancel, err)
	}
	return json.Marshal(map[string]string{"id": a.ID, "status": "cancelled"})
}

func handleSubscriptionsUpgrade(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a subscriptionsUpgradeArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%s: decode args: %w", OpSubscriptionsUpgrade, err)
	}
	if oc.SubMutator == nil || oc.Tariffs == nil {
		return nil, ErrCapabilityUnavailable
	}
	if err := verifyOwnership(ctx, oc, a.TelegramID, a.ID); err != nil {
		return nil, fmt.Errorf("%s: %w", OpSubscriptionsUpgrade, err)
	}
	// Tenant-visibility guard: the target plan must be visible to THIS shop
	// (plans resolve from a global catalog; without this a forged new_plan_id
	// could upgrade to another shop's plan). Tariffs.Get is tenant-scoped.
	if err := inTenantTx(ctx, oc, func(txCtx context.Context) error {
		_, err := oc.Tariffs.Get(txCtx, a.NewPlanID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("%s: %w", OpSubscriptionsUpgrade, err)
	}
	// Upgrade self-wraps its own RunInTx; pass tenant-annotated ctx, no outer wrap.
	if err := oc.SubMutator.Upgrade(tenantctx.WithTenantID(ctx, oc.TenantID), a.ID, a.NewPlanID); err != nil {
		return nil, fmt.Errorf("%s: %w", OpSubscriptionsUpgrade, err)
	}
	return json.Marshal(map[string]string{"id": a.ID, "status": "upgraded", "plan_id": a.NewPlanID})
}

func handleInvoicesPending(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a invoicesPendingArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%s: decode args: %w", OpInvoicesPending, err)
	}
	if oc.Invoices == nil {
		return nil, ErrCapabilityUnavailable
	}
	userID, err := resolveUser(ctx, oc, a.TelegramID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", OpInvoicesPending, err)
	}
	var invoices []Invoice
	if err := inTenantTx(ctx, oc, func(txCtx context.Context) error {
		var err error
		invoices, err = oc.Invoices.PendingByUser(txCtx, userID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("%s: %w", OpInvoicesPending, err)
	}
	return json.Marshal(invoices)
}

func handleBalanceGet(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a balanceGetArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%s: decode args: %w", OpBalanceGet, err)
	}
	if oc.Balance == nil {
		return nil, ErrCapabilityUnavailable
	}
	userID, err := resolveUser(ctx, oc, a.TelegramID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", OpBalanceGet, err)
	}
	var wallets []Wallet
	if err := inTenantTx(ctx, oc, func(txCtx context.Context) error {
		var err error
		wallets, err = oc.Balance.WalletsByUser(txCtx, userID)
		return err
	}); err != nil {
		return nil, fmt.Errorf("%s: %w", OpBalanceGet, err)
	}
	return json.Marshal(wallets)
}

// handleCheckoutCreate resolves the calling user (inside tenant tx, per
// resolveUser contract) then starts the checkout flow with a tenant-SCOPED but
// UNWRAPPED ctx. oc.Checkout.Start self-wraps its own RunInTx internally, so we
// must NOT wrap it in another RunInTx (that would break the tx-manager
// re-entrancy contract) — but we DO annotate the ctx with the shop's tenant so
// the subscription/invoice INSERTs (which stamp tenant_id from the GUC) are
// owned by this shop rather than defaulting to NULL/platform-owned.
func handleCheckoutCreate(ctx context.Context, oc *OpContext, args json.RawMessage) (json.RawMessage, error) {
	var a checkoutCreateArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("%s: decode args: %w", OpCheckoutCreate, err)
	}
	if oc.Checkout == nil {
		return nil, ErrCapabilityUnavailable
	}
	userID, err := resolveUser(ctx, oc, a.TelegramID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", OpCheckoutCreate, err)
	}
	scopedCtx := tenantctx.WithTenantID(ctx, oc.TenantID)
	result, err := oc.Checkout.Start(scopedCtx, CheckoutInput{
		UserID:      userID,
		PlanID:      a.PlanID,
		UserEmail:   a.Email,
		UserCountry: a.Country,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", OpCheckoutCreate, err)
	}
	return json.Marshal(result)
}
