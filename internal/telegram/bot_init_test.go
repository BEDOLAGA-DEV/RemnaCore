package telegram

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBot_Init_EmptyToken_WebhookUnavailable(t *testing.T) {
	b := newShopBotForTest("", "", "", "tenant-1", "https://c")
	require.NoError(t, b.Init(context.Background()))
	rec := httptest.NewRecorder()
	b.WebhookHandler()(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestBot_Init_WithToken_BuildsBot(t *testing.T) {
	b := newShopBotForTest("123:fake-offline-token", "", "secret", "tenant-1", "https://c")
	require.NoError(t, b.Init(context.Background()))
	require.NotNil(t, b.bot)
	rec := httptest.NewRecorder()
	// No secret-token header → the lib's WebhookHandler rejects (not 503: bot exists).
	b.WebhookHandler()(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	assert.NotEqual(t, http.StatusServiceUnavailable, rec.Code)
}

// newShopBotForTest builds a shop bot with minimal deps for tests that only
// need Init/WebhookHandler behaviour and do not dispatch updates.
func newShopBotForTest(token, webhookURL, webhookSecret, tenantID, cabinetURL string) *Bot {
	reg := NewBuiltinBotRegistry()
	return NewShopBot(token, webhookURL, webhookSecret, tenantID, cabinetURL, nil, "", reg, botHostRegistryForTest(), nil, nil, testLogger())
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
