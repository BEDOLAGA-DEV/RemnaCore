package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// AddonService manages subscription addon operations. It owns
// AddSubscriptionAddon and RemoveSubscriptionAddon, which were extracted from
// BillingService to keep each service focused on a single responsibility.
type AddonService struct {
	subRepo   billing.SubscriptionRepository
	planReader billing.PlanReader
	publisher domainevent.Publisher
	txRunner  txmanager.Runner
	clock     clock.Clock
	logger    *slog.Logger
}

// NewAddonService creates an AddonService with the given dependencies.
func NewAddonService(
	subRepo billing.SubscriptionRepository,
	planReader billing.PlanReader,
	publisher domainevent.Publisher,
	txRunner txmanager.Runner,
	clk clock.Clock,
	logger *slog.Logger,
) *AddonService {
	return &AddonService{
		subRepo:   subRepo,
		planReader: planReader,
		publisher: publisher,
		txRunner:  txRunner,
		clock:     clk,
		logger:    logger,
	}
}

// AddSubscriptionAddon adds an addon to a subscription. The plan is loaded to
// build an AddonSpec that enforces plan-level constraints (available addons,
// max addons limit). The read (with FOR UPDATE lock), mutation, update, and
// outbox event are all performed inside a single database transaction to
// prevent TOCTOU races.
func (s *AddonService) AddSubscriptionAddon(ctx context.Context, subID, addonID string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subRepo.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		plan, err := s.planReader.GetByID(txCtx, sub.PlanID)
		if err != nil {
			return fmt.Errorf("get plan: %w", err)
		}

		spec := aggregate.NewAddonSpec(plan)
		if err := sub.AddAddon(addonID, spec, s.clock.Now()); err != nil {
			return err
		}

		if err := s.subRepo.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, sub)
	})
}

// RemoveSubscriptionAddon removes an addon from a subscription. The read (with
// FOR UPDATE lock), mutation, update, and outbox event are all performed inside
// a single database transaction to prevent TOCTOU races.
func (s *AddonService) RemoveSubscriptionAddon(ctx context.Context, subID, addonID string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		sub, err := s.subRepo.GetByIDForUpdate(txCtx, subID)
		if err != nil {
			return fmt.Errorf("get subscription: %w", err)
		}

		if err := sub.RemoveAddon(addonID, s.clock.Now()); err != nil {
			return err
		}

		if err := s.subRepo.Update(txCtx, sub); err != nil {
			return fmt.Errorf("update subscription: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, sub)
	})
}
