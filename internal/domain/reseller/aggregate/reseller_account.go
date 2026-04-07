package aggregate

import (
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

const (
	// PercentBase is the divisor used when converting a percentage integer
	// (0-100) to a fractional multiplier for commission calculations.
	PercentBase = 100

	// MinCommissionRate is the minimum allowed commission percentage.
	MinCommissionRate = 0

	// MaxCommissionRate is the maximum allowed commission percentage.
	MaxCommissionRate = PercentBase
)

// ResellerAccount represents a reseller's account linked to a specific tenant.
// It embeds EventRecorder to accumulate domain events during mutations.
// Services must call DomainEvents() after persisting the aggregate to
// retrieve and publish all pending events.
type ResellerAccount struct {
	domainevent.EventRecorder

	ID             string
	TenantID       string
	UserID         string
	CommissionRate int   // percent (0-100)
	Balance        int64 // cents, accumulated commission
	CreatedAt      time.Time
}

// NewResellerAccount creates a new ResellerAccount after validating the
// commission rate is within the allowed range. The creation event is recorded
// on the aggregate; callers must flush via DomainEvents() after persisting.
func NewResellerAccount(tenantID, userID string, commissionRate int, now time.Time) (*ResellerAccount, error) {
	if commissionRate < MinCommissionRate || commissionRate > MaxCommissionRate {
		return nil, ErrInvalidCommissionRate
	}

	a := &ResellerAccount{
		ID:             uuid.Must(uuid.NewV7()).String(),
		TenantID:       tenantID,
		UserID:         userID,
		CommissionRate: commissionRate,
		Balance:        0,
		CreatedAt:      now,
	}
	a.RecordEvent(domainevent.NewTyped(ResellerCreatedPayload{
		ResellerID: a.ID,
		TenantID:   tenantID,
		UserID:     userID,
	}, now, a.ID))
	return a, nil
}

// safeCommissionAmount calculates the commission amount with overflow protection.
// It returns ErrCommissionOverflow if the multiplication would exceed int64 range.
func safeCommissionAmount(saleAmount int64, rate int) (int64, error) {
	if saleAmount == 0 || rate == 0 {
		return 0, nil
	}
	r := int64(rate)
	if r > 0 && saleAmount > math.MaxInt64/r {
		return 0, ErrCommissionOverflow
	}
	if r < 0 && saleAmount < math.MaxInt64/r {
		return 0, ErrCommissionOverflow
	}
	return saleAmount * r / PercentBase, nil
}
