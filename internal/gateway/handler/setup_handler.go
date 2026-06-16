package handler

import (
	"encoding/json"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// SetupHandler exposes the first-run admin setup endpoints. These are public
// (no auth) and self-lock once the first administrator exists.
type SetupHandler struct {
	service *identity.Service
}

// NewSetupHandler creates a SetupHandler backed by the given identity service.
func NewSetupHandler(service *identity.Service) *SetupHandler {
	return &SetupHandler{service: service}
}

type createAdminRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Status handles GET /api/setup/status. It reports whether the first-run admin
// setup is still required.
func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	needed, err := h.service.SetupNeeded(r.Context())
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_setup": needed})
}

// CreateAdmin handles POST /api/setup/admin. It creates the first administrator
// and returns login tokens so the wizard can authenticate immediately. Once an
// admin exists it returns 409 (IDENTITY.SETUP_ALREADY_COMPLETED).
func (h *SetupHandler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	var req createAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Email == "" || req.Password == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("email and password are required"))
		return
	}

	result, err := h.service.CreateFirstAdmin(r.Context(), identity.CreateFirstAdminInput{
		Email:     req.Email,
		Password:  req.Password,
		IPAddress: extractIP(r),
		UserAgent: r.Header.Get("User-Agent"),
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"user":          userToResponse(result.User),
	})
}
