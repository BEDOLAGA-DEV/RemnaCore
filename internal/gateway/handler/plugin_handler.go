package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// PluginHandler exposes admin-only HTTP endpoints for managing plugins.
type PluginHandler struct {
	lifecycle *plugin.LifecycleManager
	repo      plugin.PluginRepository
}

// NewPluginHandler creates a PluginHandler backed by the lifecycle manager and
// plugin repository.
func NewPluginHandler(lifecycle *plugin.LifecycleManager, repo plugin.PluginRepository) *PluginHandler {
	return &PluginHandler{
		lifecycle: lifecycle,
		repo:      repo,
	}
}

// --- Request DTOs ---

type installPluginRequest struct {
	Manifest string `json:"manifest"` // base64-encoded or plain TOML
	WASM     []byte `json:"wasm"`     // base64-encoded WASM bytes
}

type updatePluginConfigRequest struct {
	Config map[string]string `json:"config"`
}

// --- Handlers ---

// ListPlugins handles GET /api/admin/plugins.
func (h *PluginHandler) ListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins, err := h.repo.GetAll(r.Context())
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, plugins)
}

// GetPlugin handles GET /api/admin/plugins/{pluginID}.
func (h *PluginHandler) GetPlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	if pluginID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plugin ID is required"))
		return
	}

	p, err := h.repo.GetByID(r.Context(), pluginID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, p)
}

// InstallPlugin handles POST /api/admin/plugins.
func (h *PluginHandler) InstallPlugin(w http.ResponseWriter, r *http.Request) {
	var req installPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Manifest == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("manifest is required"))
		return
	}

	// The CLI sends the manifest as base64-encoded TOML. Attempt to decode;
	// if it fails, treat the string as plain TOML.
	manifestBytes := []byte(req.Manifest)
	if decoded, decErr := base64.StdEncoding.DecodeString(req.Manifest); decErr == nil {
		manifestBytes = decoded
	}

	p, err := h.lifecycle.Install(r.Context(), manifestBytes, req.WASM)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, p)
}

// EnablePlugin handles POST /api/admin/plugins/{pluginID}/enable.
func (h *PluginHandler) EnablePlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	if pluginID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plugin ID is required"))
		return
	}

	if err := h.lifecycle.Enable(r.Context(), pluginID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

// DisablePlugin handles POST /api/admin/plugins/{pluginID}/disable.
func (h *PluginHandler) DisablePlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	if pluginID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plugin ID is required"))
		return
	}

	if err := h.lifecycle.Disable(r.Context(), pluginID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

// UninstallPlugin handles DELETE /api/admin/plugins/{pluginID}.
func (h *PluginHandler) UninstallPlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	if pluginID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plugin ID is required"))
		return
	}

	if err := h.lifecycle.Uninstall(r.Context(), pluginID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "uninstalled"})
}

// HotReloadPlugin handles PUT /api/admin/plugins/{pluginID}/reload.
// It atomically replaces a running plugin with a new version while preserving
// existing configuration and ensuring zero hook downtime.
func (h *PluginHandler) HotReloadPlugin(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	if pluginID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plugin ID is required"))
		return
	}

	var req installPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Manifest == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("manifest is required"))
		return
	}

	// The CLI sends the manifest as base64-encoded TOML. Attempt to decode;
	// if it fails, treat the string as plain TOML.
	manifestBytes := []byte(req.Manifest)
	if decoded, decErr := base64.StdEncoding.DecodeString(req.Manifest); decErr == nil {
		manifestBytes = decoded
	}

	if err := h.lifecycle.HotReload(r.Context(), pluginID, manifestBytes, req.WASM); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "hot_reloaded"})
}

// UpdatePluginConfig handles PUT /api/admin/plugins/{pluginID}/config.
func (h *PluginHandler) UpdatePluginConfig(w http.ResponseWriter, r *http.Request) {
	pluginID := chi.URLParam(r, "pluginID")
	if pluginID == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("plugin ID is required"))
		return
	}

	var req updatePluginConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if err := h.lifecycle.UpdateConfig(r.Context(), pluginID, req.Config); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "config_updated"})
}
