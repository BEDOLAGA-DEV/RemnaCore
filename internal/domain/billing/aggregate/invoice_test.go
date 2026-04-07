package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

func TestAllInvoiceStatusesHaveTransitions(t *testing.T) {
	err := ValidateInvoiceTransitions()
	require.NoError(t, err)
}

func TestNewInvoice_Valid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Premium Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
		vo.NewLineItem("Extra Traffic", vo.LineItemAddon, vo.NewMoney(299, vo.CurrencyUSD), 1),
	}

	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())

	require.NoError(t, err)
	assert.NotEmpty(t, inv.ID)
	assert.Equal(t, "sub-1", inv.SubscriptionID)
	assert.Equal(t, "user-1", inv.UserID)
	assert.Len(t, inv.LineItems, 2)
	assert.Equal(t, int64(1298), inv.Subtotal.Amount)
	assert.True(t, inv.TotalDiscount.IsZero())
	assert.Equal(t, int64(1298), inv.Total.Amount)
	assert.Equal(t, InvoiceDraft, inv.Status)
	assert.Nil(t, inv.PaidAt)
}

func TestNewInvoice_NoLineItems(t *testing.T) {
	_, err := NewInvoice("sub-1", "user-1", nil, nil, vo.CurrencyUSD, time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "line item")
}

func TestNewInvoice_EmptyLineItems(t *testing.T) {
	_, err := NewInvoice("sub-1", "user-1", []vo.LineItem{}, nil, vo.CurrencyUSD, time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "line item")
}

func TestNewInvoice_WithPercentDiscount(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Premium Plan", vo.LineItemPlan, vo.NewMoney(10000, vo.CurrencyUSD), 1),
	}
	discount, err := vo.NewPercentDiscount(2000, "SAVE20", nil) // 20% in basis points
	require.NoError(t, err)

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{discount}, vo.CurrencyUSD, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(10000), inv.Subtotal.Amount)
	assert.Equal(t, int64(2000), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(8000), inv.Total.Amount)
}

// TestNewInvoice_PercentDiscountFloorRounding verifies the platform's rounding
// policy: fractional cents from percent discounts are truncated (floor),
// producing a smaller discount and ensuring the merchant never undercharges.
func TestNewInvoice_PercentDiscountFloorRounding(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	// 33.33% of 999 cents = 332.9667 → floor to 332 (integer truncation).
	// The discount is computed as subtotal * pct / PercentBase, not as
	// subtotal - subtotal * (PercentBase - pct) / PercentBase, so the floor
	// applies directly to the discount amount.
	discount, err := vo.NewPercentDiscount(3333, "THIRD", nil)
	require.NoError(t, err)

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{discount}, vo.CurrencyUSD, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(999), inv.Subtotal.Amount)
	assert.Equal(t, int64(332), inv.TotalDiscount.Amount, "floor rounding: 999*3333/10000=332")
	assert.Equal(t, int64(667), inv.Total.Amount)
}

func TestNewInvoice_WithFixedDiscount(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Premium Plan", vo.LineItemPlan, vo.NewMoney(10000, vo.CurrencyUSD), 1),
	}
	discount, err := vo.NewFixedDiscount(2500, vo.CurrencyUSD, "FLAT25", nil)
	require.NoError(t, err)

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{discount}, vo.CurrencyUSD, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(10000), inv.Subtotal.Amount)
	assert.Equal(t, int64(2500), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(7500), inv.Total.Amount)
}

func TestNewInvoice_DiscountExceedsSubtotal_FloorZero(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Basic Plan", vo.LineItemPlan, vo.NewMoney(500, vo.CurrencyUSD), 1),
	}
	discount, err := vo.NewFixedDiscount(2000, vo.CurrencyUSD, "BIG", nil)
	require.NoError(t, err)

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{discount}, vo.CurrencyUSD, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(500), inv.Subtotal.Amount)
	assert.Equal(t, int64(2000), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(0), inv.Total.Amount, "total must be floored at zero")
}

