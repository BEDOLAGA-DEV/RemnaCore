package handler

import (
	"encoding/json"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// TelegramAuthHandler handles Telegram Mini App (WebApp) authentication.
// It composes the identity and reseller domains without coupling them:
// the handler resolves the shop's bot token via the reseller service and
// delegates HMAC validation + session issuance to the identity service.
type TelegramAuthHandler struct {
	identity *identity.Service
	reseller *reseller.ResellerService
}

// NewTelegramAuthHandler creates a TelegramAuthHandler backed by the given
// identity and reseller services.
func NewTelegramAuthHandler(identitySvc *identity.Service, resellerSvc *reseller.ResellerService) *TelegramAuthHandler {
	return &TelegramAuthHandler{identity: identitySvc, reseller: resellerSvc}
}

type telegramWebAppLoginRequest struct {
	ShopID   string `json:"shop_id"`
	InitData string `json:"init_data"`
}

// WebAppLogin handles POST /api/auth/telegram/webapp (public).
//
// Flow:
//  1. Decode and validate the request body.
//  2. Resolve the shop's bot token via reseller.GetShopBot under platform scope
//     (unauthenticated read; the GUC sentinel allows cross-tenant visibility).
//  3. Return 401 if the shop has no configured or disabled bot.
//  4. Call identity.LoginViaTelegramWebApp which verifies the HMAC and issues a session.
//  5. Return 401 on any identity error (no detail leak).
//  6. Return the standard login response (access_token, refresh_token, user).
//     The bot token is never included in the response.
func (h *TelegramAuthHandler) WebAppLogin(w http.ResponseWriter, r *http.Request) {
	var req telegramWebAppLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}
	if req.ShopID == "" || req.InitData == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("shop_id and init_data are required"))
		return
	}

	cfg, err := h.reseller.GetShopBot(tenantctx.WithPlatformScope(r.Context()), req.ShopID)
	if err != nil || cfg == nil || !cfg.Enabled {
		writeAPIError(w, apierror.Unauthorized.WithDetails("telegram authentication unavailable for this shop"))
		return
	}

	result, err := h.identity.LoginViaTelegramWebApp(
		r.Context(), req.InitData, cfg.Token.Expose(), req.ShopID, extractIP(r), r.UserAgent(),
	)
	if err != nil {
		writeAPIError(w, apierror.Unauthorized.WithDetails("invalid telegram session"))
		return
	}

	writeJSON(w, http.StatusOK, loginResultResponse(result))
}
