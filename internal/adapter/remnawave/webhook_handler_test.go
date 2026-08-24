package remnawave

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func computeHMAC(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHandler_ValidSignature(t *testing.T) {
	secret := "webhook-secret-key"
	body := `{"scope":"user","event":"created","timestamp":"2026-01-01T00:00:00Z","data":{"uuid":"u-1"}}`

	var received *WebhookPayload
	handler := NewWebhookHandlerWithSecret(secret, func(p WebhookPayload) {
		received = &p
	})

	sig := computeHMAC(body, secret)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set(HeaderWebhookSecret, sig)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, received)
	assert.Equal(t, "user", received.Scope)
	assert.Equal(t, "created", received.Event)
	assert.Contains(t, string(received.Data), "u-1")
}

func TestWebhookHandler_InvalidSignature(t *testing.T) {
	handler := NewWebhookHandlerWithSecret("correct-secret", func(p WebhookPayload) {
		t.Fatal("callback should not be invoked for invalid signature")
	})

	body := `{"scope":"user","event":"disabled","timestamp":"2026-01-01T00:00:00Z","data":{}}`
	wrongSig := computeHMAC(body, "wrong-secret")

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set(HeaderWebhookSecret, wrongSig)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestWebhookHandler_EmptyBody(t *testing.T) {
	handler := NewWebhookHandlerWithSecret("secret", func(p WebhookPayload) {
		t.Fatal("callback should not be invoked for empty body")
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(""))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestHeaderWebhookSecret_MatchesPanel locks the signature header name to the
// value the Remnawave panel actually sends (2.6.4–2.8.0).
func TestHeaderWebhookSecret_MatchesPanel(t *testing.T) {
	assert.Equal(t, "X-Remnawave-Signature", HeaderWebhookSecret)
}

// TestWebhookHandler_EmptySecretRejectsAll verifies fail-closed behavior: with
// no configured secret, even a signature correctly computed over the empty key
// is rejected (otherwise webhooks would be forgeable).
func TestWebhookHandler_EmptySecretRejectsAll(t *testing.T) {
	called := false
	handler := NewWebhookHandlerWithSecret("", func(WebhookPayload) { called = true })

	body := `{"scope":"user","event":"created"}`
	// Signature an attacker would compute against the empty key.
	sig := computeHMAC(body, "")

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/remnawave", strings.NewReader(body))
	req.Header.Set(HeaderWebhookSecret, sig)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.False(t, called, "callback must not fire when the secret is unset")
}

// The signing secret is administered through the Remnawave plugin, so it must
// be read per request: an operator setting or rotating it has to take effect
// without restarting the platform.
func TestWebhookHandler_ReadsSecretPerRequest(t *testing.T) {
	current := ""
	h := NewWebhookHandler(
		func(context.Context) string { return current },
		func(WebhookPayload) {},
	)

	body := []byte(`{"event":"x"}`)
	sign := func(secret string) string {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		return hex.EncodeToString(mac.Sum(nil))
	}
	post := func(sig string) int {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set(HeaderWebhookSecret, sig)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	// Before configuration the handler fails closed, whatever it is sent.
	require.Equal(t, http.StatusForbidden, post(sign("later-secret")))

	// Configuring the plugin takes effect on the next request, no restart.
	current = "later-secret"
	require.Equal(t, http.StatusOK, post(sign("later-secret")))

	// Rotating it invalidates signatures made with the previous value.
	current = "rotated-secret"
	require.Equal(t, http.StatusForbidden, post(sign("later-secret")))
	require.Equal(t, http.StatusOK, post(sign("rotated-secret")))
}
