package gateway

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
)

// RegisterPluginRoutes reads route declarations from enabled plugins and
// registers them on the chi router. Public routes are added without auth
// middleware; protected routes require JWT authentication.
//
// For WASM plugins, requests are proxied to the plugin's RPC function.
// For built-in plugins, the handler is resolved from the BuiltinRouteRegistry.
//
// TODO: For hot-reloaded plugins, routes would need re-registration. Currently
// routes are registered once at startup.
func RegisterPluginRoutes(
	r chi.Router,
	pluginRepo plugin.PluginRepository,
	routeHandler *handler.PluginRouteHandler,
	builtinRegistry *BuiltinRouteRegistry,
	jwtIssuer *authutil.JWTIssuer,
	logger *slog.Logger,
) {
	ctx := context.Background()
	plugins, err := pluginRepo.GetEnabled(ctx)
	if err != nil {
		logger.Error("failed to load plugins for route registration", "error", err)
		return
	}

	registered := 0
	for _, p := range plugins {
		if p.Manifest == nil || len(p.Manifest.Routes) == 0 {
			continue
		}
		for _, route := range p.Manifest.Routes {
			var handlerFn http.HandlerFunc

			if p.IsBuiltIn {
				// Resolve native Go handler from the built-in registry.
				handlerFn = builtinRegistry.Lookup(p.Slug, route.Function)
				if handlerFn == nil {
					logger.Warn("no built-in handler registered for plugin route",
						slog.String("plugin", p.Slug),
						slog.String("function", route.Function),
					)
					continue
				}
			} else {
				// Proxy to WASM plugin RPC function.
				handlerFn = routeHandler.ProxyToPlugin(p.Slug, route.Function)
			}

			if route.Public {
				registerRoute(r, route.Method, route.Path, handlerFn)
			} else {
				r.With(middleware.Auth(jwtIssuer)).Method(route.Method, route.Path, handlerFn)
			}

			registered++
			logger.Info("registered plugin route",
				slog.String("plugin", p.Slug),
				slog.String("method", route.Method),
				slog.String("path", route.Path),
				slog.String("function", route.Function),
				slog.Bool("public", route.Public),
				slog.Bool("builtin", p.IsBuiltIn),
			)
		}
	}

	if registered > 0 {
		logger.Info("plugin routes registered", slog.Int("count", registered))
	}
}

// registerRoute dispatches a single route registration to the appropriate chi
// method.
func registerRoute(r chi.Router, method, path string, handler http.HandlerFunc) {
	switch method {
	case http.MethodGet:
		r.Get(path, handler)
	case http.MethodPost:
		r.Post(path, handler)
	case http.MethodPut:
		r.Put(path, handler)
	case http.MethodDelete:
		r.Delete(path, handler)
	case http.MethodPatch:
		r.Patch(path, handler)
	}
}
