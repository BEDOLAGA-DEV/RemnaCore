package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// ErrShopBotNotConfigured is the port-level sentinel meaning "this shop has no
// bot row". The resolver adapter translates the reseller domain's not-found
// error into this so the handler can distinguish a client condition (unknown
// shop → 401) from an infrastructure failure (→ 500) without importing the
// reseller service context (gateway single-context architecture rule).
var ErrShopBotNotConfigured = errors.New("shop bot not configured")

// ShopBotConfigResolver resolves a shop's Telegram bot configuration by tenant.
// A missing configuration is reported as ErrShopBotNotConfigured; any other
// error is treated as infrastructure failure. The handler depends on this
// narrow port (returning only the reseller vo type) instead of importing the
// reseller service package directly, so it stays within a single domain
// service context per the gateway single-context architecture rule.
type ShopBotConfigResolver interface {
	GetShopBot(ctx context.Context, tenantID string) (*vo.ShopBotConfig, error)
}

// TelegramAuthHandler handles Telegram Mini App (WebApp) authentication. It
// resolves the shop's bot token via a ShopBotConfigResolver and delegates HMAC
// validation + session issuance to the identity service.
type TelegramAuthHandler struct {
	identity *identity.Service
	shopBots ShopBotConfigResolver
	logger   *slog.Logger
}

// NewTelegramAuthHandler creates a TelegramAuthHandler backed by the identity
// service and a shop-bot-config resolver.
func NewTelegramAuthHandler(identitySvc *identity.Service, shopBots ShopBotConfigResolver, logger *slog.Logger) *TelegramAuthHandler {
	return &TelegramAuthHandler{
		identity: identitySvc,
		shopBots: shopBots,
		logger:   logger.With(slog.String("component", "telegram_auth_handler")),
	}
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
	if err != nil {
		// A genuine not-found is a client condition (unknown shop) → 401; any
		// other error (DB down, decryption failure) is infrastructure → 500.
		// Collapsing both to 401 (the old behavior) hid real outages.
		if errors.Is(err, ErrShopBotNotConfigured) {
			h.logger.Info("telegram webapp login: shop bot not found", slog.String("shop_id", shopID))
			writeAPIError(w, apierror.Unauthorized.WithDetails("telegram authentication unavailable for this shop"))
			return
		}
		h.logger.Error("telegram webapp login: shop bot lookup failed",
			slog.String("shop_id", shopID), slog.Any("error", err))
		writeAPIError(w, apierror.Internal)
		return
	}
	if cfg == nil || !cfg.Enabled {
		h.logger.Info("telegram webapp login: shop bot missing or disabled", slog.String("shop_id", shopID))
		writeAPIError(w, apierror.Unauthorized.WithDetails("telegram authentication unavailable for this shop"))
		return
	}

	result, err := h.identity.LoginViaTelegramWebApp(
		r.Context(), req.InitData, cfg.Token.Expose(), shopID, extractIP(r), r.UserAgent(),
	)
	if err != nil {
		h.logger.Info("telegram webapp login: invalid session",
			slog.String("shop_id", shopID), slog.Any("error", err))
		writeAPIError(w, apierror.Unauthorized.WithDetails("invalid telegram session"))
		return
	}

	writeJSON(w, http.StatusOK, loginResultResponse(result))
}
