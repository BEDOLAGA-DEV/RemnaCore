package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubagg "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tracing"
)

const (
	// CompensationMaxRetries is the number of attempts for compensating actions
	// (e.g. deleting an orphaned Remnawave user after a DB failure).
	CompensationMaxRetries = 3

	// CompensationBaseDelay is the base delay for exponential backoff between
	// compensation retry attempts.
	CompensationBaseDelay = 1 * time.Second
)

// ProvisionRequest holds the input data for a provisioning saga run.
type ProvisionRequest struct {
	SubscriptionID  string
	PlatformUserID  string
	Plan            multisubdomain.PlanSnapshot
	AddonIDs        []string
	FamilyMemberIDs []string
}

// ProvisionResult holds the output of a single binding provisioning step.
type ProvisionResult struct {
	BindingID          string
	RemnawaveUUID      string
	RemnawaveShortUUID string
	Purpose            multisubagg.BindingPurpose
}

// ProvisioningSaga orchestrates the creation of Remnawave VPN users for a
// subscription. It implements saga compensation: if a step fails, previously
// successful steps are not rolled back (they remain active), but the failed
// binding is marked as failed.
type ProvisioningSaga struct {
	bindings   multisubdomain.BindingRepository
	gateway    multisubdomain.RemnawaveGateway
	publisher  domainevent.Publisher
	calculator *BindingCalculator
	clock      clock.Clock
}

// NewProvisioningSaga creates a ProvisioningSaga with its dependencies.
func NewProvisioningSaga(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	calculator *BindingCalculator,
	clk clock.Clock,
) *ProvisioningSaga {
	return &ProvisioningSaga{
		bindings:   bindings,
		gateway:    gateway,
		publisher:  publisher,
		calculator: calculator,
		clock:      clk,
	}
}

// Provision creates all needed Remnawave users for a subscription. It
// calculates the required bindings from the plan, addons, and family members,
// then provisions each one sequentially with saga compensation.
func (s *ProvisioningSaga) Provision(ctx context.Context, req ProvisionRequest) ([]ProvisionResult, error) {
	ctx, span := tracing.StartSpan(ctx, "multisub.provisioning_saga.provision")
	defer span.End()

	specs := s.calculator.Calculate(req.Plan, req.AddonIDs, req.FamilyMemberIDs)

	// Validate binding limit before provisioning any bindings.
	if req.Plan.MaxRemnawaveBindings > 0 && len(specs) > req.Plan.MaxRemnawaveBindings {
		return nil, multisubdomain.ErrMaxBindingsExceeded
	}

	results := make([]ProvisionResult, 0, len(specs))
	now := s.clock.Now()

	for i, spec := range specs {
		// 1. Create binding in our DB (PENDING)
		binding, err := multisubagg.NewBinding(
			req.SubscriptionID,
			req.PlatformUserID,
			spec.Purpose,
			i,
			spec.TrafficLimit,
			now,
		)
		if err != nil {
			return results, fmt.Errorf("create binding aggregate: %w", err)
		}
		if err := s.bindings.Create(ctx, binding); err != nil {
			return results, fmt.Errorf("persist binding: %w", err)
		}

		// 2. Create user in Remnawave
		rwUser, err := s.gateway.CreateUser(ctx, multisubdomain.CreateRemnawaveUserRequest{
			Username:          binding.RemnawaveUsername,
			TrafficLimitBytes: spec.TrafficLimit,
			TrafficStrategy:   multisubdomain.DefaultTrafficStrategy,
			Tag:               multisubdomain.PlatformTag,
		})
		if err != nil {
			// COMPENSATION: mark binding as failed
			_ = binding.MarkFailed(err.Error(), now)
			_ = s.bindings.Update(ctx, binding)
			return results, fmt.Errorf("remnawave create user: %w", err)
		}

		// 3. Mark binding as provisioned
		if err := binding.MarkProvisioned(rwUser.UUID, rwUser.ShortUUID, now); err != nil {
			return results, fmt.Errorf("mark provisioned: %w", err)
		}
		if err := s.bindings.Update(ctx, binding); err != nil {
			// COMPENSATION: delete Remnawave user since our DB update failed.
			// Uses exponential backoff to avoid leaving ghost users in Remnawave.
			s.compensateDeleteUser(ctx, rwUser.UUID, binding.ID)
			return results, fmt.Errorf("update binding: %w", err)
		}

		// 4. Publish binding's self-recorded events
		for _, evt := range binding.DomainEvents() {
			if err := s.publisher.Publish(ctx, evt); err != nil {
				slog.Warn("failed to publish binding event",
					slog.String("binding_id", binding.ID),
					slog.String("event_type", string(evt.Type)),
					slog.Any("error", err),
				)
			}
		}

		results = append(results, ProvisionResult{
			BindingID:          binding.ID,
			RemnawaveUUID:      rwUser.UUID,
			RemnawaveShortUUID: rwUser.ShortUUID,
			Purpose:            spec.Purpose,
		})
	}

	return results, nil
}

// compensateDeleteUser attempts to delete an orphaned Remnawave user with
// exponential backoff. If all retries are exhausted, it logs an error that
// signals manual cleanup is required. The BindingReconciler will eventually
// pick up any remaining orphans.
func (s *ProvisioningSaga) compensateDeleteUser(ctx context.Context, remnawaveUUID, bindingID string) {
	for attempt := range CompensationMaxRetries {
		err := s.gateway.DeleteUser(ctx, remnawaveUUID)
		if err == nil {
			slog.Info("compensation: deleted remnawave user",
				slog.String("remnawave_uuid", remnawaveUUID),
				slog.Int("attempt", attempt+1),
			)
			return
		}

		slog.Warn("compensation: failed to delete remnawave user, retrying",
			slog.String("remnawave_uuid", remnawaveUUID),
			slog.String("binding_id", bindingID),
			slog.Any("error", err),
			slog.Int("attempt", attempt+1),
		)

		if attempt < CompensationMaxRetries-1 {
			delay := CompensationBaseDelay * time.Duration(1<<attempt) // 1s, 2s, 4s
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}

	// All retries exhausted -- log error for manual cleanup / reconciler.
	slog.Error("compensation: FAILED to delete remnawave user after all retries -- manual cleanup required",
		slog.String("remnawave_uuid", remnawaveUUID),
		slog.String("binding_id", bindingID),
	)
}
