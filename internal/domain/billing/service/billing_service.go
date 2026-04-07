package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// CreateSubscriptionCmd holds the parameters for creating a new subscription.
type CreateSubscriptionCmd struct {
	UserID      string
	PlanID      string
	AddonIDs    []string
	UserCountry string // optional; empty = all countries allowed
}

// BillingService implements CQRS command handlers for the billing domain.
//
// # Architectural rationale
//
// BillingService is a "thick" domain service that combines domain logic
// (aggregate construction, state transitions, invariant enforcement) with
// application orchestration (transaction management, event publishing,
// repository coordination). In strict hexagonal architecture these would be
// separate layers, but RemnaCore intentionally merges them:
//
//   - Every public method follows the same pattern: load aggregates, enforce
//     invariants via aggregate methods, persist changes and publish events within
//     a single database transaction (txmanager.Runner). Splitting this into a
//     thin domain service + application service would duplicate the aggregate
//     load/validate/persist/publish ceremony in every method.
//
//   - Aggregates (Subscription, Invoice, FamilyGroup) own their own state
//     machines and domain events. BillingService orchestrates them but does not
//     contain business rules -- those live in the aggregates and value objects.
//
//   - Transaction boundaries are a cross-cutting concern. Using txmanager.Runner
//     (a pkg/ interface) keeps the service testable without depending on concrete
//     database infrastructure.
//
// Rationale: in a modular monolith, the application service layer would be a
// thin pass-through that adds ceremony without adding separation. The thick
// service pattern keeps the billing context's public API in one place, making
// it easier to audit invariants and transaction boundaries.
//
// When to split: if BillingService exceeds ~500 lines or gains responsibilities
// beyond CRUD orchestration (e.g., complex saga coordination across contexts),
// extract focused services (e.g., RenewalService, FamilyService) that each own
// a subset of the aggregate interactions.
type BillingService struct {
	plans       billing.PlanRepository
	subs        billing.SubscriptionRepository
	invoices    billing.InvoiceRepository
	families    billing.FamilyRepository
	publisher   domainevent.Publisher
	prorate     *ProrateCalculator
	trial       *TrialManager
	txRunner    txmanager.Runner
	clock       clock.Clock
	logger      *slog.Logger
	cancelHook  CancelHookFn  // nil-safe; all dispatches guarded
	renewHook   RenewHookFn   // nil-safe; all dispatches guarded
	upgradeHook UpgradeHookFn // nil-safe; all dispatches guarded
	asyncHook   AsyncHookFn   // nil-safe; all dispatches guarded
}

// BillingServiceOption configures optional dependencies on BillingService.
type BillingServiceOption func(*BillingService)

// WithCancelHook sets the typed subscription.cancelling sync hook function.
func WithCancelHook(fn CancelHookFn) BillingServiceOption {
	return func(s *BillingService) { s.cancelHook = fn }
}

// WithRenewHook sets the typed subscription.renewing sync hook function.
func WithRenewHook(fn RenewHookFn) BillingServiceOption {
	return func(s *BillingService) { s.renewHook = fn }
}

// WithUpgradeHook sets the typed subscription.upgrading sync hook function.
func WithUpgradeHook(fn UpgradeHookFn) BillingServiceOption {
	return func(s *BillingService) { s.upgradeHook = fn }
}

// WithAsyncHook sets the asynchronous hook dispatch function for post-commit
// lifecycle notifications (e.g., subscription.cancelled.post). The function is
// called with a domain-local payload struct. Pass nil to disable.
func WithAsyncHook(fn AsyncHookFn) BillingServiceOption {
	return func(s *BillingService) { s.asyncHook = fn }
}

// BillingDeps groups the core dependencies for BillingService. Using a struct
// avoids a long positional parameter list and makes adding new dependencies a
// backward-compatible change.
type BillingDeps struct {
	Plans     billing.PlanRepository
	Subs      billing.SubscriptionRepository
	Invoices  billing.InvoiceRepository
	Families  billing.FamilyRepository
	Publisher domainevent.Publisher
	Prorate   *ProrateCalculator
	Trial     *TrialManager
	TxRunner  txmanager.Runner
	Clock     clock.Clock
	Logger    *slog.Logger
}

