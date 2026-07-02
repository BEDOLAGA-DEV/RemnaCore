package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram"
)

// stubBotCatalog is a BotPluginLister stub for handler-level tests.
type stubBotCatalog struct {
	plugins []telegram.BotPluginInfo
	err     error
}

func (s *stubBotCatalog) ListBotPlugins(_ context.Context) ([]telegram.BotPluginInfo, error) {
	return s.plugins, s.err
}

// TestBotCatalogHandler_ListBotPlugins_OK verifies a successful list returns 200
// and the expected JSON array shape.
func TestBotCatalogHandler_ListBotPlugins_OK(t *testing.T) {
	catalog := &stubBotCatalog{plugins: []telegram.BotPluginInfo{
		{Slug: "cabinet-bot", Name: "Cabinet Bot", Kind: telegram.BotPluginKindBuiltin},
		{Slug: "wasm-bot", Name: "WASM Bot", Kind: telegram.BotPluginKindWASM, Description: "demo"},
	}}
	h := NewBotCatalogHandler(catalog)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bot-plugins", nil)
	h.ListBotPlugins(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var got []telegram.BotPluginInfo
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.Equal(t, "cabinet-bot", got[0].Slug)
	assert.Equal(t, telegram.BotPluginKindBuiltin, got[0].Kind)
	assert.Equal(t, "wasm-bot", got[1].Slug)
	assert.Equal(t, telegram.BotPluginKindWASM, got[1].Kind)
}

// TestBotCatalogHandler_ListBotPlugins_Error verifies a catalog error yields a
// 5xx response.
func TestBotCatalogHandler_ListBotPlugins_Error(t *testing.T) {
	catalog := &stubBotCatalog{err: errors.New("catalog down")}
	h := NewBotCatalogHandler(catalog)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bot-plugins", nil)
	h.ListBotPlugins(rec, req)

	assert.GreaterOrEqual(t, rec.Code, http.StatusInternalServerError)
}
