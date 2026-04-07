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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// CreateSubscriptionCmd holds the parameters for creating a new subscription.
type CreateSubscriptionCmd struct {
	UserID   string
	PlanID   string
	AddonIDs []string
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
	plans      billing.PlanRepository
	subs       billing.SubscriptionRepository
	invoices   billing.InvoiceRepository
	families   billing.FamilyRepository
	publisher  domainevent.Publisher
	prorate    *ProrateCalculator
	trial      *TrialManager
	txRunner   txmanager.Runner
	clock     clock.Clock
	logger    *slog.Logger
	syncHook  SyncHookFn  // nil-safe; all dispatches guarded
	asyncHook AsyncHookFn // nil-safe; all dispatches guarded
}

// BillingServiceOption configures optional dependencies on BillingService.
type BillingServiceOption func(*BillingService)

// WithSyncHook sets the synchronous hook dispatch function for pre-transition
// lifecycle hooks (e.g., subscription.cancelling). The function is called with
// a domain-local payload struct that is JSON-serializable. Pass nil to disable.
func WithSyncHook(fn SyncHookFn) BillingServiceOption {
	return func(s *BillingService) { s.syncHook = fn }
}

// WithAsyncHook sets the asynchronous hook dispatch function for post-commit
// lifecycle notifications (e.g., subscription.cancelled.post). The function is
// called with a domain-local payload struct. Pass nil to disable.
func WithAsyncHook(fn AsyncHookFn) BillingServiceOption {
	return func(s *BillingService) { s.asyncHook = fn }
}

// NewBillingService creates a BillingService with the given dependencies.
// Optional dependencies (dispatcher, feature flags) are configured via
// BillingServiceOption functions to keep the constructor backward-compatible.
func NewBillingService(
	plans billing.PlanRepository,
	subs billing.SubscriptionRepository,
	invoices billing.InvoiceRepository,
	families billing.FamilyRepository,
	publisher domainevent.Publisher,
	prorate *ProrateCalculator,
	trial *TrialManager,
	txRunner txmanager.Runner,
	clk clock.Clock,
	logger *slog.Logger,
	opts ...BillingServiceOption,
) *BillingService {
	s := &BillingService{
		plans:     plans,
		subs:      subs,
		invoices:  invoices,
		families:  families,
		publisher: publisher,
		prorate:   prorate,
		trial:     trial,
		txRunner:  txRunner,
		clock:     clk,
		logger:    logger,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// dispatchHook dispatches a sync hook via the injected SyncHookFn if set.
// Returns nil response when no hook function is configured (fallback to
// default behavior). The hook function handles marshaling, dispatch, error
// logging, and compensation internally.
func (s *BillingService) dispatchHook(ctx context.Context, hookName string, payload any) (json.RawMessage, error) {
	if s.syncHook == nil {
		return nil, nil
	}
	return s.syncHook(ctx, hookName, payload)
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
// otherwise it starts as active. The subscription, invoice, and outbox event
// are persisted in a single database transaction.
func (s *BillingService) CreateSubscription(
	ctx context.Context,
	cmd CreateSubscriptionCmd,
) (*aggregate.Subscription, *aggregate.Invoice, error) {
	ctx, span := tracing.StartSpan(ctx, "billing.create_subscription")
	defer span.End()

	plan, err := s.plans.GetByID(ctx, cmd.PlanID)
	if err != nil {
		return nil, nil, fmt.Errorf("get plan: %w", err)
	}

	if err := (aggregate.CheckoutEligibility{Plan: plan}).Check(); err != nil {
		return nil, nil, fmt.Errorf("checkout eligibility: %w", err)
	}

	// Create subscription (defaults to trial). The subscription.creating
	// hook is dispatched by CheckoutService during the checkout flow —
	// CreateSubscription does not dispatch it to avoid double-firing.
	now := s.clock.Now()
	sub, err := aggregate.NewSubscription(cmd.UserID, plan.ID, plan.Interval, cmd.AddonIDs, now)
	if err != nil {
		return nil, nil, fmt.Errorf("create subscription: %w", err)
	}

	// Build line items for the invoice.
	lineItems := buildLineItems(plan, cmd.AddonIDs, s.logger)

	inv, err := aggregate.NewInvoice(sub.ID, cmd.UserID, lineItems, nil, plan.BasePrice.Currency, now)
	if err != nil {
		return nil, nil, fmt.Errorf("create invoice: %w", err)
	}

	// Aggregate already recorded its own creation event in NewSubscription.

	err = s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
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
		hookResp, _ := s.dispatchHook(txCtx, HookSubCancelling, CancellingPayload{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			PlanID:         sub.PlanID,
			Reason:         reason,
		})
		if hookResp != nil {
			var resp CancellingResponse
			if err := json.Unmarshal(hookResp, &resp); err == nil && resp.Block {
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
