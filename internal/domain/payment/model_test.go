package payment

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPaymentRecord_RecordsChargeCreatedEvent(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

	record, err := NewPaymentRecord("inv-1", "stripe", "pi_123", 999, "usd", now)
	require.NoError(t, err)

	require.True(t, record.HasEvents())
	events := record.DomainEvents()
	require.Len(t, events, 1)

	evt := events[0]
	assert.Equal(t, EventChargeCreated, evt.Type)
	assert.Equal(t, record.ID, evt.EntityID)
	assert.Equal(t, now, evt.Timestamp)

	payload, ok := evt.Data.(ChargeCreatedPayload)
	require.True(t, ok)
	assert.Equal(t, record.ID, payload.PaymentID)
	assert.Equal(t, "inv-1", payload.InvoiceID)
	assert.Equal(t, "stripe", payload.Provider)
	assert.Equal(t, "pi_123", payload.ExternalID)
	assert.Equal(t, int64(999), payload.Amount)
}

func TestNewPaymentRecord_Validation(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		invoiceID  string
		provider   string
		externalID string
		amount     int64
		currency   string
		wantErr    error
	}{
		{
			name:       "empty invoice ID",
			invoiceID:  "",
			provider:   "stripe",
			externalID: "pi_123",
			amount:     999,
			currency:   "usd",
			wantErr:    ErrMissingInvoiceID,
		},
		{
			name:       "empty provider",
			invoiceID:  "inv-1",
			provider:   "",
			externalID: "pi_123",
			amount:     999,
			currency:   "usd",
			wantErr:    ErrInvalidProvider,
		},
		{
			name:       "empty external ID",
			invoiceID:  "inv-1",
			provider:   "stripe",
			externalID: "",
			amount:     999,
			currency:   "usd",
			wantErr:    ErrMissingExternalID,
		},
		{
			name:       "zero amount",
			invoiceID:  "inv-1",
			provider:   "stripe",
			externalID: "pi_123",
			amount:     0,
			currency:   "usd",
			wantErr:    ErrMissingAmount,
		},
		{
			name:       "negative amount",
			invoiceID:  "inv-1",
			provider:   "stripe",
			externalID: "pi_123",
			amount:     -100,
			currency:   "usd",
			wantErr:    ErrMissingAmount,
		},
		{
			name:       "empty currency",
			invoiceID:  "inv-1",
			provider:   "stripe",
			externalID: "pi_123",
			amount:     999,
			currency:   "",
			wantErr:    ErrMissingCurrency,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := NewPaymentRecord(tt.invoiceID, tt.provider, tt.externalID, tt.amount, tt.currency, now)

			require.Error(t, err)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Nil(t, record)
		})
	}
}

func TestMarkCompleted_RecordsChargeCompletedEvent(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	record, err := NewPaymentRecord("inv-1", "stripe", "pi_123", 999, "usd", now)
	require.NoError(t, err)
	record.DomainEvents() // flush creation event

	completedAt := now.Add(time.Hour)
	err = record.MarkCompleted(completedAt)

	require.NoError(t, err)
	assert.Equal(t, PaymentCompleted, record.Status)
	assert.Equal(t, completedAt, record.UpdatedAt)

	require.True(t, record.HasEvents())
	events := record.DomainEvents()
	require.Len(t, events, 1)

	evt := events[0]
	assert.Equal(t, EventChargeCompleted, evt.Type)
	assert.Equal(t, record.ID, evt.EntityID)
	assert.Equal(t, completedAt, evt.Timestamp)

	payload, ok := evt.Data.(ChargeCompletedPayload)
	require.True(t, ok)
	assert.Equal(t, record.ID, payload.PaymentID)
	assert.Equal(t, "inv-1", payload.InvoiceID)
	assert.Equal(t, "stripe", payload.Provider)
	assert.Equal(t, int64(999), payload.Amount)
}

