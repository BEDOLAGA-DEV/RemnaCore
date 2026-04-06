package service

import (
	"context"
	"fmt"
	"log/slog"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubagg "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// BindingLifecycleService coordinates binding state changes in response to
// subscription lifecycle events. This is a domain service that encodes
// business rules about how subscription states map to binding states.
//
// Each method is best-effort per-binding: if a gateway call fails for one
// binding, that binding is marked as failed and the loop continues to process
// the remaining bindings. Gateway calls are intentionally kept outside of
// database transactions — each binding is updated independently after its
// gateway call succeeds.
type BindingLifecycleService struct {
	bindings  multisubdomain.BindingRepository
	gateway   multisubdomain.RemnawaveGateway
	publisher domainevent.Publisher
	clock     clock.Clock
	logger    *slog.Logger
}

// NewBindingLifecycleService creates a BindingLifecycleService with its
// repository and gateway dependencies.
func NewBindingLifecycleService(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	clk clock.Clock,
	logger *slog.Logger,
) *BindingLifecycleService {
	return &BindingLifecycleService{
		bindings:  bindings,
		gateway:   gateway,
		publisher: publisher,
		clock:     clk,
		logger:    logger,
	}
}

// DisableAllForSubscription disables all active bindings for a subscription.
// Called when a subscription is paused or enters past_due.
//
// For each active binding:
//  1. Call gateway.DisableUser (outside any DB transaction)
//  2. On success: transition binding to disabled, persist, publish events
//  3. On failure: mark binding as failed with the reason, persist
//
// Idempotency: returns nil if no active bindings exist.
func (s *BindingLifecycleService) DisableAllForSubscription(ctx context.Context, subscriptionID string) error {
	bindings, err := s.bindings.GetActiveBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get active bindings: %w", err)
	}

	if len(bindings) == 0 {
		return nil
	}

	now := s.clock.Now()

	for _, binding := range bindings {
		if binding.RemnawaveUUID == "" {
			continue
		}

		if err := s.gateway.DisableUser(ctx, binding.RemnawaveUUID); err != nil {
			s.handleGatewayFailure(ctx, binding, "remnawave disable", err)
			continue
		}

		if disableErr := binding.Disable(now); disableErr != nil {
			s.logger.Warn("failed to transition binding to disabled",
				slog.String("binding_id", binding.ID),
				slog.Any("error", disableErr),
			)
			continue
		}

		s.persistAndPublish(ctx, binding)
	}

	return nil
}

// EnableAllForSubscription re-enables all disabled bindings for a subscription.
// Called when a subscription is resumed from paused state.
//
// For each disabled binding:
//  1. Call gateway.EnableUser (outside any DB transaction)
//  2. On success: transition binding to active, persist, publish events
//  3. On failure: mark binding as failed with the reason, persist
//
// Bindings in non-disabled states (active, deprovisioned, failed) are skipped.
// Idempotency: returns nil if no disabled bindings exist.
func (s *BindingLifecycleService) EnableAllForSubscription(ctx context.Context, subscriptionID string) error {
	bindings, err := s.bindings.GetBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get bindings: %w", err)
	}

	hasDisabled := false
	for _, binding := range bindings {
		if binding.Status == multisubagg.BindingDisabled {
			hasDisabled = true
			break
		}
	}
	if !hasDisabled {
		return nil
	}

	now := s.clock.Now()

	for _, binding := range bindings {
		if binding.Status != multisubagg.BindingDisabled {
			continue
		}
		if binding.RemnawaveUUID == "" {
			continue
		}

		if err := s.gateway.EnableUser(ctx, binding.RemnawaveUUID); err != nil {
			s.handleGatewayFailure(ctx, binding, "remnawave enable", err)
			continue
		}

		if enableErr := binding.Enable(now); enableErr != nil {
			s.logger.Warn("failed to transition binding to active",
				slog.String("binding_id", binding.ID),
				slog.Any("error", enableErr),
			)
			continue
		}

		s.persistAndPublish(ctx, binding)
	}

	return nil
}

// LimitBinding transitions a single binding to limited state.
// No gateway call — the VPN provider already restricted the user.
func (s *BindingLifecycleService) LimitBinding(ctx context.Context, bindingID string, reason string) error {
	binding, err := s.bindings.GetByID(ctx, bindingID)
	if err != nil {
		return fmt.Errorf("get binding: %w", err)
	}

	if err := binding.Limit(reason, s.clock.Now()); err != nil {
		return err
	}

	s.persistAndPublish(ctx, binding)
	return nil
}

