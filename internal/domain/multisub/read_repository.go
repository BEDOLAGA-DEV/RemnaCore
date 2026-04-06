package multisub

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
)

// Read-only repository interfaces for CQRS preparation.
//
// These interfaces represent the query side of the multisub persistence.
// They are subsets of the full BindingRepository and exist to support a
// future CQRS split where reads go to a read replica and writes to the primary.
//
// The MultiSubHandler currently depends on BindingRepository for read-only
// endpoints (GetMyBindings, GetBindingsBySubscription). When migrating to CQRS:
//  1. Query handlers should depend on BindingReader instead.
//  2. Saga services and the orchestrator keep the full BindingRepository.
//  3. Fx wiring binds BindingReader to a read-replica-backed implementation.
//
// Design note: not embedded into BindingRepository — Go structural typing
// ensures any concrete type implementing BindingRepository already satisfies
// BindingReader.

// BindingReader provides read-only access to Remnawave bindings.
// Separated from BindingRepository to support future CQRS split where reads
// go to a read replica and writes to the primary.
type BindingReader interface {
	// GetByID retrieves a single binding by its domain ID.
	GetByID(ctx context.Context, id string) (*aggregate.RemnawaveBinding, error)

	// GetBySubscriptionID retrieves all bindings for a given subscription.
	GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error)

	// GetByPlatformUserID retrieves all bindings for a given platform user.
	GetByPlatformUserID(ctx context.Context, userID string) ([]*aggregate.RemnawaveBinding, error)

	// GetActiveBySubscriptionID retrieves only active bindings for a subscription.
	GetActiveBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error)
}
