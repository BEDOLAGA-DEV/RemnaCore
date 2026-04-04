package service

import (
	"context"
	"fmt"
	"log/slog"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// DeprovisioningSaga orchestrates the removal of Remnawave VPN users for a
// subscription. It is best-effort: if deleting a single Remnawave user fails,
// the saga logs the failure, marks the binding as failed, and continues with
// the remaining bindings.
type DeprovisioningSaga struct {
	bindings  multisubdomain.BindingRepository
	gateway   multisubdomain.RemnawaveGateway
	publisher domainevent.Publisher
	clock     clock.Clock
}

// NewDeprovisioningSaga creates a DeprovisioningSaga with its dependencies.
func NewDeprovisioningSaga(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	clk clock.Clock,
) *DeprovisioningSaga {
	return &DeprovisioningSaga{
		bindings:  bindings,
		gateway:   gateway,
		publisher: publisher,
		clock:     clk,
	}
}

// Deprovision removes all Remnawave users for a subscription. It fetches every
// active binding for the given subscription and, for each one, attempts to
// delete the Remnawave user, mark the binding as deprovisioned, and publish an
// event. Individual Remnawave failures are recorded on the binding but do not
// abort the whole operation.
func (s *DeprovisioningSaga) Deprovision(ctx context.Context, subscriptionID string) error {
	bindings, err := s.bindings.GetActiveBySubscriptionID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("get active bindings: %w", err)
	}

	for _, binding := range bindings {
		s.deprovisionOne(ctx, binding)
	}

	return nil
}

// deprovisionOne handles a single binding. It never returns an error — failures
// are recorded on the binding itself so that the caller can continue with the
// next binding.
func (s *DeprovisioningSaga) deprovisionOne(ctx context.Context, binding *aggregate.RemnawaveBinding) {
	now := s.clock.Now()
	// 1. Delete user in Remnawave
	if binding.RemnawaveUUID != "" {
		if err := s.gateway.DeleteUser(ctx, binding.RemnawaveUUID); err != nil {
			// Mark binding as failed and persist — do not stop the saga.
			binding.MarkFailed(fmt.Sprintf("remnawave delete: %s", err.Error()), now)
			if updateErr := s.bindings.Update(ctx, binding); updateErr != nil {
				slog.Warn("failed to update binding after remnawave delete failure",
					slog.String("binding_id", binding.ID),
					slog.Any("error", updateErr),
				)
			}
			return
		}
	}

	// 2. Mark binding as deprovisioned
	binding.Deprovision(now)
	if err := s.bindings.Update(ctx, binding); err != nil {
		slog.Warn("failed to update binding after deprovision",
			slog.String("binding_id", binding.ID),
			slog.Any("error", err),
		)
	}

	// 3. Publish deprovisioned event
	event := multisubdomain.NewBindingDeprovisionedEvent(
		binding.ID,
		binding.SubscriptionID,
		binding.RemnawaveUUID,
	)
	if err := s.publisher.Publish(ctx, event); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(event.Type)),
			slog.Any("error", err),
		)
	}
}