func TestNewInvoice_MultipleDiscounts(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(10000, vo.CurrencyUSD), 1),
	}
	d1, err := vo.NewFixedDiscount(1000, vo.CurrencyUSD, "FLAT10", nil)
	require.NoError(t, err)
	d2, err := vo.NewFixedDiscount(500, vo.CurrencyUSD, "FLAT5", nil)
	require.NoError(t, err)

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{d1, d2}, vo.CurrencyUSD, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(10000), inv.Subtotal.Amount)
	assert.Equal(t, int64(1500), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(8500), inv.Total.Amount)
}

func TestNewInvoice_WithQuantity(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Extra Nodes", vo.LineItemAddon, vo.NewMoney(200, vo.CurrencyUSD), 3),
	}

	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())

	require.NoError(t, err)
	// 200 * 3 = 600
	assert.Equal(t, int64(600), inv.Subtotal.Amount)
	assert.Equal(t, int64(600), inv.Total.Amount)
}

func TestInvoice_MarkPending(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	err = inv.MarkPending(time.Now())

	require.NoError(t, err)
	assert.Equal(t, InvoicePending, inv.Status)
}

func TestInvoice_MarkPaid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))

	err = inv.MarkPaid(time.Now())

	require.NoError(t, err)
	assert.Equal(t, InvoicePaid, inv.Status)
	assert.NotNil(t, inv.PaidAt)
}

func TestInvoice_MarkPaid_FromDraft_Invalid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	err = inv.MarkPaid(time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "pending")
}

func TestInvoice_MarkPaid_AlreadyPaid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))
	require.NoError(t, inv.MarkPaid(time.Now()))

	err = inv.MarkPaid(time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "pending")
}

func TestInvoice_MarkFailed(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))

	err = inv.MarkFailed(time.Now())

	require.NoError(t, err)
	assert.Equal(t, InvoiceFailed, inv.Status)
}

func TestInvoice_Refund(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))
	require.NoError(t, inv.MarkPaid(time.Now()))

	err = inv.Refund(time.Now())

	require.NoError(t, err)
	assert.Equal(t, InvoiceRefunded, inv.Status)
}

func TestInvoice_Refund_NotPaid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))

	err = inv.Refund(time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "paid")
}

func TestInvoice_MarkFailed_FromDraft(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	err = inv.MarkFailed(time.Now())

	require.Error(t, err)
	assert.ErrorContains(t, err, "pending")
}

// --- ApplyPricingModification ---

func TestInvoice_ApplyPricingModification_SubtotalOnly(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	inv.DomainEvents() // drain creation event

	newSubtotal := int64(800)
	now := time.Now()
	err = inv.ApplyPricingModification(&newSubtotal, nil, "promo_subtotal", now)

	require.NoError(t, err)
	assert.Equal(t, int64(800), inv.Subtotal.Amount)
	assert.True(t, inv.TotalDiscount.IsZero(), "discount should remain zero")
	assert.Equal(t, int64(800), inv.Total.Amount)
	assert.Equal(t, "promo_subtotal", inv.PricingReason)
	assert.Equal(t, now, inv.UpdatedAt)

	events := inv.DomainEvents()
	require.Len(t, events, 1)
	assert.Equal(t, EventInvTotalOverridden, events[0].Type)
}

func TestInvoice_ApplyPricingModification_DiscountOnly(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	discountAmt := int64(300)
	now := time.Now()
	err = inv.ApplyPricingModification(nil, &discountAmt, "loyalty_10pct", now)

	require.NoError(t, err)
	assert.Equal(t, int64(1000), inv.Subtotal.Amount, "subtotal should be unchanged")
	assert.Equal(t, int64(300), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(700), inv.Total.Amount)
	assert.Equal(t, "loyalty_10pct", inv.PricingReason)
}