// NewBillingService creates a BillingService with the given dependencies.
// Optional dependencies (dispatcher, feature flags) are configured via
// BillingServiceOption functions to keep the constructor backward-compatible.
func NewBillingService(deps BillingDeps, opts ...BillingServiceOption) *BillingService {
	s := &BillingService{
		plans:     deps.Plans,
		subs:      deps.Subs,
		invoices:  deps.Invoices,
		families:  deps.Families,
		publisher: deps.Publisher,
		prorate:   deps.Prorate,
		trial:     deps.Trial,
		txRunner:  deps.TxRunner,
		clock:     deps.Clock,
		logger:    deps.Logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// dispatchAsync fires an async hook via the injected AsyncHookFn if set.
// No-op when no hook function is configured.
func (s *BillingService) dispatchAsync(ctx context.Context, hookName string, payload any) {
	if s.asyncHook == nil {
		return
	}
	s.asyncHook(ctx, hookName, payload)
}

// CreateSubscription creates a new subscription and its initial invoice.
// If the plan supports trials, the subscription starts in trial status;
// otherwise it starts as active. The plan read, eligibility check,
// subscription, invoice, and outbox event are all performed inside a single
// database transaction to prevent TOCTOU races (e.g., plan deactivation or
// duplicate subscription between the read and the insert).
func (s *BillingService) CreateSubscription(
	ctx context.Context,
	cmd CreateSubscriptionCmd,
) (*aggregate.Subscription, *aggregate.Invoice, error) {
	ctx, span := tracing.StartSpan(ctx, "billing.create_subscription")
	defer span.End()

	var sub *aggregate.Subscription
	var inv *aggregate.Invoice

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		// Read plan inside tx — prevents deactivation race.
		plan, err := s.plans.GetByID(txCtx, cmd.PlanID)
		if err != nil {
			return fmt.Errorf("get plan: %w", err)
		}

		// Check for existing active subscription inside tx — prevents
		// duplicate race. The DB exclusion constraint is the safety net,
		// but the app-level check prevents wasted work.
		existingSubs, err := s.subs.GetActiveByUserID(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("check existing subscriptions: %w", err)
		}

		eligibility := aggregate.CheckoutEligibility{
			Plan:                  plan,
			HasActiveSubscription: len(existingSubs) > 0,
			AllowedCountries:      plan.AllowedCountries,
			UserCountry:           cmd.UserCountry,
		}
		if err := eligibility.Check(); err != nil {
			return fmt.Errorf("checkout eligibility: %w", err)
		}

		// Create subscription (defaults to trial). The subscription.creating
		// hook is dispatched by CheckoutService during the checkout flow —
		// CreateSubscription does not dispatch it to avoid double-firing.
		now := s.clock.Now()
		sub, err = aggregate.NewSubscription(cmd.UserID, plan.ID, plan.Interval, cmd.AddonIDs, now)
		if err != nil {
			return fmt.Errorf("create subscription: %w", err)
		}

		// Build line items for the invoice.
		lineItems := buildLineItems(plan, cmd.AddonIDs, s.logger)

		inv, err = aggregate.NewInvoice(sub.ID, cmd.UserID, lineItems, nil, plan.BasePrice.Currency, now)
		if err != nil {
			return fmt.Errorf("create invoice: %w", err)
		}

		if err := s.subs.Create(txCtx, sub); err != nil {
			return fmt.Errorf("persist subscription: %w", err)
		}

		if err := s.invoices.Create(txCtx, inv); err != nil {
			return fmt.Errorf("persist invoice: %w", err)
		}

		if err := domainevent.PublishAll(txCtx, s.publisher, sub); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return sub, inv, nil
}

// CancelSubscription cancels an existing subscription. The read, status
// transition, update, and outbox event are all performed inside a single
// database transaction with a FOR UPDATE lock to prevent TOCTOU races.
//
// If hooks are enabled, the subscription.cancelling hook is dispatched before
// the aggregate transition. A plugin may block the cancellation by returning
// block: true in its response. After a successful commit, the
// subscription.cancelled.post async hook fires for notifications.
func (s *BillingService) CancelSubscription(ctx context.Context, subID string, reason *string) error {
	// asyncPayload is captured inside the tx for post-commit async dispatch.
	var asyncPayload *CancellingPayload

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		// Dispatch subscription.cancelling pre-hook.
		if s.cancelHook != nil {
			resp, _ := s.cancelHook(txCtx, CancellingPayload{
				SubscriptionID: sub.ID,
				UserID:         sub.UserID,
				PlanID:         sub.PlanID,
				Reason:         reason,
			})
			if resp != nil && resp.Block {
				return billing.ErrCancellationBlocked
			}
		}

		if err := sub.Cancel(s.clock.Now()); err != nil {
			return fmt.Errorf("cancel subscription: %w", err)
		}

		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		// Capture data for post-commit async hook.
		asyncPayload = &CancellingPayload{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			PlanID:         sub.PlanID,
			Reason:         reason,
		}

		return domainevent.PublishAll(txCtx, s.publisher, sub)
	})
	if err != nil {
		return err
	}

	// Fire async post-hook AFTER successful commit.
	if asyncPayload != nil {
		s.dispatchAsync(ctx, HookSubCancelledPost, *asyncPayload)
	}
	return nil
}