func TestMarkCompleted_RejectsNonPending(t *testing.T) {
	tests := []struct {
		name   string
		status PaymentStatus
	}{
		{name: "completed", status: PaymentCompleted},
		{name: "failed", status: PaymentFailed},
		{name: "refunded", status: PaymentRefunded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &PaymentRecord{ID: "pay-1", Status: tt.status}

			err := record.MarkCompleted(time.Now())

			require.ErrorIs(t, err, ErrInvalidPaymentState)
			assert.False(t, record.HasEvents())
		})
	}
}

func TestMarkFailed_RecordsChargeFailedEvent(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	record, err := NewPaymentRecord("inv-1", "stripe", "pi_123", 999, "usd", now)
	require.NoError(t, err)
	record.DomainEvents() // flush creation event

	failedAt := now.Add(time.Hour)
	err = record.MarkFailed("card declined", failedAt)

	require.NoError(t, err)
	assert.Equal(t, PaymentFailed, record.Status)
	assert.Equal(t, failedAt, record.UpdatedAt)

	require.True(t, record.HasEvents())
	events := record.DomainEvents()
	require.Len(t, events, 1)

	evt := events[0]
	assert.Equal(t, EventChargeFailed, evt.Type)
	assert.Equal(t, record.ID, evt.EntityID)
	assert.Equal(t, failedAt, evt.Timestamp)

	payload, ok := evt.Data.(ChargeFailedPayload)
	require.True(t, ok)
	assert.Equal(t, record.ID, payload.PaymentID)
	assert.Equal(t, "inv-1", payload.InvoiceID)
	assert.Equal(t, "stripe", payload.Provider)
	assert.Equal(t, "card declined", payload.Reason)
}

func TestMarkFailed_RejectsNonPending(t *testing.T) {
	tests := []struct {
		name   string
		status PaymentStatus
	}{
		{name: "completed", status: PaymentCompleted},
		{name: "failed", status: PaymentFailed},
		{name: "refunded", status: PaymentRefunded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &PaymentRecord{ID: "pay-1", Status: tt.status}

			err := record.MarkFailed("timeout", time.Now())

			require.ErrorIs(t, err, ErrInvalidPaymentState)
			assert.False(t, record.HasEvents())
		})
	}
}

func TestMarkRefunded_RecordsRefundCompletedEvent(t *testing.T) {
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	record, err := NewPaymentRecord("inv-1", "stripe", "pi_123", 999, "usd", now)
	require.NoError(t, err)
	record.DomainEvents() // flush creation event

	// Transition to completed first.
	completedAt := now.Add(time.Hour)
	require.NoError(t, record.MarkCompleted(completedAt))
	record.DomainEvents() // flush completed event

	refundedAt := completedAt.Add(time.Hour)
	refundAmount := int64(500)
	err = record.MarkRefunded(refundAmount, refundedAt)

	require.NoError(t, err)
	assert.Equal(t, PaymentRefunded, record.Status)
	assert.Equal(t, refundedAt, record.UpdatedAt)

	require.True(t, record.HasEvents())
	events := record.DomainEvents()
	require.Len(t, events, 1)

	evt := events[0]
	assert.Equal(t, EventRefundCompleted, evt.Type)
	assert.Equal(t, record.ID, evt.EntityID)
	assert.Equal(t, refundedAt, evt.Timestamp)

	payload, ok := evt.Data.(RefundCompletedPayload)
	require.True(t, ok)
	assert.Equal(t, record.ID, payload.PaymentID)
	assert.Equal(t, "inv-1", payload.InvoiceID)
	assert.Equal(t, "stripe", payload.Provider)
	assert.Equal(t, refundAmount, payload.Amount)
}

func TestMarkRefunded_RejectsNonCompleted(t *testing.T) {
	tests := []struct {
		name   string
		status PaymentStatus
	}{
		{name: "pending", status: PaymentPending},
		{name: "failed", status: PaymentFailed},
		{name: "refunded", status: PaymentRefunded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &PaymentRecord{ID: "pay-1", Status: tt.status}

			err := record.MarkRefunded(999, time.Now())

			require.ErrorIs(t, err, ErrInvalidPaymentState)
			assert.False(t, record.HasEvents())
		})
	}
}
