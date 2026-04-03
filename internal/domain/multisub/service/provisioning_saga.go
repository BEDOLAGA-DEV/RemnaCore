package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubagg "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// ProvisionRequest holds the input data for a provisioning saga run.
type ProvisionRequest struct {
	SubscriptionID string
	PlatformUserID string
	Plan           *aggregate.Plan
	AddonIDs       []string
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
}

// NewProvisioningSaga creates a ProvisioningSaga with its dependencies.
func NewProvisioningSaga(
	bindings multisubdomain.BindingRepository,
	gateway multisubdomain.RemnawaveGateway,
	publisher domainevent.Publisher,
	calculator *BindingCalculator,
) *ProvisioningSaga {
	return &ProvisioningSaga{
		bindings:   bindings,
		gateway:    gateway,
		publisher:  publisher,
		calculator: calculator,
	}
}

// Provision creates all needed Remnawave users for a subscription. It
// calculates the required bindings from the plan, addons, and family members,
// then provisions each one sequentially with saga compensation.
func (s *ProvisioningSaga) Provision(ctx context.Context, req ProvisionRequest) ([]ProvisionResult, error) {
	specs := s.calculator.Calculate(req.Plan, req.AddonIDs, req.FamilyMemberIDs)

	results := make([]ProvisionResult, 0, len(specs))

	for i, spec := range specs {
		// 1. Create binding in our DB (PENDING)
		binding := multisubagg.NewBinding(
			req.SubscriptionID,
			req.PlatformUserID,
			string(spec.Purpose),
			i,
			spec.TrafficLimit,
		)
		if err := s.bindings.Create(ctx, binding); err != nil {
			return results, fmt.Errorf("create binding: %w", err)
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
			binding.MarkFailed(err.Error())
			_ = s.bindings.Update(ctx, binding)
			return results, fmt.Errorf("remnawave create user: %w", err)
		}

		// 3. Mark binding as provisioned
		binding.MarkProvisioned(rwUser.UUID, rwUser.ShortUUID)
		if err := s.bindings.Update(ctx, binding); err != nil {
			// COMPENSATION: delete Remnawave user since our DB update failed
			_ = s.gateway.DeleteUser(ctx, rwUser.UUID)
			return results, fmt.Errorf("update binding: %w", err)
		}

		// 4. Publish event
		if err := s.publisher.Publish(ctx, multisubdomain.NewBindingProvisionedEvent(
			binding.ID,
			binding.SubscriptionID,
			rwUser.UUID,
			string(spec.Purpose),
		)); err != nil {
			// Log but don't fail — binding is provisioned, event publish is secondary
			slog.Warn("failed to publish binding.provisioned event",
				slog.String("binding_id", binding.ID),
				slog.String("error", err.Error()),
			)
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
