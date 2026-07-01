package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// ShopBotConfigResolver resolves a shop's Telegram bot configuration by tenant.
// It is satisfied by *reseller.ResellerService. The handler depends on this
// narrow port (returning only the reseller vo type) instead of importing the
// reseller service package directly, so it stays within a single domain
// context per the gateway single-context architecture rule.
type ShopBotConfigResolver interface {
	GetShopBot(ctx context.Context, tenantID string) (*vo.ShopBotConfig, error)
}

// TelegramAuthHandler handles Telegram Mini App (WebApp) authentication. It
// resolves the shop's bot token via a ShopBotConfigResolver and delegates HMAC
// validation + session issuance to the identity service.
type TelegramAuthHandler struct {
	identity *identity.Service
	shopBots ShopBotConfigResolver
}

// NewTelegramAuthHandler creates a TelegramAuthHandler backed by the identity
// service and a shop-bot-config resolver.
func NewTelegramAuthHandler(identitySvc *identity.Service, shopBots ShopBotConfigResolver) *TelegramAuthHandler {
	return &TelegramAuthHandler{identity: identitySvc, shopBots: shopBots}
}

type telegramWebAppLoginRequest struct {
	ShopID   string `json:"shop_id"`
	InitData string `json:"init_data"`
}

// WebAppLogin handles POST /api/auth/telegram/webapp (public).
//
// Flow:
//  1. Decode the request body; init_data is required.
//  2. Determine the shop: an explicit shop_id wins; otherwise fall back to the
//     tenant resolved from the request domain (TenantResolver). 401 if neither.
//  3. Resolve the shop's bot token via reseller.GetShopBot under platform scope
//     (unauthenticated read; the GUC sentinel allows cross-tenant visibility).
//  4. Return 401 if the shop has no configured or disabled bot.
//  5. Call identity.LoginViaTelegramWebApp which verifies the HMAC and issues a session.
//  6. Return 401 on any identity error (no detail leak).
//  7. Return the standard login response (access_token, refresh_token, user).
//     The bot token is never included in the response.
func (h *TelegramAuthHandler) WebAppLogin(w http.ResponseWriter, r *http.Request) {
	var req telegramWebAppLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidationError(w, err)
		return
	}
	if req.InitData == "" {
		writeAPIError(w, apierror.ValidationFailed.WithDetails("init_data is required"))
		return
	}

	// shop_id is optional: when omitted, fall back to the tenant resolved from
	// the request domain (TenantResolver runs on every /api route). This lets a
	// shop's cabinet — served on the shop's own domain — authenticate without
	// having to know its own tenant id. An explicit shop_id still takes priority.
	shopID := req.ShopID
	if shopID == "" {
		if tenant := middleware.GetTenant(r.Context()); tenant != nil {
			shopID = tenant.ID
		}
	}
	if shopID == "" {
		writeAPIError(w, apierror.Unauthorized.WithDetails("telegram authentication unavailable for this shop"))
		return
	}

	cfg, err := h.shopBots.GetShopBot(tenantctx.WithPlatformScope(r.Context()), shopID)
	if err != nil || cfg == nil || !cfg.Enabled {
		writeAPIError(w, apierror.Unauthorized.WithDetails("telegram authentication unavailable for this shop"))
		return
	}

	result, err := h.identity.LoginViaTelegramWebApp(
		r.Context(), req.InitData, cfg.Token.Expose(), shopID, extractIP(r), r.UserAgent(),
	)
	if err != nil {
		writeAPIError(w, apierror.Unauthorized.WithDetails("invalid telegram session"))
		return
	}

	writeJSON(w, http.StatusOK, loginResultResponse(result))
}
