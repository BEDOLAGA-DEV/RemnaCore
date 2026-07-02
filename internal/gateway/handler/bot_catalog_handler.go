package handler

import (
	"context"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram"
)

// BotPluginLister lists the bot plugins selectable as a shop bot. It is
// satisfied by *telegram.BotPluginCatalog. The handler depends on this narrow
// port rather than the concrete catalog so it stays decoupled and testable.
type BotPluginLister interface {
	ListBotPlugins(ctx context.Context) ([]telegram.BotPluginInfo, error)
}

// BotCatalogHandler exposes the selectable bot-plugin catalog over HTTP.
type BotCatalogHandler struct {
	catalog BotPluginLister
}

// NewBotCatalogHandler constructs a BotCatalogHandler backed by the given
// bot-plugin catalog.
func NewBotCatalogHandler(catalog BotPluginLister) *BotCatalogHandler {
	return &BotCatalogHandler{catalog: catalog}
}

// ListBotPlugins writes the JSON array of selectable bot plugins.
func (h *BotCatalogHandler) ListBotPlugins(w http.ResponseWriter, r *http.Request) {
	plugins, err := h.catalog.ListBotPlugins(r.Context())
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plugins)
}
