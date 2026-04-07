package payment_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment/paymenttest"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// --- test helpers ---

type eventCollector struct {
	events []domainevent.Event
}

func (ec *eventCollector) Publish(_ context.Context, event domainevent.Event) error {
	ec.events = append(ec.events, event)
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// stubCreateCharge returns a CreateChargeHookFn that returns the given result.
func stubCreateCharge(result *payment.CreateChargeResult, err error) payment.CreateChargeHookFn {
	return func(_ context.Context, _ payment.CreateChargeRequest) (*payment.CreateChargeResult, error) {
		return result, err
	}
}

// stubVerifyWebhook returns a VerifyWebhookHookFn that returns the given result.
func stubVerifyWebhook(result *payment.VerifiedPayment, err error) payment.VerifyWebhookHookFn {
	return func(_ context.Context, _ payment.VerifyWebhookHookRequest) (*payment.VerifiedPayment, error) {
		return result, err
	}
}

// stubRefund returns a RefundHookFn that returns the given error.
func stubRefund(err error) payment.RefundHookFn {
	return func(_ context.Context, _ payment.RefundHookRequest) error {
		return err
	}
}

// noopCreateCharge is a no-op CreateChargeHookFn for tests that don't exercise it.
func noopCreateCharge(_ context.Context, _ payment.CreateChargeRequest) (*payment.CreateChargeResult, error) {
	return nil, errors.New("unexpected create charge call")
}

// noopVerifyWebhook is a no-op VerifyWebhookHookFn for tests that don't exercise it.
func noopVerifyWebhook(_ context.Context, _ payment.VerifyWebhookHookRequest) (*payment.VerifiedPayment, error) {
	return nil, errors.New("unexpected verify webhook call")
}

// noopRefund is a no-op RefundHookFn for tests that don't exercise it.
func noopRefund(_ context.Context, _ payment.RefundHookRequest) error {
	return errors.New("unexpected refund call")
}

// --- Tests ---

func TestCreateCharge_Success(t *testing.T) {
	chargeResult := &payment.CreateChargeResult{
		Provider:    "stripe",
		ExternalID:  "pi_123",
		CheckoutURL: "https://checkout.stripe.com/session/123",
		Status:      "pending",
	}

	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(
		stubCreateCharge(chargeResult, nil),
		noopVerifyWebhook,
		noopRefund,
		repo, pub, txmanagertest.NoopTxRunner{}, testLogger(), clock.NewReal(),
	)

	repo.On("CreatePayment", mock.Anything, mock.AnythingOfType("*payment.PaymentRecord")).Return(nil)

	result, err := facade.CreateCharge(context.Background(), payment.CreateChargeRequest{
		InvoiceID: "inv-1",
		Amount:    999,
		Currency:  "usd",
		UserID:    "user-1",
		UserEmail: "test@example.com",
		PlanName:  "Premium VPN",
		ReturnURL: "https://example.com/success",
		CancelURL: "https://example.com/cancel",
	})

	require.NoError(t, err)
	assert.Equal(t, "stripe", result.Provider)
	assert.Equal(t, "pi_123", result.ExternalID)
	assert.Equal(t, "https://checkout.stripe.com/session/123", result.CheckoutURL)
	assert.Len(t, pub.events, 1)
	assert.Equal(t, payment.EventChargeCreated, pub.events[0].Type)

	repo.AssertExpectations(t)
}

func TestCreateCharge_NoHandler(t *testing.T) {
	// Hook returns a result with empty provider/external_id -> should fail.
	incompleteResult := &payment.CreateChargeResult{
		Status: "pending",
	}

	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(
		stubCreateCharge(incompleteResult, nil),
		noopVerifyWebhook,
		noopRefund,
		repo, pub, txmanagertest.NoopTxRunner{}, testLogger(), clock.NewReal(),
	)

	_, err := facade.CreateCharge(context.Background(), payment.CreateChargeRequest{
		InvoiceID: "inv-1",
		Amount:    999,
		Currency:  "usd",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, payment.ErrPaymentFailed)
}

func TestCreateCharge_ValidationErrors(t *testing.T) {
	facade := payment.NewPaymentFacade(
		noopCreateCharge,
		noopVerifyWebhook,
		noopRefund,
		&paymenttest.MockPaymentRepo{}, &eventCollector{}, txmanagertest.NoopTxRunner{}, testLogger(), clock.NewReal(),
	)

	_, err := facade.CreateCharge(context.Background(), payment.CreateChargeRequest{
		Amount:   999,
		Currency: "usd",
	})
	assert.ErrorIs(t, err, payment.ErrMissingInvoiceID)

	_, err = facade.CreateCharge(context.Background(), payment.CreateChargeRequest{
		InvoiceID: "inv-1",
		Amount:    0,
		Currency:  "usd",
	})
	assert.ErrorIs(t, err, payment.ErrMissingAmount)

	_, err = facade.CreateCharge(context.Background(), payment.CreateChargeRequest{
		InvoiceID: "inv-1",
		Amount:    999,
	})
	assert.ErrorIs(t, err, payment.ErrMissingCurrency)
}

func TestVerifyWebhook_Success(t *testing.T) {
	verified := &payment.VerifiedPayment{
		Provider:   "stripe",
		ExternalID: "pi_123",
		InvoiceID:  "inv-1",
		Amount:     999,
		Currency:   "usd",
		Status:     "succeeded",
	}

	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(
		noopCreateCharge,
		stubVerifyWebhook(verified, nil),
		noopRefund,
		repo, pub, txmanagertest.NoopTxRunner{}, testLogger(), clock.NewReal(),
	)

	result, err := facade.VerifyWebhook(context.Background(), "stripe", map[string]string{
		"stripe-signature": "sig_abc",
	}, []byte(`{"type":"payment_intent.succeeded"}`))

	require.NoError(t, err)
	assert.Equal(t, "stripe", result.Provider)
	assert.Equal(t, "pi_123", result.ExternalID)
	assert.Equal(t, "inv-1", result.InvoiceID)
	assert.Equal(t, "succeeded", result.Status)
	assert.Len(t, pub.events, 1)
	assert.Equal(t, payment.EventWebhookReceived, pub.events[0].Type)
}

func TestVerifyWebhook_InvalidProvider(t *testing.T) {
	facade := payment.NewPaymentFacade(
		noopCreateCharge,
		noopVerifyWebhook,
		noopRefund,
		&paymenttest.MockPaymentRepo{}, &eventCollector{}, txmanagertest.NoopTxRunner{}, testLogger(), clock.NewReal(),
	)

	_, err := facade.VerifyWebhook(context.Background(), "", nil, nil)
	assert.ErrorIs(t, err, payment.ErrInvalidProvider)
}

func TestCheckIdempotency_NewWebhook(t *testing.T) {
	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(
		noopCreateCharge,
		noopVerifyWebhook,
		noopRefund,
		repo, pub, txmanagertest.NoopTxRunner{}, testLogger(), clock.NewReal(),
	)

	repo.On("CreateWebhookLog", mock.Anything, mock.AnythingOfType("*payment.WebhookLog")).Return(nil)

	isDuplicate, err := facade.CheckIdempotency(context.Background(), "stripe", "evt_123", []byte(`{}`))

	require.NoError(t, err)
	assert.False(t, isDuplicate)
	repo.AssertExpectations(t)
}

func TestCheckIdempotency_DuplicateWebhook(t *testing.T) {
	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(
		noopCreateCharge,
		noopVerifyWebhook,
		noopRefund,
		repo, pub, txmanagertest.NoopTxRunner{}, testLogger(), clock.NewReal(),
	)

	repo.On("CreateWebhookLog", mock.Anything, mock.AnythingOfType("*payment.WebhookLog")).Return(payment.ErrWebhookDuplicate)

	isDuplicate, err := facade.CheckIdempotency(context.Background(), "stripe", "evt_123", []byte(`{}`))

	require.NoError(t, err)
	assert.True(t, isDuplicate)
	repo.AssertExpectations(t)
}

func TestRefund_Success(t *testing.T) {
	// Initial read (outside tx) for provider/externalID.
	initialRecord := &payment.PaymentRecord{
		ID:         "pay-1",
		InvoiceID:  "inv-1",
		Provider:   "stripe",
		ExternalID: "pi_123",
		Amount:     999,
		Currency:   "usd",
		Status:     payment.PaymentCompleted,
	}
	// Re-read inside tx with FOR UPDATE lock.
	lockedRecord := &payment.PaymentRecord{
		ID:         "pay-1",
		InvoiceID:  "inv-1",
		Provider:   "stripe",
		ExternalID: "pi_123",
		Amount:     999,
		Currency:   "usd",
		Status:     payment.PaymentCompleted,
	}

	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(
		noopCreateCharge,
		noopVerifyWebhook,
		stubRefund(nil),
		repo, pub, txmanagertest.NoopTxRunner{}, testLogger(), clock.NewReal(),
	)

	repo.On("GetPaymentByID", mock.Anything, "pay-1").Return(initialRecord, nil)
	repo.On("GetPaymentByIDForUpdate", mock.Anything, "pay-1").Return(lockedRecord, nil)
	repo.On("UpdatePayment", mock.Anything, lockedRecord).Return(nil)

	err := facade.Refund(context.Background(), "pay-1", 999, "customer request")

	require.NoError(t, err)
	assert.Equal(t, payment.PaymentRefunded, lockedRecord.Status)
	assert.Len(t, pub.events, 1)
	assert.Equal(t, payment.EventRefundCompleted, pub.events[0].Type)

	repo.AssertExpectations(t)
}
