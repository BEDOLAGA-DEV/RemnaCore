package payment_test

import (
	"context"
	"encoding/json"
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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch/hookdispatchtest"
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

// --- Tests ---

func TestCreateCharge_Success(t *testing.T) {
	chargeResult := payment.CreateChargeResult{
		Provider:    "stripe",
		ExternalID:  "pi_123",
		CheckoutURL: "https://checkout.stripe.com/session/123",
		Status:      "pending",
	}
	resultJSON, _ := json.Marshal(chargeResult)

	dispatcher := &hookdispatchtest.MockDispatcher{}
	dispatcher.On("DispatchSync", mock.Anything, payment.HookCreateCharge, mock.AnythingOfType("json.RawMessage")).
		Return(json.RawMessage(resultJSON), nil)

	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(dispatcher, repo, pub, testLogger(), clock.NewReal())

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
	dispatcher.AssertExpectations(t)
}

func TestCreateCharge_NoHandler(t *testing.T) {
	// Dispatcher returns the original payload unchanged (no hooks registered).
	// The facade will try to unmarshal the CreateChargeRequest as a CreateChargeResult,
	// which won't have provider/external_id, so it should fail.
	dispatcher := &hookdispatchtest.MockDispatcher{}
	dispatcher.On("DispatchSync", mock.Anything, payment.HookCreateCharge, mock.AnythingOfType("json.RawMessage")).
		Return(json.RawMessage(`{"invoice_id":"inv-1","amount":999,"currency":"usd"}`), nil)

	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(dispatcher, repo, pub, testLogger(), clock.NewReal())

	_, err := facade.CreateCharge(context.Background(), payment.CreateChargeRequest{
		InvoiceID: "inv-1",
		Amount:    999,
		Currency:  "usd",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, payment.ErrPaymentFailed)

	dispatcher.AssertExpectations(t)
}

func TestCreateCharge_ValidationErrors(t *testing.T) {
	dispatcher := &hookdispatchtest.MockDispatcher{}
	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(dispatcher, repo, pub, testLogger(), clock.NewReal())

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
	verified := payment.VerifiedPayment{
		Provider:   "stripe",
		ExternalID: "pi_123",
		InvoiceID:  "inv-1",
		Amount:     999,
		Currency:   "usd",
		Status:     "succeeded",
	}
	verifiedJSON, _ := json.Marshal(verified)

	dispatcher := &hookdispatchtest.MockDispatcher{}
	dispatcher.On("DispatchSync", mock.Anything, payment.HookVerifyWebhook, mock.AnythingOfType("json.RawMessage")).
		Return(json.RawMessage(verifiedJSON), nil)

	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(dispatcher, repo, pub, testLogger(), clock.NewReal())

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

	dispatcher.AssertExpectations(t)
}

func TestVerifyWebhook_InvalidProvider(t *testing.T) {
	dispatcher := &hookdispatchtest.MockDispatcher{}
	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(dispatcher, repo, pub, testLogger(), clock.NewReal())

	_, err := facade.VerifyWebhook(context.Background(), "", nil, nil)
	assert.ErrorIs(t, err, payment.ErrInvalidProvider)
}

func TestCheckIdempotency_NewWebhook(t *testing.T) {
	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	dispatcher := &hookdispatchtest.MockDispatcher{}
	facade := payment.NewPaymentFacade(dispatcher, repo, pub, testLogger(), clock.NewReal())

	repo.On("CreateWebhookLog", mock.Anything, mock.AnythingOfType("*payment.WebhookLog")).Return(nil)

	isDuplicate, err := facade.CheckIdempotency(context.Background(), "stripe", "evt_123", []byte(`{}`))

	require.NoError(t, err)
	assert.False(t, isDuplicate)
	repo.AssertExpectations(t)
}

func TestCheckIdempotency_DuplicateWebhook(t *testing.T) {
	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	dispatcher := &hookdispatchtest.MockDispatcher{}
	facade := payment.NewPaymentFacade(dispatcher, repo, pub, testLogger(), clock.NewReal())

	repo.On("CreateWebhookLog", mock.Anything, mock.AnythingOfType("*payment.WebhookLog")).Return(payment.ErrWebhookDuplicate)

	isDuplicate, err := facade.CheckIdempotency(context.Background(), "stripe", "evt_123", []byte(`{}`))

	require.NoError(t, err)
	assert.True(t, isDuplicate)
	repo.AssertExpectations(t)
}

func TestRefund_Success(t *testing.T) {
	record := &payment.PaymentRecord{
		ID:         "pay-1",
		InvoiceID:  "inv-1",
		Provider:   "stripe",
		ExternalID: "pi_123",
		Amount:     999,
		Currency:   "usd",
		Status:     payment.PaymentCompleted,
	}

	refundResult := map[string]string{"status": "refunded"}
	resultJSON, _ := json.Marshal(refundResult)

	dispatcher := &hookdispatchtest.MockDispatcher{}
	dispatcher.On("DispatchSync", mock.Anything, payment.HookRefund, mock.AnythingOfType("json.RawMessage")).
		Return(json.RawMessage(resultJSON), nil)

	repo := &paymenttest.MockPaymentRepo{}
	pub := &eventCollector{}
	facade := payment.NewPaymentFacade(dispatcher, repo, pub, testLogger(), clock.NewReal())

	repo.On("GetPaymentByID", mock.Anything, "pay-1").Return(record, nil)
	repo.On("UpdatePayment", mock.Anything, record).Return(nil)

	err := facade.Refund(context.Background(), "pay-1", 999, "customer request")

	require.NoError(t, err)
	assert.Equal(t, payment.PaymentRefunded, record.Status)
	assert.Len(t, pub.events, 1)
	assert.Equal(t, payment.EventRefundCompleted, pub.events[0].Type)

	repo.AssertExpectations(t)
	dispatcher.AssertExpectations(t)
}
