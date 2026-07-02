package gateway

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	// GzipCompressionLevel is the standard gzip compression level (1=fastest, 9=best).
	GzipCompressionLevel = 5

	// CORSMaxAge is the maximum cache duration (in seconds) for preflight
	// responses sent to browsers.
	CORSMaxAge = 300
)

// defaultCORSOrigins is used when no CORS_ALLOWED_ORIGINS env var is set.
// Only localhost variants are allowed by default; production origins MUST be
// set via the CORS_ALLOWED_ORIGINS environment variable.
var defaultCORSOrigins = []string{
	"http://localhost:*",
	"http://127.0.0.1:*",
}

// RouterParams groups the dependencies required to build the HTTP router.
type RouterParams struct {
	fx.In

	Config                *config.Config
	JWT                   *authutil.JWTIssuer
	AccessService         *service.AccessService
	RateLimiter           middleware.RateLimiter
	AuthRateLimiters      *middleware.AuthRateLimiters
	IdentityHandler       *handler.IdentityHandler
	SetupHandler          *handler.SetupHandler
	HealthHandler         *handler.HealthHandler
	WebhookHandler        http.Handler `name:"remnawave_webhook"`
	BillingHandler        *handler.BillingHandler
	MultiSubHandler       *handler.MultiSubHandler
	PluginHandler         *handler.PluginHandler
	CheckoutHandler       *handler.CheckoutHandler
	PaymentWebhookHandler *handler.PaymentWebhookHandler
	FamilyHandler         *handler.FamilyHandler
	AdminHandler          *handler.AdminHandler
	ResellerHandler       *handler.ResellerHandler
	ResellerService       *reseller.ResellerService
	RoutingHandler        *handler.RoutingHandler
	SettingsHandler       *handler.SettingsHandler
	StatsHandler          *handler.StatsHandler
	ActivityHandler       *handler.ActivityHandler
	IdentityAdminHandler  *handler.IAMHandler
	PluginRPCHandler      *handler.PluginRPCHandler
	PluginRouteHandler    *handler.PluginRouteHandler
	BuiltinRouteRegistry  *BuiltinRouteRegistry
	PluginRepo            plugin.PluginRepository
	BotManager            *telegram.BotManager
	TelegramAuthHandler   *handler.TelegramAuthHandler
	BotCatalogHandler     *handler.BotCatalogHandler
}

