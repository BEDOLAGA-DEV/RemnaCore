package handler

import (
	"encoding/json"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// IdentityHandler exposes HTTP endpoints for user registration, login, email
// verification, token refresh, and profile retrieval.
type IdentityHandler struct {
	service *identity.Service
}

// NewIdentityHandler creates an IdentityHandler backed by the given identity
// service.
func NewIdentityHandler(service *identity.Service) *IdentityHandler {
	return &IdentityHandler{service: service}
}

// --- Request DTOs ---

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// --- Handlers ---

// Register handles POST /api/auth/register.
func (h *IdentityHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Email == "" || req.Password == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("email and password are required"))
		return
	}

	result, err := h.service.Register(r.Context(), identity.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"user_id":            result.User.ID,
		"email":              result.User.Email,
		"verification_token": result.VerificationToken,
	})
}

// Login handles POST /api/auth/login.
func (h *IdentityHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Email == "" || req.Password == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("email and password are required"))
		return
	}

	result, err := h.service.Login(r.Context(), identity.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
		"user":          userToResponse(result.User),
	})
}

// VerifyEmail handles POST /api/auth/verify-email.
func (h *IdentityHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Token == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("token is required"))
		return
	}

	if err := h.service.VerifyEmail(r.Context(), req.Token); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

// RefreshToken handles POST /api/auth/refresh.
func (h *IdentityHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req refreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.RefreshToken == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("refresh_token is required"))
		return
	}

	result, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  result.AccessToken,
		"refresh_token": result.RefreshToken,
	})
}

// Me handles GET /api/me (protected).
func (h *IdentityHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	user, err := h.service.GetMe(r.Context(), claims.UserID)
	if err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(user))
}

// --- Cabinet Profile Endpoints ---

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
}

type linkTelegramRequest struct {
	TelegramID int64 `json:"telegram_id"`
}

// UpdateProfile handles PUT /api/me -- update display name.
func (h *IdentityHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if err := h.service.UpdateDisplayName(r.Context(), claims.UserID, req.DisplayName); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// LinkTelegram handles POST /api/me/link-telegram -- link a Telegram ID.
func (h *IdentityHandler) LinkTelegram(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	var req linkTelegramRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.TelegramID == 0 {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("telegram_id is required"))
		return
	}

	if err := h.service.LinkTelegram(r.Context(), claims.UserID, req.TelegramID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "linked"})
}

// UnlinkTelegram handles DELETE /api/me/link-telegram -- unlink Telegram.
func (h *IdentityHandler) UnlinkTelegram(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		writeAPIError(w, apierror.Unauthorized)
		return
	}

	if err := h.service.UnlinkTelegram(r.Context(), claims.UserID); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}

// --- Password Reset DTOs ---

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// ForgotPassword handles POST /api/auth/forgot-password.
func (h *IdentityHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Email == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("email is required"))
		return
	}

	if err := h.service.RequestPasswordReset(r.Context(), req.Email); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	// Always return success to prevent email enumeration.
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "if the email exists, a reset link has been sent",
	})
}

// ResetPassword handles POST /api/auth/reset-password.
func (h *IdentityHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("token and new_password are required"))
		return
	}

	if err := h.service.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		writeErrorFromDomain(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password reset successful"})
}

// userToResponse converts a PlatformUser to a JSON-friendly map.
func userToResponse(u *identity.PlatformUser) map[string]any {
	return map[string]any{
		"id":             u.ID,
		"email":          u.Email,
		"display_name":   u.DisplayName,
		"email_verified": u.EmailVerified,
		"role":           string(u.Role),
		"created_at":     u.CreatedAt,
		"updated_at":     u.UpdatedAt,
	}
}
