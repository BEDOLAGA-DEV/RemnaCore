package billing

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
)

// Read-only repository interfaces for CQRS preparation.
//
// These interfaces represent the query side of each aggregate's persistence.
// They are subsets of the full repository interfaces and exist to support a
// future CQRS split where reads go to a read replica and writes to the primary.
//
// Current state:
//   - Handlers and services still depend on the full repository interfaces.
//   - The existing *Repository implementations already satisfy these reader
//     interfaces implicitly (no embedding required).
//
// Migration path:
//  1. (This commit) Define read-only interfaces alongside existing repos.
//  2. Migrate handler constructors to accept *Reader interfaces for query
//     endpoints, keeping *Repository for command endpoints.
//  3. In Fx wiring, bind *Reader to a read-replica-backed implementation
//     while *Repository stays on the primary.
//
// Design note: these interfaces are intentionally NOT embedded into the full
// repository interfaces. Embedding would force every mock and implementation
// to satisfy both, adding complexity with no benefit at this stage. Go's
// structural typing means that any concrete type implementing the full
// repository already satisfies the reader interface — no code changes needed.

// PlanReader provides read-only access to plans.
// Separated from PlanRepository to support future CQRS split where reads go
// to a read replica and writes to the primary.
type PlanReader interface {
	// GetByID retrieves a single plan by its domain ID.
	GetByID(ctx context.Context, id string) (*aggregate.Plan, error)

	// GetAll retrieves all plans regardless of status.
	GetAll(ctx context.Context) ([]*aggregate.Plan, error)

	// GetActive retrieves only active (purchasable) plans.
	GetActive(ctx context.Context) ([]*aggregate.Plan, error)
}

// SubscriptionReader provides read-only access to subscriptions.
// Separated from SubscriptionRepository to support future CQRS split where
// reads go to a read replica and writes to the primary.
type SubscriptionReader interface {
	// GetByID retrieves a single subscription by its domain ID.
	GetByID(ctx context.Context, id string) (*aggregate.Subscription, error)

	// GetByUserID retrieves all subscriptions for a given user.
	GetByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error)

	// GetActiveByUserID retrieves only active subscriptions for a given user.
	GetActiveByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error)

	// Deprecated: Use GetAllCursor for new code. GetAll uses LIMIT/OFFSET which
	// degrades at high page numbers. Retained for backward compatibility.
	GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Subscription, error)

	// GetAllCursor retrieves a paginated list of all subscriptions using cursor-based
	// pagination keyed on (created_at DESC, id DESC). Nil cursor returns the first page.
	GetAllCursor(ctx context.Context, params CursorParams) ([]*aggregate.Subscription, error)
}

// InvoiceReader provides read-only access to invoices.
// Separated from InvoiceRepository to support future CQRS split where reads
// go to a read replica and writes to the primary.
type InvoiceReader interface {
	// GetByID retrieves a single invoice by its domain ID.
	GetByID(ctx context.Context, id string) (*aggregate.Invoice, error)

	// GetBySubscriptionID retrieves all invoices for a given subscription.
	GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.Invoice, error)

	// GetPendingByUserID retrieves unpaid invoices for a given user.
	GetPendingByUserID(ctx context.Context, userID string) ([]*aggregate.Invoice, error)

	// Deprecated: Use GetAllCursor for new code. GetAll uses LIMIT/OFFSET which
	// degrades at high page numbers. Retained for backward compatibility.
	GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Invoice, error)

	// GetAllCursor retrieves a paginated list of all invoices using cursor-based
	// pagination keyed on (created_at DESC, id DESC). Nil cursor returns the first page.
	GetAllCursor(ctx context.Context, params CursorParams) ([]*aggregate.Invoice, error)
}

// FamilyReader provides read-only access to family groups.
// Separated from FamilyRepository to support future CQRS split where reads
// go to a read replica and writes to the primary.
type FamilyReader interface {
	// GetByID retrieves a family group by its domain ID.
	GetByID(ctx context.Context, id string) (*aggregate.FamilyGroup, error)

	// GetByOwnerID retrieves a family group by the owner's user ID.
	GetByOwnerID(ctx context.Context, ownerID string) (*aggregate.FamilyGroup, error)
}
