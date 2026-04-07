package aggregate

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// CommissionStatus is an alias for the vo.CommissionStatus value object.
type CommissionStatus = vo.CommissionStatus

// Re-exported status constants for backward compatibility within aggregate.
const (
	CommissionPending = vo.CommissionPending
	CommissionPaid    = vo.CommissionPaid
)

// commissionTransitions defines the valid state transitions for a commission.
var commissionTransitions = map[CommissionStatus][]CommissionStatus{
	CommissionPending: {CommissionPaid},
	CommissionPaid:    {},
}

// Commission records a commission earned by a reseller for a sale.
// It embeds EventRecorder to accumulate domain events during mutations.
// Services must call DomainEvents() after persisting the aggregate to
// retrieve and publish all pending events.
type Commission struct {
	domainevent.EventRecorder

	ID         string
	ResellerID string
	SaleID     string // subscription or invoice ID
	Amount     int64  // cents
	Currency   string
	Status     CommissionStatus
	CreatedAt  time.Time
	PaidAt     *time.Time
}

// CanTransitionTo reports whether the commission can move to the target status.
func (c *Commission) CanTransitionTo(target CommissionStatus) bool {
	allowed, ok := commissionTransitions[c.Status]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

// MarkPaid transitions the commission from pending to paid.
func (c *Commission) MarkPaid(now time.Time) error {
	if !c.CanTransitionTo(CommissionPaid) {
		return ErrCommissionAlreadyPaid
	}
	c.Status = CommissionPaid
	c.PaidAt = &now
	c.RecordEvent(domainevent.NewTyped(CommissionPaidPayload{
		CommissionID: c.ID,
		ResellerID:   c.ResellerID,
		Amount:       c.Amount,
	}, now, c.ID))
	return nil
}

// NewCommission calculates and creates a commission record for a sale.
// The creation event is recorded on the aggregate; callers must flush
// via DomainEvents() after persisting. Returns ErrCommissionOverflow if
// the sale amount and rate would cause integer overflow.
func NewCommission(resellerID, saleID string, saleAmount int64, commissionRate int, currency string, now time.Time) (*Commission, error) {
	amount, err := safeCommissionAmount(saleAmount, commissionRate)
	if err != nil {
		return nil, fmt.Errorf("commission for sale %s: %w", saleID, err)
	}

	c := &Commission{
		ID:         uuid.Must(uuid.NewV7()).String(),
		ResellerID: resellerID,
		SaleID:     saleID,
		Amount:     amount,
		Currency:   currency,
		Status:     CommissionPending,
		CreatedAt:  now,
	}
	c.RecordEvent(domainevent.NewTyped(CommissionCreatedPayload{
		CommissionID: c.ID,
		ResellerID:   resellerID,
		Amount:       amount,
	}, now, c.ID))
	return c, nil
}
