package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// maxRPCBodyBytes is the maximum request body size for RPC and collection
// document payloads (1 MB).
const maxRPCBodyBytes int64 = 1 << 20

// PluginRPCHandler handles plugin RPC calls and collection CRUD endpoints.
type PluginRPCHandler struct {
	runtime     *plugin.RuntimePool
	pluginRepo  plugin.PluginRepository
	collections pluginstore.Store
	logger      *slog.Logger
}

// NewPluginRPCHandler creates a PluginRPCHandler.
func NewPluginRPCHandler(
	runtime *plugin.RuntimePool,
	pluginRepo plugin.PluginRepository,
	collections pluginstore.Store,
	logger *slog.Logger,
) *PluginRPCHandler {
	return &PluginRPCHandler{
		runtime:     runtime,
		pluginRepo:  pluginRepo,
		collections: collections,
		logger:      logger,
	}
}

// verifyPluginEnabled looks up a plugin by slug and returns an API error if it
// does not exist or is not enabled. Returns the plugin on success.
func (h *PluginRPCHandler) verifyPluginEnabled(w http.ResponseWriter, r *http.Request) (*plugin.Plugin, bool) {
	slugParam := chi.URLParam(r, "pluginSlug")
	if slugParam == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plugin slug is required"))
		return nil, false
	}

	p, err := h.pluginRepo.GetBySlug(r.Context(), slugParam)
	if err != nil {
		if errors.Is(err, plugin.ErrPluginNotFound) {
			writeAPIError(w, apierror.PluginNotFound)
			return nil, false
		}
		writeAPIError(w, apierror.Internal)
		return nil, false
	}

	if p.Status != plugin.StatusEnabled {
		writeAPIError(w, apierror.PluginNotEnabled)
		return nil, false
	}

	return p, true
}

// --- RPC ---

// CallFunction handles POST /api/plugins/{pluginSlug}/rpc/{function}.
// It dispatches the call to a WASM plugin function and returns the result.
func (h *PluginRPCHandler) CallFunction(w http.ResponseWriter, r *http.Request) {
	p, ok := h.verifyPluginEnabled(w, r)
	if !ok {
		return
	}

	funcName := chi.URLParam(r, "function")
	if funcName == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("function name is required"))
		return
	}

	// Built-in plugins have no WASM runtime — RPC is not yet supported.
	if p.IsBuiltIn {
		writeError(w, http.StatusNotImplemented, "RPC not supported for built-in plugins")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRPCBodyBytes+1))
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if int64(len(body)) > maxRPCBodyBytes {
		writeAPIError(w, apierror.BodyTooLarge)
		return
	}

	output, err := h.runtime.CallHook(r.Context(), p.Slug, funcName, body)
	if err != nil {
		h.logger.Warn("plugin RPC call failed",
			slog.String("slug", p.Slug),
			slog.String("function", funcName),
			slog.Any("error", err),
		)
		writeErrorFromDomain(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	// Write raw output — the plugin is responsible for valid JSON.
	_, _ = w.Write(output)
}

// --- Collections ---

// ListDocuments handles GET /api/plugins/{pluginSlug}/collections/{collection}.
func (h *PluginRPCHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	_, ok := h.verifyPluginEnabled(w, r)
	if !ok {
		return
	}

	pluginSlug := chi.URLParam(r, "pluginSlug")
	collection := chi.URLParam(r, "collection")

	docs, err := h.collections.ListDocuments(r.Context(), pluginSlug, collection)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}

	writeJSON(w, http.StatusOK, docs)
}

// InsertDocument handles POST /api/plugins/{pluginSlug}/collections/{collection}.
func (h *PluginRPCHandler) InsertDocument(w http.ResponseWriter, r *http.Request) {
	_, ok := h.verifyPluginEnabled(w, r)
	if !ok {
		return
	}

	pluginSlug := chi.URLParam(r, "pluginSlug")
	collection := chi.URLParam(r, "collection")

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRPCBodyBytes+1))
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if int64(len(body)) > maxRPCBodyBytes {
		writeAPIError(w, apierror.BodyTooLarge)
		return
	}

	if !json.Valid(body) {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("request body must be valid JSON"))
		return
	}

	doc, err := h.collections.InsertDocument(r.Context(), pluginSlug, collection, json.RawMessage(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to insert document")
		return
	}

	writeJSON(w, http.StatusCreated, doc)
}

// GetDocument handles GET /api/plugins/{pluginSlug}/collections/{collection}/{docID}.
func (h *PluginRPCHandler) GetDocument(w http.ResponseWriter, r *http.Request) {
	_, ok := h.verifyPluginEnabled(w, r)
	if !ok {
		return
	}

	pluginSlug := chi.URLParam(r, "pluginSlug")
	collection := chi.URLParam(r, "collection")
	docID := chi.URLParam(r, "docID")

	doc, err := h.collections.GetDocument(r.Context(), pluginSlug, collection, docID)
	if err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get document")
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

// UpdateDocument handles PUT /api/plugins/{pluginSlug}/collections/{collection}/{docID}.
func (h *PluginRPCHandler) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	_, ok := h.verifyPluginEnabled(w, r)
	if !ok {
		return
	}

	pluginSlug := chi.URLParam(r, "pluginSlug")
	collection := chi.URLParam(r, "collection")
	docID := chi.URLParam(r, "docID")

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRPCBodyBytes+1))
	if err != nil {
		writeAPIError(w, apierror.Internal)
		return
	}
	if int64(len(body)) > maxRPCBodyBytes {
		writeAPIError(w, apierror.BodyTooLarge)
		return
	}

	if !json.Valid(body) {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("request body must be valid JSON"))
		return
	}

	if err := h.collections.UpdateDocument(r.Context(), pluginSlug, collection, docID, json.RawMessage(body)); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update document")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteDocument handles DELETE /api/plugins/{pluginSlug}/collections/{collection}/{docID}.
func (h *PluginRPCHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	_, ok := h.verifyPluginEnabled(w, r)
	if !ok {
		return
	}

	pluginSlug := chi.URLParam(r, "pluginSlug")
	collection := chi.URLParam(r, "collection")
	docID := chi.URLParam(r, "docID")

	if err := h.collections.DeleteDocument(r.Context(), pluginSlug, collection, docID); err != nil {
		if errors.Is(err, pluginstore.ErrDocumentNotFound) {
			writeAPIError(w, apierror.NotFound)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete document")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
