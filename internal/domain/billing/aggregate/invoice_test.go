package aggregate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

func TestNewInvoice_Valid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Premium Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
		vo.NewLineItem("Extra Traffic", vo.LineItemAddon, vo.NewMoney(299, vo.CurrencyUSD), 1),
	}

	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)

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
	_, err := NewInvoice("sub-1", "user-1", nil, nil, vo.CurrencyUSD)

	require.Error(t, err)
	assert.ErrorContains(t, err, "line item")
}

func TestNewInvoice_EmptyLineItems(t *testing.T) {
	_, err := NewInvoice("sub-1", "user-1", []vo.LineItem{}, nil, vo.CurrencyUSD)

	require.Error(t, err)
	assert.ErrorContains(t, err, "line item")
}

func TestNewInvoice_WithPercentDiscount(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Premium Plan", vo.LineItemPlan, vo.NewMoney(10000, vo.CurrencyUSD), 1),
	}
	discount, err := vo.NewPercentDiscount(20, "SAVE20", nil)
	require.NoError(t, err)

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{discount}, vo.CurrencyUSD)

	require.NoError(t, err)
	assert.Equal(t, int64(10000), inv.Subtotal.Amount)
	assert.Equal(t, int64(2000), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(8000), inv.Total.Amount)
}

func TestNewInvoice_WithFixedDiscount(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Premium Plan", vo.LineItemPlan, vo.NewMoney(10000, vo.CurrencyUSD), 1),
	}
	discount, err := vo.NewFixedDiscount(2500, vo.CurrencyUSD, "FLAT25", nil)
	require.NoError(t, err)

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{discount}, vo.CurrencyUSD)

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

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{discount}, vo.CurrencyUSD)

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

	inv, err := NewInvoice("sub-1", "user-1", items, []vo.Discount{d1, d2}, vo.CurrencyUSD)

	require.NoError(t, err)
	assert.Equal(t, int64(10000), inv.Subtotal.Amount)
	assert.Equal(t, int64(1500), inv.TotalDiscount.Amount)
	assert.Equal(t, int64(8500), inv.Total.Amount)
}

func TestNewInvoice_WithQuantity(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Extra Nodes", vo.LineItemAddon, vo.NewMoney(200, vo.CurrencyUSD), 3),
	}

	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)

	require.NoError(t, err)
	// 200 * 3 = 600
	assert.Equal(t, int64(600), inv.Subtotal.Amount)
	assert.Equal(t, int64(600), inv.Total.Amount)
}

func TestInvoice_MarkPending(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)
	require.NoError(t, err)

	err = inv.MarkPending()

	require.NoError(t, err)
	assert.Equal(t, InvoicePending, inv.Status)
}

func TestInvoice_MarkPaid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending())

	err = inv.MarkPaid()

	require.NoError(t, err)
	assert.Equal(t, InvoicePaid, inv.Status)
	assert.NotNil(t, inv.PaidAt)
}

func TestInvoice_MarkPaid_FromDraft_Invalid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)
	require.NoError(t, err)

	err = inv.MarkPaid()

	require.Error(t, err)
	assert.ErrorContains(t, err, "pending")
}

func TestInvoice_MarkPaid_AlreadyPaid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending())
	require.NoError(t, inv.MarkPaid())

	err = inv.MarkPaid()

	require.Error(t, err)
	assert.ErrorContains(t, err, "pending")
}

func TestInvoice_MarkFailed(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending())

	err = inv.MarkFailed()

	require.NoError(t, err)
	assert.Equal(t, InvoiceFailed, inv.Status)
}

func TestInvoice_Refund(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending())
	require.NoError(t, inv.MarkPaid())

	err = inv.Refund()

	require.NoError(t, err)
	assert.Equal(t, InvoiceRefunded, inv.Status)
}

func TestInvoice_Refund_NotPaid(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)
	require.NoError(t, err)
	require.NoError(t, inv.MarkPending())

	err = inv.Refund()

	require.Error(t, err)
	assert.ErrorContains(t, err, "paid")
}

func TestInvoice_MarkFailed_FromDraft(t *testing.T) {
	items := []vo.LineItem{
		vo.NewLineItem("Plan", vo.LineItemPlan, vo.NewMoney(999, vo.CurrencyUSD), 1),
	}
	inv, err := NewInvoice("sub-1", "user-1", items, nil, vo.CurrencyUSD)
	require.NoError(t, err)

	err = inv.MarkFailed()

	require.Error(t, err)
	assert.ErrorContains(t, err, "pending")
}