// UnlimitBinding transitions a single binding from limited back to active.
// No gateway call — the VPN provider already reset the traffic.
func (s *BindingLifecycleService) UnlimitBinding(ctx context.Context, bindingID string) error {
	binding, err := s.bindings.GetByID(ctx, bindingID)
	if err != nil {
		return fmt.Errorf("get binding: %w", err)
	}

	if err := binding.Unlimit(s.clock.Now()); err != nil {
		return err
	}

	s.persistAndPublish(ctx, binding)
	return nil
}

// LimitAllForSubscription transitions all active bindings for a subscription
// to limited. Used when the subscription-level traffic limit is exceeded.
// No gateway calls — the VPN provider already restricted the users.
//
// Best-effort per-binding: if a single binding fails to transition, the error
// is logged and the loop continues with the remaining bindings.
// Idempotency: returns nil if no active bindings exist.
func (s *BindingLifecycleService) LimitAllForSubscription(ctx context.Context, subscriptionID string, reason string) error {
	bindings, err := s.bindings.GetActiveBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get active bindings: %w", err)
	}

	if len(bindings) == 0 {
		return nil
	}

	now := s.clock.Now()

	for _, binding := range bindings {
		if limitErr := binding.Limit(reason, now); limitErr != nil {
			s.logger.Warn("failed to transition binding to limited",
				slog.String("binding_id", binding.ID),
				slog.Any("error", limitErr),
			)
			continue
		}

		s.persistAndPublish(ctx, binding)
	}

	return nil
}

// UnlimitAllForSubscription transitions all limited bindings for a subscription
// back to active. Used when traffic resets for the subscription.
// No gateway calls — the VPN provider already reset the traffic.
//
// Best-effort per-binding: if a single binding fails to transition, the error
// is logged and the loop continues with the remaining bindings.
// Idempotency: returns nil if no limited bindings exist.
func (s *BindingLifecycleService) UnlimitAllForSubscription(ctx context.Context, subscriptionID string) error {
	bindings, err := s.bindings.GetBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get bindings: %w", err)
	}

	hasLimited := false
	for _, binding := range bindings {
		if binding.Status == multisubagg.BindingLimited {
			hasLimited = true
			break
		}
	}
	if !hasLimited {
		return nil
	}

	now := s.clock.Now()

	for _, binding := range bindings {
		if binding.Status != multisubagg.BindingLimited {
			continue
		}

		if unlimitErr := binding.Unlimit(now); unlimitErr != nil {
			s.logger.Warn("failed to transition binding to active",
				slog.String("binding_id", binding.ID),
				slog.Any("error", unlimitErr),
			)
			continue
		}

		s.persistAndPublish(ctx, binding)
	}

	return nil
}

// handleGatewayFailure marks a binding as failed after a gateway call error
// and persists the status change.
func (s *BindingLifecycleService) handleGatewayFailure(
	ctx context.Context,
	binding *multisubagg.RemnawaveBinding,
	operation string,
	gatewayErr error,
) {
	reason := fmt.Sprintf("%s: %s", operation, gatewayErr.Error())
	if failErr := binding.MarkFailed(reason, s.clock.Now()); failErr != nil {
		s.logger.Warn("failed to transition binding to failed",
			slog.String("binding_id", binding.ID),
			slog.Any("error", failErr),
		)
		return
	}
	if updateErr := s.bindings.Update(ctx, binding); updateErr != nil {
		s.logger.Warn("failed to update binding status",
			slog.String("binding_id", binding.ID),
			slog.Any("error", updateErr),
		)
	}
}

// persistAndPublish updates the binding in the repository and publishes all
// accumulated domain events.
func (s *BindingLifecycleService) persistAndPublish(ctx context.Context, binding *multisubagg.RemnawaveBinding) {
	if err := s.bindings.Update(ctx, binding); err != nil {
		s.logger.Warn("failed to update binding status",
			slog.String("binding_id", binding.ID),
			slog.Any("error", err),
		)
		return
	}

	if err := domainevent.PublishAll(ctx, s.publisher, binding); err != nil {
		s.logger.Warn("failed to publish binding event",
			slog.String("binding_id", binding.ID),
			slog.Any("error", err),
		)
	}
}
