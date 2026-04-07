package handler

import (
	"encoding/json"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/routing"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// RoutingHandler exposes the smart routing API.
type RoutingHandler struct {
	router *routing.SmartRouter
}

// NewRoutingHandler creates a RoutingHandler backed by the given SmartRouter.
func NewRoutingHandler(router *routing.SmartRouter) *RoutingHandler {
	return &RoutingHandler{router: router}
}

// SelectNode handles POST /api/routing/select. It reads a RouteRequest from the
// body and returns the best node plus fallbacks.
func (h *RoutingHandler) SelectNode(w http.ResponseWriter, r *http.Request) {
	var req routing.RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	resp, err := h.router.SelectNode(r.Context(), req)
	if err != nil {
		writeAPIError(w, apierror.RoutingNoNodes)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
