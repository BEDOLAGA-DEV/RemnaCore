package aggregate

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

var (
	// ErrInvoiceRequiresLineItems indicates an attempt to create an invoice
	// without any line items.
	ErrInvoiceRequiresLineItems = errors.New("at least one line item is required")

	// ErrInvoiceMustBeDraftForPending indicates an invalid transition from
	// a non-draft state to pending.
	ErrInvoiceMustBeDraftForPending = errors.New("invoice must be draft to mark pending")

	// ErrInvoiceMustBePendingForPaid indicates an invalid transition from
	// a non-pending state to paid.
	ErrInvoiceMustBePendingForPaid = errors.New("invoice must be pending to mark paid")

	// ErrInvoiceMustBePendingForFailed indicates an invalid transition from
	// a non-pending state to failed.
	ErrInvoiceMustBePendingForFailed = errors.New("invoice must be pending to mark failed")

	// ErrInvoiceMustBePaidForRefund indicates an invalid transition from
	// a non-paid state to refunded.
	ErrInvoiceMustBePaidForRefund = errors.New("invoice must be paid to refund")
)

// InvoiceStatus represents the current state of an invoice.
type InvoiceStatus string

const (
	InvoiceDraft    InvoiceStatus = "draft"
	InvoicePending  InvoiceStatus = "pending"
	InvoicePaid     InvoiceStatus = "paid"
	InvoiceFailed   InvoiceStatus = "failed"
	InvoiceRefunded InvoiceStatus = "refunded"
)

// Invoice is the aggregate root for a billing invoice.
// It embeds EventRecorder to accumulate domain events during mutations.
type Invoice struct {
	domainevent.EventRecorder

	ID             string
	SubscriptionID string
	UserID         string
	LineItems      []vo.LineItem
	Discounts      []vo.Discount
	Subtotal       vo.Money
	TotalDiscount  vo.Money
	Total          vo.Money
	Status         InvoiceStatus
	PaidAt         *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewInvoice creates an Invoice, calculating subtotal, discount and total.
// At least one line item is required.
func NewInvoice(subID, userID string, lineItems []vo.LineItem, discounts []vo.Discount, currency vo.Currency, now time.Time) (*Invoice, error) {
	if len(lineItems) == 0 {
		return nil, ErrInvoiceRequiresLineItems
	}

	subtotal, discount, total, err := calculateTotal(lineItems, discounts, currency)
	if err != nil {
		return nil, err
	}

	return &Invoice{
		ID:             uuid.New().String(),
		SubscriptionID: subID,
		UserID:         userID,
		LineItems:      lineItems,
		Discounts:      discounts,
		Subtotal:       subtotal,
		TotalDiscount:  discount,
		Total:          total,
		Status:         InvoiceDraft,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// MarkPending transitions the invoice from draft to pending.
func (inv *Invoice) MarkPending(now time.Time) error {
	if inv.Status != InvoiceDraft {
		return ErrInvoiceMustBeDraftForPending
	}
	inv.Status = InvoicePending
	inv.UpdatedAt = now
	return nil
}

// MarkPaid transitions the invoice from pending to paid.
func (inv *Invoice) MarkPaid(now time.Time) error {
	if inv.Status != InvoicePending {
		return ErrInvoiceMustBePendingForPaid
	}
	inv.Status = InvoicePaid
	inv.PaidAt = &now
	inv.UpdatedAt = now
	return nil
}

// MarkFailed transitions the invoice from pending to failed.
func (inv *Invoice) MarkFailed(now time.Time) error {
	if inv.Status != InvoicePending {
		return ErrInvoiceMustBePendingForFailed
	}
	inv.Status = InvoiceFailed
	inv.UpdatedAt = now
	return nil
}

// Refund transitions the invoice from paid to refunded.
func (inv *Invoice) Refund(now time.Time) error {
	if inv.Status != InvoicePaid {
		return ErrInvoiceMustBePaidForRefund
	}
	inv.Status = InvoiceRefunded
	inv.UpdatedAt = now
	return nil
}

// calculateTotal computes the subtotal from line items, sums discounts, and
// derives the final total (floored at zero).
func calculateTotal(items []vo.LineItem, discounts []vo.Discount, currency vo.Currency) (subtotal, discount, total vo.Money, err error) {
	subtotal = vo.Zero(currency)
	for _, item := range items {
		itemTotal := item.Total()
		subtotal, err = subtotal.Add(itemTotal)
		if err != nil {
			return vo.Money{}, vo.Money{}, vo.Money{}, err
		}
	}

	discount = vo.Zero(currency)
	for _, d := range discounts {
		switch d.Type {
		case vo.DiscountPercent:
			// Percent discount: percentage of subtotal
			discountAmount := subtotal.Amount * d.Value / vo.PercentBase
			disc := vo.NewMoney(discountAmount, currency)
			discount, err = discount.Add(disc)
			if err != nil {
				return vo.Money{}, vo.Money{}, vo.Money{}, err
			}
		case vo.DiscountFixed:
			disc := vo.NewMoney(d.Value, currency)
			discount, err = discount.Add(disc)
			if err != nil {
				return vo.Money{}, vo.Money{}, vo.Money{}, err
			}
		}
	}

	total, err = subtotal.Subtract(discount)
	if err != nil {
		return vo.Money{}, vo.Money{}, vo.Money{}, err
	}
	if total.Amount < 0 {
		total = vo.Zero(currency)
	}

	return subtotal, discount, total, nil
}