func TestInvoice_ApplyPricingModification_FloorAtZero(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(500, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	discountAmt := int64(9999)
	now := time.Now()
	err = inv.ApplyPricingModification(nil, &discountAmt, "massive_discount", now)

	require.NoError(t, err)
	assert.Equal(t, int64(500), inv.Subtotal.Amount)
	assert.Equal(t, int64(9999), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(0), inv.Total.Amount, "total must be floored at zero")
}

func TestInvoice_ApplyPricingModification_BothFields(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	newSubtotal := int64(1200)
	discountAmt := int64(200)
	now := time.Now()
	err = inv.ApplyPricingModification(&newSubtotal, &discountAmt, "bundle_adjust", now)

	require.NoError(t, err)
	assert.Equal(t, int64(1200), inv.Subtotal.Amount)
	assert.Equal(t, int64(200), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(1000), inv.Total.Amount)
	assert.Equal(t, "bundle_adjust", inv.PricingReason)
}

func TestInvoice_ApplyPricingModification_NilFields_NoChange(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	originalSubtotal := inv.Subtotal.Amount
	originalTotal := inv.Total.Amount
	now := time.Now()
	err = inv.ApplyPricingModification(nil, nil, "", now)

	require.NoError(t, err)
	assert.Equal(t, originalSubtotal, inv.Subtotal.Amount, "subtotal should be unchanged")
	assert.Equal(t, originalTotal, inv.Total.Amount, "total should be unchanged")
	assert.Empty(t, inv.PricingReason)
}

func TestInvoice_ApplyPricingModification_Pending_Error(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))

	newSubtotal := int64(500)
	err = inv.ApplyPricingModification(&newSubtotal, nil, "late_mod", time.Now())

	require.ErrorIs(t, err, ErrInvoiceNotDraft)
	assert.Equal(t, int64(1000), inv.Subtotal.Amount, "subtotal should remain unchanged")
}

func TestInvoice_ApplyPricingModification_Paid_Error(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))
	require.NoError(t, inv.MarkPaid(time.Now()))

	newSubtotal := int64(500)
	err = inv.ApplyPricingModification(&newSubtotal, nil, "too_late", time.Now())

	require.ErrorIs(t, err, ErrInvoiceNotDraft)
}

// --- ApplyDiscount ---

func TestInvoice_ApplyDiscount_Draft_FixedDiscount(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Premium Plan", vo.LineItemPlan, vo.NewMoney(10000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	// Drain creation event.
	inv.DomainEvents()

	discount, err := vo.NewFixedDiscount(2500, vo.CurrencyUSD, "FLAT25", nil)
	require.NoError(t, err)

	now := time.Now()
	err = inv.ApplyDiscount(discount, now)

	require.NoError(t, err)
	assert.Len(t, inv.Discounts, 1)
	assert.Equal(t, int64(10000), inv.Subtotal.Amount)
	assert.Equal(t, int64(2500), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(7500), inv.Total.Amount)
	assert.Equal(t, now, inv.UpdatedAt)

	events := inv.DomainEvents()
	require.Len(t, events, 1)
	assert.Equal(t, EventInvDiscountApplied, events[0].Type)
	payload, ok := events[0].Data.(InvDiscountAppliedPayload)
	require.True(t, ok)
	assert.Equal(t, inv.ID, payload.InvoiceID)
	assert.Equal(t, "FLAT25", payload.DiscountCode)
	assert.Equal(t, int64(2500), payload.DiscountAmount)
}

func TestInvoice_ApplyDiscount_Draft_PercentDiscount(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(10000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	inv.DomainEvents()

	discount, err := vo.NewPercentDiscount(5000, "HALF", nil) // 50%
	require.NoError(t, err)

	err = inv.ApplyDiscount(discount, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(10000), inv.Subtotal.Amount)
	assert.Equal(t, int64(5000), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(5000), inv.Total.Amount)
}

func TestInvoice_ApplyDiscount_Pending_Error(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))

	discount, err := vo.NewFixedDiscount(100, vo.CurrencyUSD, "LATE", nil)
	require.NoError(t, err)

	err = inv.ApplyDiscount(discount, time.Now())

	require.ErrorIs(t, err, ErrInvoiceNotDraft)
}