// PayInvoice marks an invoice as paid and activates the associated subscription
// if it is in trial or past_due status. All reads, state transitions, writes,
// and outbox events are performed inside a single database transaction with
// FOR UPDATE locks to prevent TOCTOU races.
//
// After a successful commit that activated the subscription, the
// subscription.activated.post async hook fires for notifications/analytics.
func (s *BillingService) PayInvoice(ctx context.Context, invoiceID string) error {
	// asyncPayload is captured inside the tx for post-commit async dispatch.
	var asyncPayload *ActivatedPayload

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inv, err := s.invoices.GetByIDForUpdate(txCtx, invoiceID)
		if err != nil {
			return fmt.Errorf("get invoice: %w", err)
		}

		if inv.Status == aggregate.InvoicePaid {
			return billing.ErrInvoiceAlreadyPaid
		}

		now := s.clock.Now()

		// Transition draft -> pending if still in draft
		if inv.Status == aggregate.InvoiceDraft {
			if err := inv.MarkPending(now); err != nil {
				return fmt.Errorf("mark pending: %w", err)
			}
		}

		if err := inv.MarkPaid(now); err != nil {
			return fmt.Errorf("mark paid: %w", err)
		}

		if err := s.invoices.Update(txCtx, inv); err != nil {
			return fmt.Errorf("update invoice: %w", err)
		}

		if err := domainevent.PublishAll(txCtx, s.publisher, inv); err != nil {
			return err
		}

		// Activate subscription if it is in trial or past_due
		sub, err := s.subs.GetByIDForUpdate(txCtx, inv.SubscriptionID)
		if err != nil {
			return fmt.Errorf("get subscription for activation: %w", err)
		}

		if sub.Status == aggregate.StatusTrial || sub.Status == aggregate.StatusPastDue {
			if err := sub.Activate(now); err != nil {
				return fmt.Errorf("activate subscription: %w", err)
			}

			if err := s.subs.Update(txCtx, sub); err != nil {
				return fmt.Errorf("update subscription: %w", err)
			}

			if err := domainevent.PublishAll(txCtx, s.publisher, sub); err != nil {
				return err
			}

			// Capture data for post-commit async hook.
			asyncPayload = &ActivatedPayload{
				SubscriptionID: sub.ID,
				UserID:         sub.UserID,
				PlanID:         sub.PlanID,
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Fire async post-hook AFTER successful commit.
	if asyncPayload != nil {
		s.dispatchAsync(ctx, HookSubActivatedPost, *asyncPayload)
	}
	return nil
}
