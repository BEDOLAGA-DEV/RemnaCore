package handler

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// PaymentWebhookHandler receives webhooks from payment providers (Stripe,
// BTCPay, etc.) and dispatches them through the payment facade for
// verification and processing.
type PaymentWebhookHandler struct {
	facade   *payment.PaymentFacade
	checkout *billingservice.CheckoutService
}

// NewPaymentWebhookHandler creates a PaymentWebhookHandler.
func NewPaymentWebhookHandler(
	facade *payment.PaymentFacade,
	checkout *billingservice.CheckoutService,
) *PaymentWebhookHandler {
	return &PaymentWebhookHandler{
		facade:   facade,
		checkout: checkout,
	}
}

// HandlePaymentWebhook receives webhooks from payment providers.
// Route: POST /api/webhooks/payment/{provider}
func (h *PaymentWebhookHandler) HandlePaymentWebhook(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, httpconst.MaxWebhookBodySize))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
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
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	// 2. Check idempotency — skip if already processed.
	isDuplicate, err := h.facade.CheckIdempotency(r.Context(), provider, verified.ExternalID, body)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "error"})
		return
	}
	if isDuplicate {
		writeJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
		return
	}

	// 3. Process based on payment status.
	if verified.Status == payment.VerifiedStatusSucceeded {
		// Complete the payment record.
		if _, err := h.facade.CompletePayment(r.Context(), provider, verified.ExternalID); err != nil {
			_ = h.facade.MarkWebhookFailed(r.Context(), provider, verified.ExternalID)
			writeJSON(w, http.StatusOK, map[string]string{"status": "error"})
			return
		}

		// Complete checkout (marks invoice paid, activates subscription).
		if err := h.checkout.CompleteCheckout(r.Context(), verified.InvoiceID); err != nil {
			_ = h.facade.MarkWebhookFailed(r.Context(), provider, verified.ExternalID)
			writeJSON(w, http.StatusOK, map[string]string{"status": "error"})
			return
		}
	}

	// 4. Mark webhook as processed.
	_ = h.facade.MarkWebhookProcessed(r.Context(), provider, verified.ExternalID)

	// Always return 200 OK immediately.
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