func TestInvoice_ApplyDiscount_Paid_Error(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))
	require.NoError(t, inv.MarkPaid(time.Now()))

	discount, err := vo.NewFixedDiscount(100, vo.CurrencyUSD, "TOOLATE", nil)
	require.NoError(t, err)

	err = inv.ApplyDiscount(discount, time.Now())

	require.ErrorIs(t, err, ErrInvoiceNotDraft)
}

func TestInvoice_ApplyDiscount_StacksWithExistingDiscounts(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(10000, vo.CurrencyUSD), 1),
	}
	d1, err := vo.NewFixedDiscount(1000, vo.CurrencyUSD, "FIRST", nil)
	require.NoError(t, err)

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{d1}, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	inv.DomainEvents()

	d2, err := vo.NewFixedDiscount(500, vo.CurrencyUSD, "SECOND", nil)
	require.NoError(t, err)

	err = inv.ApplyDiscount(d2, time.Now())

	require.NoError(t, err)
	assert.Len(t, inv.Discounts, 2)
	assert.Equal(t, int64(10000), inv.Subtotal.Amount)
	assert.Equal(t, int64(1500), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(8500), inv.Total.Amount)
}

func TestInvoice_ApplyDiscount_FloorAtZero(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(500, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	discount, err := vo.NewFixedDiscount(9999, vo.CurrencyUSD, "HUGE", nil)
	require.NoError(t, err)

	err = inv.ApplyDiscount(discount, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(0), inv.Total.Amount, "total must be floored at zero")
}

// --- OverrideTotal ---

func TestInvoice_OverrideTotal_Draft_Success(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	inv.DomainEvents()

	newTotal := vo.NewMoney(750, vo.CurrencyUSD)
	now := time.Now()
	err = inv.OverrideTotal(newTotal, "plugin_pricing", now)

	require.NoError(t, err)
	assert.Equal(t, int64(750), inv.Total.Amount)
	assert.Equal(t, now, inv.UpdatedAt)

	events := inv.DomainEvents()
	require.Len(t, events, 1)
	assert.Equal(t, EventInvTotalOverridden, events[0].Type)
	payload, ok := events[0].Data.(InvTotalOverriddenPayload)
	require.True(t, ok)
	assert.Equal(t, inv.ID, payload.InvoiceID)
	assert.Equal(t, int64(1000), payload.OldAmount)
	assert.Equal(t, int64(750), payload.NewAmount)
	assert.Equal(t, "plugin_pricing", payload.Reason)
}

func TestInvoice_OverrideTotal_ZeroAmount_Success(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	err = inv.OverrideTotal(vo.Zero(vo.CurrencyUSD), "free_trial", time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(0), inv.Total.Amount)
}

func TestInvoice_OverrideTotal_NegativeAmount_Error(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	negative := vo.NewMoney(-500, vo.CurrencyUSD)
	err = inv.OverrideTotal(negative, "bad_value", time.Now())

	require.ErrorIs(t, err, ErrNegativeTotal)
	assert.Equal(t, int64(1000), inv.Total.Amount, "total should remain unchanged")
}

func TestInvoice_OverrideTotal_CurrencyMismatch_Error(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)

	eurTotal := vo.NewMoney(900, vo.CurrencyEUR)
	err = inv.OverrideTotal(eurTotal, "wrong_currency", time.Now())

	require.ErrorIs(t, err, vo.ErrCurrencyMismatch)
	assert.Equal(t, int64(1000), inv.Total.Amount, "total should remain unchanged")
}

func TestInvoice_OverrideTotal_Pending_Error(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))

	newTotal := vo.NewMoney(500, vo.CurrencyUSD)
	err = inv.OverrideTotal(newTotal, "too_late", time.Now())

	require.ErrorIs(t, err, ErrInvoiceNotDraft)
}

func TestInvoice_OverrideTotal_Paid_Error(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(1000, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD, time.Now())
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending(time.Now()))
	require.NoError(t, inv.MarkPaid(time.Now()))

	newTotal := vo.NewMoney(500, vo.CurrencyUSD)
	err = inv.OverrideTotal(newTotal, "too_late", time.Now())

	require.ErrorIs(t, err, ErrInvoiceNotDraft)
}
