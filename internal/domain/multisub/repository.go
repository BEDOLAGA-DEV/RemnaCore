package multisub

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
)

// BindingRepository defines persistence operations for the RemnawaveBinding aggregate.
type BindingRepository interface {
	GetByID(ctx context.Context, id string) (*aggregate.RemnawaveBinding, error)
	GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error)
	GetByPlatformUserID(ctx context.Context, userID string) ([]*aggregate.RemnawaveBinding, error)
	GetByRemnawaveUUID(ctx context.Context, rwUUID string) (*aggregate.RemnawaveBinding, error)
	GetActiveBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error)
	GetAllActive(ctx context.Context) ([]*aggregate.RemnawaveBinding, error)
	GetFailedWithRemnawaveUUID(ctx context.Context) ([]*aggregate.RemnawaveBinding, error)
	Create(ctx context.Context, binding *aggregate.RemnawaveBinding) error
	Update(ctx context.Context, binding *aggregate.RemnawaveBinding) error
	Delete(ctx context.Context, id string) error
}
