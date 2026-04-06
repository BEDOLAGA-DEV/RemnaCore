package handler

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// Webhook response status values returned to payment providers.
const (
	WebhookStatusOK        = "ok"
	WebhookStatusIgnored   = "ignored"
	WebhookStatusError     = "error"
	WebhookStatusDuplicate = "duplicate"
)

// PaymentWebhookHandler receives webhooks from payment providers (Stripe,
// BTCPay, etc.) and dispatches them through the payment facade for
// verification and processing.
//
// The handler talks exclusively to the payment bounded context. Billing-side
// effects (invoice payment, subscription activation) happen asynchronously:
// CompletePayment publishes a payment.charge_completed event via the outbox,
// and BillingEventConsumer subscribes to it to call CompleteCheckout.
type PaymentWebhookHandler struct {
	facade *payment.PaymentFacade
}

// NewPaymentWebhookHandler creates a PaymentWebhookHandler.
func NewPaymentWebhookHandler(facade *payment.PaymentFacade) *PaymentWebhookHandler {
	return &PaymentWebhookHandler{facade: facade}
}

// HandlePaymentWebhook receives webhooks from payment providers.
// Route: POST /api/webhooks/payment/{provider}
func (h *PaymentWebhookHandler) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if provider == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("provider is required"))
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, httpconst.MaxWebhookBodySize))
	if err != nil {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("failed to read request body"))
		return
	}
	defer r.Body.Close()

	// Collect relevant headers for signature verification.
	headers := make(map[string]string)
	for key, vals := range r.Header {
		if len(vals) > 0 {
			headers[key] = vals[0]
		}
	}

	// 1. Verify webhook via payment facade (dispatches to plugin).
	verified, err := h.facade.VerifyWebhook(r.Context(), provider, headers, body)
	if err != nil {
		// Return 200 to prevent retries from the payment provider even on
		// verification failure. Errors are logged internally.
		writeJSON(w, http.StatusOK, map[string]string{"status": WebhookStatusIgnored})
		return
	}

	// 2. Check idempotency -- skip if already processed.
	isDuplicate, err := h.facade.CheckIdempotency(r.Context(), provider, verified.ExternalID, body)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": WebhookStatusError})
		return
	}
	if isDuplicate {
		writeJSON(w, http.StatusOK, map[string]string{"status": WebhookStatusDuplicate})
		return
	}

	// 3. Process based on payment status. CompletePayment publishes a
	// payment.charge_completed event via the outbox; billing subscribes to it
	// asynchronously via BillingEventConsumer to call CompleteCheckout.
	if verified.Status == payment.VerifiedStatusSucceeded {
		if _, err := h.facade.CompletePayment(r.Context(), provider, verified.ExternalID); err != nil {
			_ = h.facade.MarkWebhookFailed(r.Context(), provider, verified.ExternalID)
			writeJSON(w, http.StatusOK, map[string]string{"status": WebhookStatusError})
			return
		}
	}

	// 4. Mark webhook as processed.
	_ = h.facade.MarkWebhookProcessed(r.Context(), provider, verified.ExternalID)

	// Always return 200 OK immediately.
	writeJSON(w, http.StatusOK, map[string]string{"status": WebhookStatusOK})
}
