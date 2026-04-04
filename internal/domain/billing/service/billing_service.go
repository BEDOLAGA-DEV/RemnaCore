package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// lineItemQuantityOne is the standard quantity for plan and addon line items.
const lineItemQuantityOne = 1

// CreateSubscriptionCmd holds the parameters for creating a new subscription.
type CreateSubscriptionCmd struct {
	UserID   string
	PlanID   string
	AddonIDs []string
}

// BillingService implements CQRS command handlers for the billing domain.
type BillingService struct {
	plans    billing.PlanRepository
	subs     billing.SubscriptionRepository
	invoices billing.InvoiceRepository
	families billing.FamilyRepository
	publisher domainevent.Publisher
	prorate  *ProrateCalculator
	trial    *TrialManager
	txRunner txmanager.Runner
	clock    clock.Clock
}

// NewBillingService creates a BillingService with the given dependencies.
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
) *BillingService {
	return &BillingService{
		plans:    plans,
		subs:     subs,
		invoices: invoices,
		families: families,
		publisher: publisher,
		prorate:  prorate,
		trial:    trial,
		txRunner: txRunner,
		clock:    clk,
	}
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

	// Create subscription (defaults to trial)
	now := s.clock.Now()
	sub, err := aggregate.NewSubscription(cmd.UserID, plan.ID, plan.Interval, cmd.AddonIDs, now)
	if err != nil {
		return nil, nil, fmt.Errorf("create subscription: %w", err)
	}

	// Build line items for the invoice
	lineItems := buildLineItems(plan, cmd.AddonIDs)

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

		if err := s.publishAggregateEvents(txCtx, sub); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return sub, inv, nil
}

// CancelSubscription cancels an existing subscription. The status update and
// outbox event are persisted in a single database transaction.
func (s *BillingService) CancelSubscription(ctx context.Context, subID string) error {
	sub, err := s.subs.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	if err := sub.Cancel(s.clock.Now()); err != nil {
		return fmt.Errorf("cancel subscription: %w", err)
	}

	// Aggregate already recorded its own cancelled event in Cancel().

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.subs.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}

		if err := s.publishAggregateEvents(txCtx, sub); err != nil {
			return err
		}

		return nil
	})
}

// PayInvoice marks an invoice as paid and activates the associated subscription
// if it is in trial or past_due status. All writes and outbox events are
// persisted in a single database transaction.
func (s *BillingService) PayInvoice(ctx context.Context, invoiceID string) error {
	inv, err := s.invoices.GetByID(ctx, invoiceID)
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

	// Aggregate already recorded its own paid event in MarkPaid().

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.invoices.Update(txCtx, inv); err != nil {
			return fmt.Errorf("update invoice: %w", err)
		}

		if err := s.publishAggregateEvents(txCtx, inv); err != nil {
			return err
		}

		// Activate subscription if it is in trial or past_due
		sub, err := s.subs.GetByID(txCtx, inv.SubscriptionID)
		if err != nil {
			return fmt.Errorf("get subscription for activation: %w", err)
		}

		if sub.Status == aggregate.StatusTrial || sub.Status == aggregate.StatusPastDue {
			if err := sub.Activate(now); err != nil {
				return fmt.Errorf("activate subscription: %w", err)
			}

			// Aggregate already recorded its own activated event in Activate().

			if err := s.subs.Update(txCtx, sub); err != nil {
				return fmt.Errorf("update subscription: %w", err)
			}

			if err := s.publishAggregateEvents(txCtx, sub); err != nil {
				return err
			}
		}

		return nil
	})
}

// AddFamilyMember adds a member to the subscription owner's family group.
// The subscription's plan must have family sharing enabled. The family group
// update and outbox event are persisted in a single database transaction.
func (s *BillingService) AddFamilyMember(
	ctx context.Context,
	subID, memberUserID, nickname string,
) error {
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
		// Get or create family group
		fg, err := s.families.GetByOwnerID(txCtx, sub.UserID)
		if err != nil {
			if !errors.Is(err, billing.ErrFamilyGroupNotFound) {
				return fmt.Errorf("get family group: %w", err)
			}
			// Create a new family group if not found
			fg = aggregate.NewFamilyGroup(sub.UserID, plan.MaxFamilyMembers, now)
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

		// Aggregate already recorded its own member_added event in AddMember().

		if err := s.families.Update(txCtx, fg); err != nil {
			return fmt.Errorf("update family group: %w", err)
		}

		if err := s.publishAggregateEvents(txCtx, fg); err != nil {
			return err
		}

		return nil
	})
}

// RemoveFamilyMember removes a member from the subscription owner's family group.
// The family group update and outbox event are persisted in a single database
// transaction.
func (s *BillingService) RemoveFamilyMember(
	ctx context.Context,
	subID, memberUserID string,
) error {
	sub, err := s.subs.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	fg, err := s.families.GetByOwnerID(ctx, sub.UserID)
	if err != nil {
		return fmt.Errorf("get family group: %w", err)
	}

	if err := fg.RemoveMember(memberUserID, s.clock.Now()); err != nil {
		return fmt.Errorf("remove family member: %w", err)
	}

	// Aggregate already recorded its own member_removed event in RemoveMember().

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.families.Update(txCtx, fg); err != nil {
			return fmt.Errorf("update family group: %w", err)
		}

		if err := s.publishAggregateEvents(txCtx, fg); err != nil {
			return err
		}

		return nil
	})
}

// eventSource is implemented by aggregates that embed domainevent.EventRecorder.
type eventSource interface {
	DomainEvents() []domainevent.Event
}

// publishAggregateEvents flushes all pending events from the aggregate and
// publishes them through the publisher. This centralises the flush-and-publish
// pattern so individual service methods cannot forget to publish.
func (s *BillingService) publishAggregateEvents(ctx context.Context, src eventSource) error {
	for _, event := range src.DomainEvents() {
		if err := s.publisher.Publish(ctx, event); err != nil {
			return fmt.Errorf("publish %s: %w", event.Type, err)
		}
	}
	return nil
}

// buildLineItems creates invoice line items from a plan and selected addon IDs.
func buildLineItems(plan *aggregate.Plan, addonIDs []string) []vo.LineItem {
	items := []vo.LineItem{
		vo.NewLineItem(plan.Name, vo.LineItemPlan, plan.BasePrice, lineItemQuantityOne),
	}

	addonMap := make(map[string]aggregate.Addon, len(plan.AvailableAddons))
	for _, addon := range plan.AvailableAddons {
		addonMap[addon.ID] = addon
	}

	for _, addonID := range addonIDs {
		if addon, ok := addonMap[addonID]; ok {
			items = append(items, vo.NewLineItem(
				addon.Name,
				vo.LineItemAddon,
				addon.Price,
				lineItemQuantityOne,
			))
		}
	}

	return items
}
