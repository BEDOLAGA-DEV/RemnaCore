package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
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
	clock      clock.Clock
	logger     *slog.Logger
	dispatcher hookdispatch.Dispatcher // nil-safe; all dispatches guarded
	hooksEnabled bool                  // subscription lifecycle hooks feature flag
}

// BillingServiceOption configures optional dependencies on BillingService.
type BillingServiceOption func(*BillingService)

// WithDispatcher sets the WASM hook dispatcher for subscription lifecycle hooks.
func WithDispatcher(d hookdispatch.Dispatcher) BillingServiceOption {
	return func(s *BillingService) { s.dispatcher = d }
}

// WithHooksEnabled enables subscription lifecycle hook dispatch points.
func WithHooksEnabled(enabled bool) BillingServiceOption {
	return func(s *BillingService) { s.hooksEnabled = enabled }
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

// dispatchHook dispatches a sync hook if dispatcher is available and hooks are
// enabled. payload must be a struct with json tags (marshaled via json.Marshal
// before dispatch). Returns nil response on error or nil dispatcher (fallback
// to default behavior). Uses DispatchSyncSafe so that plugin failures never
// block the business operation.
func (s *BillingService) dispatchHook(ctx context.Context, hookName string, payload any) (json.RawMessage, error) {
	if s.dispatcher == nil || !s.hooksEnabled {
		return nil, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal hook payload: %w", err)
	}
	result := s.dispatcher.DispatchSyncSafe(ctx, hookName, data)
	if result.Err != nil {
		s.logger.Warn("hook dispatch failed, proceeding with defaults",
			slog.String("hook", hookName),
			slog.Any("error", result.Err),
		)
		return nil, nil
	}
	return result.Payload, nil
}

// dispatchAsync fires an async hook if dispatcher is available and hooks are
// enabled. Errors are logged but never propagated (fire-and-forget).
func (s *BillingService) dispatchAsync(ctx context.Context, hookName string, payload any) {
	if s.dispatcher == nil || !s.hooksEnabled {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Warn("failed to marshal async hook payload",
			slog.String("hook", hookName),
			slog.Any("error", err),
		)
		return
	}
	s.dispatcher.DispatchAsync(ctx, hookName, data)
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
	var asyncPayload *sdk.SubCancellingRequest

	err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		// Dispatch subscription.cancelling pre-hook.
		hookResp, _ := s.dispatchHook(txCtx, HookSubCancelling, sdk.SubCancellingRequest{
			SubscriptionID: sub.ID,
			UserID:         sub.UserID,
			PlanID:         sub.PlanID,
			Reason:         reason,
		})
		if hookResp != nil {
			var resp sdk.SubCancellingResponse
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
		asyncPayload = &sdk.SubCancellingRequest{
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
	var asyncPayload *sdk.SubActivatedNotification

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
			asyncPayload = &sdk.SubActivatedNotification{
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

// AddFamilyMember adds a member to the subscription owner's family group.
// The subscription's plan must have family sharing enabled. The family group
// read (with FOR UPDATE lock), mutation, update, and outbox event are all
// performed inside a single database transaction to prevent TOCTOU races.
func (s *BillingService) AddFamilyMember(
	ctx context.Context,
	subID, memberUserID, nickname string,
) error {
	// Subscription and plan are read-only here (no mutation), so they can
	// safely be read outside the transaction.
	sub, err := s.subs.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	plan, err := s.plans.GetByID(ctx, sub.PlanID)
	if err != nil {
		return fmt.Errorf("get plan: %w", err)
	}

	now := s.clock.Now()
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		// Get or create family group (with FOR UPDATE lock when it exists)
		fg, err := s.families.GetByOwnerIDForUpdate(txCtx, sub.UserID)
		if err != nil {
			if !errors.Is(err, billing.ErrFamilyGroupNotFound) {
				return fmt.Errorf("get family group: %w", err)
			}
			// Create a new family group if not found
			fg, err = aggregate.NewFamilyGroup(sub.UserID, plan.MaxFamilyMembers, now)
			if err != nil {
				return fmt.Errorf("create family group: %w", err)
			}
			if err := s.families.Create(txCtx, fg); err != nil {
				return fmt.Errorf("create family group: %w", err)
			}
		}

		// Validate family eligibility before adding the member.
		eligibility := aggregate.FamilyEligibility{
			Plan:        plan,
			MemberCount: fg.MemberCount(),
		}
		if err := eligibility.Check(); err != nil {
			return fmt.Errorf("family eligibility: %w", err)
		}

		if err := fg.AddMember(memberUserID, nickname, now); err != nil {
			return fmt.Errorf("add family member: %w", err)
		}

		if err := s.families.Update(txCtx, fg); err != nil {
			return fmt.Errorf("update family group: %w", err)
		}

		return domainevent.PublishAll(txCtx, s.publisher, fg)
	})
}

// RemoveFamilyMember removes a member from the subscription owner's family
// group. The family group read (with FOR UPDATE lock), mutation, update, and
// outbox event are all performed inside a single database transaction to
// prevent TOCTOU races.
func (s *BillingService) RemoveFamilyMember(
	ctx context.Context,
	subID, memberUserID string,
) error {
	// Subscription is read-only here (used only to find the owner).
	sub, err := s.subs.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		fg, err := s.families.GetByOwnerIDForUpdate(txCtx, sub.UserID)
		if err != nil {
			return fmt.Errorf("get family group: %w", err)
		}

		if err := fg.RemoveMember(memberUserID, s.clock.Now()); err != nil {
			return fmt.Errorf("remove family member: %w", err)
		}

		if err := s.families.Update(txCtx, fg); err != nil {
			return fmt.Errorf("update family group: %w", err)
		}

		return domainevent.PublishAll(txCtx, s.publisher, fg)
	})
}

// AddSubscriptionAddon adds an addon to a subscription. The plan is loaded to
// build an AddonSpec that enforces plan-level constraints (available addons,
// max addons limit). The read (with FOR UPDATE lock), mutation, update, and
// outbox event are all performed inside a single database transaction to
// prevent TOCTOU races.
func (s *BillingService) AddSubscriptionAddon(ctx context.Context, subID, addonID string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		plan, err := s.plans.GetByID(txCtx, sub.PlanID)
		if err != nil {
			return fmt.Errorf("get plan: %w", err)
		}

		spec := aggregate.NewAddonSpec(plan)
		if err := sub.AddAddon(addonID, spec, s.clock.Now()); err != nil {
			return err
		}

		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, sub)
	})
}

// RemoveSubscriptionAddon removes an addon from a subscription. The read (with
// FOR UPDATE lock), mutation, update, and outbox event are all performed inside
// a single database transaction to prevent TOCTOU races.
func (s *BillingService) RemoveSubscriptionAddon(ctx context.Context, subID, addonID string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subs.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		if err := sub.RemoveAddon(addonID, s.clock.Now()); err != nil {
			return err
		}

		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, sub)
	})
}

