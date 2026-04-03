package remnawave

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	// HeaderWebhookSecret is the HTTP header containing the HMAC-SHA256 hex
	// signature of the request body.
	HeaderWebhookSecret = "X-Webhook-Secret"
)

// WebhookHandler verifies HMAC-SHA256 signatures and dispatches parsed
// WebhookPayload values to a callback.
type WebhookHandler struct {
	secret    string
	onPayload func(WebhookPayload)
}

// NewWebhookHandler returns a handler that verifies webhook signatures against
// secret and forwards valid payloads to onPayload.
func NewWebhookHandler(secret string, onPayload func(WebhookPayload)) *WebhookHandler {
	return &WebhookHandler{
		secret:    secret,
		onPayload: onPayload,
	}
}

// ServeHTTP implements http.Handler. It reads the body (limited to
// httpconst.MaxWebhookBodySize), verifies the HMAC-SHA256 signature, parses
// the payload, and invokes the callback.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, httpconst.MaxWebhookBodySize))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	sigHex := r.Header.Get(HeaderWebhookSecret)
	if !h.verifySignature(body, sigHex) {
		http.Error(w, "invalid signature", http.StatusForbidden)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	h.onPayload(payload)
	w.WriteHeader(http.StatusOK)
}

// verifySignature computes HMAC-SHA256 of body using the shared secret and
// compares it to the provided hex-encoded signature in constant time.
func (h *WebhookHandler) verifySignature(body []byte, sigHex string) bool {
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expected := mac.Sum(nil)

	return hmac.Equal(sig, expected)
}
