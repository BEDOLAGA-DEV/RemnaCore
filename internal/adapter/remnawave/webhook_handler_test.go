package remnawave

import (
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
	handler := NewWebhookHandler(secret, func(p WebhookPayload) {
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
	handler := NewWebhookHandler("correct-secret", func(p WebhookPayload) {
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
	handler := NewWebhookHandler("secret", func(p WebhookPayload) {
		t.Fatal("callback should not be invoked for empty body")
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(""))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
