package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

// PluginRouteHandler proxies HTTP requests declared in plugin manifests to the
// corresponding WASM RPC functions.
type PluginRouteHandler struct {
	runtime    *plugin.RuntimePool
	pluginRepo plugin.PluginRepository
	logger     *slog.Logger
}

// NewPluginRouteHandler creates a PluginRouteHandler.
func NewPluginRouteHandler(
	runtime *plugin.RuntimePool,
	pluginRepo plugin.PluginRepository,
	logger *slog.Logger,
) *PluginRouteHandler {
	return &PluginRouteHandler{
		runtime:    runtime,
		pluginRepo: pluginRepo,
		logger:     logger,
	}
}

// pluginRouteResponse is the structured envelope a plugin may return from a
// route handler to control status code and headers.
type pluginRouteResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// pluginRouteInput is the JSON envelope sent to the plugin function with
// request metadata.
type pluginRouteInput struct {
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      map[string]string `json:"query"`
	PathParams map[string]string `json:"path_params"`
	Headers    map[string]string `json:"headers"`
	Body       json.RawMessage   `json:"body,omitempty"`
}

// safeForwardHeaders lists the request headers forwarded to plugins. Auth
// tokens are deliberately excluded.
var safeForwardHeaders = []string{
	httpconst.HeaderContentType,
	"Accept",
	httpconst.HeaderRequestID,
}

// ProxyToPlugin creates an http.HandlerFunc that forwards the request to a
// plugin's RPC function and returns the plugin's response.
func (h *PluginRouteHandler) ProxyToPlugin(pluginSlug, functionName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Read request body (limited to 1 MB).
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRPCBodyBytes+1))
		if err != nil {
			writeAPIError(w, apierror.Internal)
			return
		}
		if int64(len(body)) > maxRPCBodyBytes {
			writeAPIError(w, apierror.BodyTooLarge)
			return
		}

		// 2. Build context JSON with request metadata.
		input := buildRouteInput(r, body)

		// 3. Call plugin RPC function.
		output, err := h.runtime.CallHook(r.Context(), pluginSlug, functionName, input)
		if err != nil {
			h.logger.Error("plugin route handler failed",
				slog.String("plugin", pluginSlug),
				slog.String("function", functionName),
				slog.Any("error", err),
			)
			writeAPIError(w, apierror.Internal)
			return
		}

		// 4. Parse plugin response and set status code + headers.
		// Plugin response format: { "status": 200, "headers": {...}, "body": ... }
		var resp pluginRouteResponse
		if err := json.Unmarshal(output, &resp); err != nil {
			// If plugin returns raw JSON (not wrapped), treat as 200 + JSON body.
			w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(output)
			return
		}

		// Set custom headers if provided.
		for k, v := range resp.Headers {
			w.Header().Set(k, v)
		}
		if w.Header().Get(httpconst.HeaderContentType) == "" {
			w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		}

		status := resp.Status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		if resp.Body != nil {
			_, _ = w.Write(resp.Body)
		}
	}
}

// buildRouteInput builds the JSON envelope sent to the plugin function.
func buildRouteInput(r *http.Request, body []byte) []byte {
	input := pluginRouteInput{
		Method:     r.Method,
		Path:       r.URL.Path,
		Query:      make(map[string]string),
		PathParams: make(map[string]string),
		Headers:    make(map[string]string),
	}

	// Copy query params (first value only).
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			input.Query[k] = v[0]
		}
	}

	// Copy chi URL params.
	rctx := chi.RouteContext(r.Context())
	if rctx != nil {
		for i, key := range rctx.URLParams.Keys {
			if i < len(rctx.URLParams.Values) {
				input.PathParams[key] = rctx.URLParams.Values[i]
			}
		}
	}

	// Copy safe headers (not auth tokens).
	for _, h := range safeForwardHeaders {
		if v := r.Header.Get(h); v != "" {
			input.Headers[h] = v
		}
	}

	if len(body) > 0 {
		input.Body = body
	}

	encoded, _ := json.Marshal(input)
	return encoded
}
