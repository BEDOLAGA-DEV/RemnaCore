package billing

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
)

// PlanRepository defines persistence operations for the Plan aggregate.
type PlanRepository interface {
	GetByID(ctx context.Context, id string) (*aggregate.Plan, error)
	GetAll(ctx context.Context) ([]*aggregate.Plan, error)
	GetActive(ctx context.Context) ([]*aggregate.Plan, error)
	Create(ctx context.Context, plan *aggregate.Plan) error
	Update(ctx context.Context, plan *aggregate.Plan) error
}

// StatusTransition holds the previous and current status of a subscription
// after an atomic status update. Returned by UpdateStatus for audit trail
// and domain event payloads.
type StatusTransition struct {
	PreviousStatus aggregate.SubscriptionStatus
	CurrentStatus  aggregate.SubscriptionStatus
}

// SubscriptionRepository defines persistence operations for the Subscription aggregate.
type SubscriptionRepository interface {
	GetByID(ctx context.Context, id string) (*aggregate.Subscription, error)
	GetByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error)
	GetActiveByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error)
	GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Subscription, error)
	Create(ctx context.Context, sub *aggregate.Subscription) error
	Update(ctx context.Context, sub *aggregate.Subscription) error
	// UpdateStatus atomically transitions a subscription's status and returns
	// both the old and new values. Uses PG18 native OLD/NEW RETURNING.
	UpdateStatus(ctx context.Context, id string, newStatus aggregate.SubscriptionStatus) (*StatusTransition, error)
}

// InvoiceRepository defines persistence operations for the Invoice aggregate.
type InvoiceRepository interface {
	GetByID(ctx context.Context, id string) (*aggregate.Invoice, error)
	GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.Invoice, error)
	GetPendingByUserID(ctx context.Context, userID string) ([]*aggregate.Invoice, error)
	GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Invoice, error)
	Create(ctx context.Context, inv *aggregate.Invoice) error
	Update(ctx context.Context, inv *aggregate.Invoice) error
}

// FamilyRepository defines persistence operations for the FamilyGroup aggregate.
type FamilyRepository interface {
	GetByID(ctx context.Context, id string) (*aggregate.FamilyGroup, error)
	GetByOwnerID(ctx context.Context, ownerID string) (*aggregate.FamilyGroup, error)
	Create(ctx context.Context, fg *aggregate.FamilyGroup) error
	Update(ctx context.Context, fg *aggregate.FamilyGroup) error
	Delete(ctx context.Context, id string) error
}
