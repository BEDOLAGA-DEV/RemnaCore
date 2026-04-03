package gateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
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
	RateLimiter           valkey.RateLimiter
	IdentityHandler       *handler.IdentityHandler
	HealthHandler         *handler.HealthHandler
	WebhookHandler        *remnawave.WebhookHandler
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
	TelegramBot           *telegram.Bot
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
	r.Use(middleware.RequestLogger)
	r.Use(chimiddleware.Compress(GzipCompressionLevel))
	r.Use(middleware.RateLimit(p.RateLimiter))
	r.Use(middleware.TenantResolver(p.ResellerService))

	// Infrastructure endpoints.
	r.Get("/healthz", p.HealthHandler.Healthz)
	r.Get("/readyz", p.HealthHandler.Readyz)
	r.Handle("/metrics", promhttp.Handler())

	// API routes.
	r.Route("/api", func(api chi.Router) {
		// Public auth endpoints.
		api.Post("/auth/register", p.IdentityHandler.Register)
		api.Post("/auth/login", p.IdentityHandler.Login)
		api.Post("/auth/verify-email", p.IdentityHandler.VerifyEmail)
		api.Post("/auth/refresh", p.IdentityHandler.RefreshToken)

		// Public webhook endpoints (authenticated by their respective handlers).
		api.Post("/webhooks/remnawave", p.WebhookHandler.ServeHTTP)
		api.Post("/webhooks/payment/{provider}", p.PaymentWebhookHandler.HandlePaymentWebhook)
		api.Post("/webhooks/telegram", p.TelegramBot.WebhookHandler())

		// Public plans — accessible without authentication for landing pages.
		api.Get("/plans", p.BillingHandler.GetPlans)
		api.Get("/plans/{planID}", p.BillingHandler.GetPlan)

		// Public password reset endpoints.
		api.Post("/auth/forgot-password", p.IdentityHandler.ForgotPassword)
		api.Post("/auth/reset-password", p.IdentityHandler.ResetPassword)

		// Protected endpoints — require valid JWT.
		api.Group(func(protected chi.Router) {
			protected.Use(middleware.Auth(p.JWT))

			// Identity / Profile
			protected.Get("/me", p.IdentityHandler.Me)
			protected.Put("/me", p.IdentityHandler.UpdateProfile)
			protected.Post("/me/link-telegram", p.IdentityHandler.LinkTelegram)
			protected.Delete("/me/link-telegram", p.IdentityHandler.UnlinkTelegram)

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

			// Admin routes — require admin role (enforced by middleware).
			protected.Route("/admin", func(admin chi.Router) {
				admin.Use(middleware.RequireAdmin)

				// Plugin management
				admin.Get("/plugins", p.PluginHandler.ListPlugins)
				admin.Post("/plugins", p.PluginHandler.InstallPlugin)
				admin.Get("/plugins/{pluginID}", p.PluginHandler.GetPlugin)
				admin.Post("/plugins/{pluginID}/enable", p.PluginHandler.EnablePlugin)
				admin.Post("/plugins/{pluginID}/disable", p.PluginHandler.DisablePlugin)
				admin.Delete("/plugins/{pluginID}", p.PluginHandler.UninstallPlugin)
				admin.Put("/plugins/{pluginID}/config", p.PluginHandler.UpdatePluginConfig)
				admin.Put("/plugins/{pluginID}/reload", p.PluginHandler.HotReloadPlugin)

				// User / subscription / invoice management
				admin.Get("/users", p.AdminHandler.ListUsers)
				admin.Get("/users/{userID}", p.AdminHandler.GetUser)
				admin.Get("/subscriptions", p.AdminHandler.ListSubscriptions)
				admin.Get("/invoices", p.AdminHandler.ListInvoices)

				// Tenant management
				admin.Post("/tenants", p.ResellerHandler.CreateTenant)
				admin.Get("/tenants", p.ResellerHandler.ListTenants)
				admin.Get("/tenants/{tenantID}", p.ResellerHandler.GetTenant)
				admin.Put("/tenants/{tenantID}/branding", p.ResellerHandler.UpdateBranding)
			})

			// Routing
			protected.Post("/routing/select", p.RoutingHandler.SelectNode)

			// Reseller self-service endpoints — require reseller or admin role.
			protected.Route("/reseller", func(resellerRouter chi.Router) {
				resellerRouter.Use(middleware.RequireReseller)
				resellerRouter.Get("/dashboard", p.ResellerHandler.Dashboard)
				resellerRouter.Get("/commissions", p.ResellerHandler.Commissions)
				resellerRouter.Get("/customers", p.ResellerHandler.Customers)
			})
		})
	})

	return r
}