// NewRouter creates and returns a fully-configured chi router with all
// middleware and route registrations.
func NewRouter(p RouterParams) http.Handler {
	r := chi.NewRouter()

	// CORS — must be the very first middleware so preflight requests are
	// handled before any auth / rate-limit checks.
	origins := p.Config.CORS.AllowedOrigins
	if len(origins) == 0 {
		origins = defaultCORSOrigins
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: origins,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowedHeaders: []string{
			httpconst.HeaderAuthorization,
			httpconst.HeaderContentType,
			httpconst.HeaderRequestID,
			httpconst.HeaderAPIKey,
			httpconst.HeaderShopID,
		},
		ExposedHeaders:   []string{httpconst.HeaderRequestID},
		AllowCredentials: true,
		MaxAge:           CORSMaxAge,
	}))

	// OpenTelemetry HTTP instrumentation — creates spans for every inbound
	// request with method, path, and status code attributes.
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, observability.ServiceName)
	})

	// Global middleware stack.
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.MaxBodySize(middleware.DefaultMaxBodyBytes))
	r.Use(middleware.RequestLogger)
	r.Use(chimiddleware.Compress(GzipCompressionLevel))
	r.Use(middleware.RateLimit(p.RateLimiter))

	// Infrastructure endpoints — no tenant resolution needed.
	r.Get("/healthz", p.HealthHandler.Healthz)
	r.Get("/readyz", p.HealthHandler.Readyz)
	r.Handle("/metrics", promhttp.Handler())

	// Per-endpoint auth rate limit middleware.
	loginRL := middleware.AuthRateLimit(
		p.AuthRateLimiters.Login,
		p.AuthRateLimiters.LoginCfg,
		"login",
	)
	forgotPwdRL := middleware.AuthRateLimit(
		p.AuthRateLimiters.ForgotPwd,
		p.AuthRateLimiters.ForgotPwdCfg,
		"forgot_password",
	)
	// First-run admin setup reuses the login rate limiter (same sensitivity);
	// the endpoint self-locks once the first admin exists.
	setupRL := middleware.AuthRateLimit(
		p.AuthRateLimiters.Login,
		p.AuthRateLimiters.LoginCfg,
		"setup",
	)
	acceptInviteRL := middleware.AuthRateLimit(
		p.AuthRateLimiters.AcceptInvitation,
		p.AuthRateLimiters.AcceptInvitationCfg,
		"accept_invitation",
	)

	// API routes — tenant middleware scoped to API only (not health/metrics).
	r.Route("/api", func(api chi.Router) {
		api.Use(middleware.TenantResolver(p.ResellerService))
		api.Use(middleware.TenantRLS)
		// First-run admin setup (public; self-locks after the first admin).
		api.Get("/setup/status", p.SetupHandler.Status)
		api.With(setupRL).Post("/setup/admin", p.SetupHandler.CreateAdmin)

		// Public auth endpoints.
		api.Post("/auth/register", p.IdentityHandler.Register)
		api.With(loginRL).Post("/auth/login", p.IdentityHandler.Login)
		api.Post("/auth/verify-email", p.IdentityHandler.VerifyEmail)
		api.Post("/auth/refresh", p.IdentityHandler.RefreshToken)

		// Public webhook endpoints (authenticated by their respective handlers).
		api.Post("/webhooks/remnawave", p.WebhookHandler.ServeHTTP)
		api.Post("/webhooks/payment/{provider}", p.PaymentWebhookHandler.HandlePaymentWebhook)
		api.Post("/webhooks/telegram", p.BotManager.PlatformWebhookHandler())
		api.Post("/webhooks/telegram/{tenantID}", p.BotManager.ShopWebhookHandler())

		// Public plans — accessible without authentication for landing pages.
		api.Get("/plans", p.BillingHandler.GetPlans)
		api.Get("/plans/{planID}", p.BillingHandler.GetPlan)

		// Public tariff routes are registered dynamically by the
		// tariff-manager built-in plugin via RegisterPluginRoutes.

		// Public Telegram Mini App auth (rate-limited as a login equivalent).
		api.With(loginRL).Post("/auth/telegram/webapp", p.TelegramAuthHandler.WebAppLogin)

		// Public password reset endpoints.
		api.With(forgotPwdRL).Post("/auth/forgot-password", p.IdentityHandler.ForgotPassword)
		api.Post("/auth/reset-password", p.IdentityHandler.ResetPassword)

		// Public invitation acceptance — rate-limited to prevent token brute-force.
		api.With(acceptInviteRL).Post("/auth/accept-invitation", p.IdentityAdminHandler.AcceptInvitation)

		// Protected endpoints — require valid JWT.
		api.Group(func(protected chi.Router) {
			protected.Use(middleware.Auth(p.JWT))
			protected.Use(middleware.ShopResolver(p.AccessService))

			// Identity / Profile
			protected.Get("/me", p.IdentityHandler.Me)
			protected.Put("/me", p.IdentityHandler.UpdateProfile)
			protected.Post("/me/link-telegram", p.IdentityHandler.LinkTelegram)
			protected.Delete("/me/link-telegram", p.IdentityHandler.UnlinkTelegram)

			// IAM — invitation management, user creation, role assignment.
			protected.With(perm(p.AccessService, rbac.UsersInvite)).Post("/users/invitations", p.IdentityAdminHandler.CreateInvitation)
			protected.With(perm(p.AccessService, rbac.UsersInvite)).Get("/users/invitations", p.IdentityAdminHandler.ListInvitations)
			protected.With(perm(p.AccessService, rbac.UsersInvite)).Delete("/users/invitations/{id}", p.IdentityAdminHandler.RevokeInvitation)
			protected.With(perm(p.AccessService, rbac.UsersInvite)).Post("/users", p.IdentityAdminHandler.CreateUserDirect)
			protected.With(perm(p.AccessService, rbac.UsersAssignRole)).Post("/users/{userID}/roles", p.IdentityAdminHandler.AssignRole)
			protected.With(perm(p.AccessService, rbac.UsersAssignRole)).Delete("/users/{userID}/roles", p.IdentityAdminHandler.RevokeRole)
			protected.With(perm(p.AccessService, rbac.UsersRead)).Get("/users/{userID}/roles", p.IdentityAdminHandler.ListUserRoles)

			// Subscriptions
			protected.Post("/subscriptions", p.BillingHandler.CreateSubscription)
			protected.Get("/subscriptions", p.BillingHandler.GetMySubscriptions)
			protected.Get("/subscriptions/{subID}", p.BillingHandler.GetSubscription)
			protected.Post("/subscriptions/{subID}/cancel", p.BillingHandler.CancelSubscription)
			protected.Get("/subscriptions/{subID}/bindings", p.MultiSubHandler.GetBindingsBySubscription)
			protected.Post("/subscriptions/{subID}/addons", p.BillingHandler.AddSubscriptionAddon)
			protected.Delete("/subscriptions/{subID}/addons/{addonID}", p.BillingHandler.RemoveSubscriptionAddon)

			// Checkout
			protected.Post("/checkout", p.CheckoutHandler.StartCheckout)

			// Invoices
			protected.Get("/invoices", p.BillingHandler.GetInvoices)
			protected.Post("/invoices/{invoiceID}/pay", p.BillingHandler.PayInvoice)

			// Bindings (read-only)
			protected.Get("/bindings", p.MultiSubHandler.GetMyBindings)

			// Family
			protected.Post("/family", p.FamilyHandler.CreateFamily)
			protected.Get("/family", p.FamilyHandler.GetMyFamily)
			protected.Post("/family/members", p.FamilyHandler.AddMember)
			protected.Delete("/family/members/{userID}", p.FamilyHandler.RemoveMember)

			// Plugin RPC — call WASM plugin functions via HTTP.
			protected.Post("/plugins/{pluginSlug}/rpc/{function}", p.PluginRPCHandler.CallFunction)

			// Plugin Collections — structured JSON document storage for plugins.
			protected.Route("/plugins/{pluginSlug}/collections/{collection}", func(cr chi.Router) {
				cr.Get("/", p.PluginRPCHandler.ListDocuments)
				cr.Post("/", p.PluginRPCHandler.InsertDocument)
				cr.Get("/{docID}", p.PluginRPCHandler.GetDocument)
				cr.Put("/{docID}", p.PluginRPCHandler.UpdateDocument)
				cr.Delete("/{docID}", p.PluginRPCHandler.DeleteDocument)
			})

			// Admin routes — each gated per-route on a specific permission via
			// RequirePermission (ShopResolver already ran at the protected-group
			// level, so the active tenant is resolved before these run).
			protected.Route("/admin", func(admin chi.Router) {
				// Plugin management — install/enable/configure/remove require manage.
				admin.With(perm(p.AccessService, rbac.PluginsManage)).Get("/plugins", p.PluginHandler.ListPlugins)
				admin.With(perm(p.AccessService, rbac.PluginsManage)).Post("/plugins", p.PluginHandler.InstallPlugin)
				admin.With(perm(p.AccessService, rbac.PluginsManage)).Get("/plugins/{pluginID}", p.PluginHandler.GetPlugin)
				admin.With(perm(p.AccessService, rbac.PluginsManage)).Post("/plugins/{pluginID}/enable", p.PluginHandler.EnablePlugin)
				admin.With(perm(p.AccessService, rbac.PluginsManage)).Post("/plugins/{pluginID}/disable", p.PluginHandler.DisablePlugin)
				admin.With(perm(p.AccessService, rbac.PluginsManage)).Delete("/plugins/{pluginID}", p.PluginHandler.UninstallPlugin)
				admin.With(perm(p.AccessService, rbac.PluginsManage)).Put("/plugins/{pluginID}/config", p.PluginHandler.UpdatePluginConfig)
				admin.With(perm(p.AccessService, rbac.PluginsManage)).Put("/plugins/{pluginID}/reload", p.PluginHandler.HotReloadPlugin)
				// Plugin-pages metadata: lighter read perm so shop roles can render pages.
				admin.With(perm(p.AccessService, rbac.PluginsRead)).Get("/plugin-pages", p.PluginHandler.ListPluginPages)

				// User / subscription / invoice management
				admin.With(perm(p.AccessService, rbac.UsersRead)).Get("/users", p.AdminHandler.ListUsers)
				admin.With(perm(p.AccessService, rbac.UsersRead)).Get("/users/{userID}", p.AdminHandler.GetUser)
				// Platform aggregate lists — gated platform-only (AnalyticsRead).
				// SubscriptionsRead/BillingRead are now shop-scoped; shop owners get
				// their own subscriptions/invoices via the tenant-scoped C4 endpoints.
				admin.With(perm(p.AccessService, rbac.AnalyticsRead)).Get("/subscriptions", p.AdminHandler.ListSubscriptions)
				admin.With(perm(p.AccessService, rbac.AnalyticsRead)).Get("/invoices", p.AdminHandler.ListInvoices)

				// System settings
				admin.With(perm(p.AccessService, rbac.SettingsManage)).Get("/settings", p.SettingsHandler.GetSettings)
				admin.With(perm(p.AccessService, rbac.SettingsManage)).Put("/settings", p.SettingsHandler.UpdateSettings)

				// Dashboard stats / observability
				admin.With(perm(p.AccessService, rbac.AnalyticsRead)).Get("/stats", p.StatsHandler.GetStats)
				admin.With(perm(p.AccessService, rbac.AnalyticsRead)).Get("/metrics", p.StatsHandler.GetMetrics)
				admin.With(perm(p.AccessService, rbac.AnalyticsRead)).Get("/metrics/history", p.StatsHandler.GetMetricsHistory)
				admin.With(perm(p.AccessService, rbac.SessionsRead)).Get("/sessions", p.StatsHandler.ListSessions)
				admin.With(perm(p.AccessService, rbac.SessionsRead)).Get("/activity", p.ActivityHandler.ListActivity)

				// Shop provisioning — create a new shop with owner + API key.
				admin.With(perm(p.AccessService, rbac.ShopsManage)).Post("/shops", p.IdentityAdminHandler.CreateShop)

				// Tenant (shop) management
				admin.With(perm(p.AccessService, rbac.ShopsManage)).Post("/tenants", p.ResellerHandler.CreateTenant)
				admin.With(perm(p.AccessService, rbac.ShopsRead)).Get("/tenants", p.ResellerHandler.ListTenants)
				admin.With(perm(p.AccessService, rbac.ShopsRead)).Get("/tenants/{tenantID}", p.ResellerHandler.GetTenant)
				admin.With(perm(p.AccessService, rbac.ShopsBranding)).Put("/tenants/{tenantID}/branding", p.ResellerHandler.UpdateBranding)
				admin.With(perm(p.AccessService, rbac.ShopsRead)).Get("/tenants/{tenantID}/bot", p.ResellerHandler.GetTenantBot)
				admin.With(perm(p.AccessService, rbac.ShopsManage)).Put("/tenants/{tenantID}/bot", p.ResellerHandler.UpdateTenantBot)

				// Selectable bot-plugin catalog (built-in + enabled WASM bots).
				admin.With(perm(p.AccessService, rbac.ShopsRead)).Get("/bot-plugins", p.BotCatalogHandler.ListBotPlugins)

				// Tariff management routes are registered dynamically by the
				// tariff-manager built-in plugin via RegisterPluginRoutes.
			})

			// Routing
			protected.Post("/routing/select", p.RoutingHandler.SelectNode)

			// Reseller self-service endpoints — gated per-route on shop-scoped
			// permissions (ShopResolver already applied at the protected-group
			// level, so the active tenant from X-Shop-Id is present here).
			protected.Route("/reseller", func(resellerRouter chi.Router) {
				resellerRouter.With(perm(p.AccessService, rbac.DashboardRead)).Get("/dashboard", p.ResellerHandler.Dashboard)
				resellerRouter.With(perm(p.AccessService, rbac.BillingRead)).Get("/commissions", p.ResellerHandler.Commissions)
				resellerRouter.With(perm(p.AccessService, rbac.CustomersRead)).Get("/customers", p.ResellerHandler.Customers)
				resellerRouter.With(perm(p.AccessService, rbac.ShopSettingsManage)).Put("/bot", p.ResellerHandler.SetResellerBot)
				resellerRouter.With(perm(p.AccessService, rbac.ShopSettingsManage)).Get("/bot", p.ResellerHandler.GetResellerBot)
				resellerRouter.With(perm(p.AccessService, rbac.ShopSettingsManage)).Get("/bot/plugins", p.BotCatalogHandler.ListBotPlugins)
			})
		})
	})

	// Dynamic plugin routes — registered from enabled plugin manifests.
	logger := slog.Default()
	RegisterPluginRoutes(r, p.PluginRepo, p.PluginRouteHandler, p.BuiltinRouteRegistry, p.JWT, p.AccessService, logger)

	return r
}

// perm returns RequirePermission middleware bound to the given AccessService.
func perm(access *service.AccessService, required rbac.Permission) func(http.Handler) http.Handler {
	return middleware.RequirePermission(access, required)
}
