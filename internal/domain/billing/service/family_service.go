package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// FamilyService manages family group membership for subscriptions. It owns
// AddFamilyMember and RemoveFamilyMember, which were extracted from
// BillingService to keep each service focused on a single responsibility.
type FamilyService struct {
	familyRepo billing.FamilyRepository
	subReader  billing.SubscriptionReader
	planReader billing.PlanReader
	publisher  domainevent.Publisher
	txRunner   txmanager.Runner
	clock      clock.Clock
	logger     *slog.Logger
}

// NewFamilyService creates a FamilyService with the given dependencies.
func NewFamilyService(
	familyRepo billing.FamilyRepository,
	subReader billing.SubscriptionReader,
	planReader billing.PlanReader,
	publisher domainevent.Publisher,
	txRunner txmanager.Runner,
	clk clock.Clock,
	logger *slog.Logger,
) *FamilyService {
	return &FamilyService{
		familyRepo: familyRepo,
		subReader:  subReader,
		planReader: planReader,
		publisher:  publisher,
		txRunner:   txRunner,
		clock:      clk,
		logger:     logger,
	}
}

// AddFamilyMember adds a member to the subscription owner's family group.
// The subscription's plan must have family sharing enabled. The family group
// read (with FOR UPDATE lock), mutation, update, and outbox event are all
// performed inside a single database transaction to prevent TOCTOU races.
func (s *FamilyService) AddFamilyMember(
	ctx context.Context,
	subID, memberUserID, nickname string,
) error {
	// Subscription and plan are read-only here (no mutation), so they can
	// safely be read outside the transaction.
	sub, err := s.subReader.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	plan, err := s.planReader.GetByID(ctx, sub.PlanID)
	if err != nil {
		return fmt.Errorf("get plan: %w", err)
	}

	now := s.clock.Now()
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		// Get or create family group (with FOR UPDATE lock when it exists)
		fg, err := s.familyRepo.GetByOwnerIDForUpdate(txCtx, sub.UserID)
		if err != nil {
			if !errors.Is(err, billing.ErrFamilyGroupNotFound) {
				return fmt.Errorf("get family group: %w", err)
			}
			// Create a new family group if not found
			fg, err = aggregate.NewFamilyGroup(sub.UserID, plan.MaxFamilyMembers, now)
			if err != nil {
				return fmt.Errorf("create family group: %w", err)
			}
			if err := s.familyRepo.Create(txCtx, fg); err != nil {
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

		if err := s.familyRepo.Update(txCtx, fg); err != nil {
			return fmt.Errorf("update family group: %w", err)
		}

		return domainevent.PublishAll(txCtx, s.publisher, fg)
	})
}

// RemoveFamilyMember removes a member from the subscription owner's family
// group. The family group read (with FOR UPDATE lock), mutation, update, and
// outbox event are all performed inside a single database transaction to
// prevent TOCTOU races.
func (s *FamilyService) RemoveFamilyMember(
	ctx context.Context,
	subID, memberUserID string,
) error {
	// Subscription is read-only here (used only to find the owner).
	sub, err := s.subReader.GetByID(ctx, subID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		fg, err := s.familyRepo.GetByOwnerIDForUpdate(txCtx, sub.UserID)
		if err != nil {
			return fmt.Errorf("get family group: %w", err)
		}

		if err := fg.RemoveMember(memberUserID, s.clock.Now()); err != nil {
			return fmt.Errorf("remove family member: %w", err)
		}

		if err := s.familyRepo.Update(txCtx, fg); err != nil {
			return fmt.Errorf("update family group: %w", err)
		}

		return domainevent.PublishAll(txCtx, s.publisher, fg)
	})
}
