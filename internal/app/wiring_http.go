package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
)

// httpShutdownTimeout is the maximum time allowed for the HTTP server to
// complete in-flight requests during graceful shutdown.
const httpShutdownTimeout = 10 * time.Second

// httpWiring provides the HTTP server lifecycle: starts the chi router on
// OnStart and shuts down gracefully on OnStop.
var httpWiring = fx.Options(
	// Gateway module
	gateway.Module,

	// Rate limiter: middleware.RateLimiter wraps *valkey.SlidingWindowRateLimiter
	fx.Provide(func(r *valkey.SlidingWindowRateLimiter) middleware.RateLimiter { return r }),

	// HTTP server lifecycle
	fx.Invoke(startHTTPServer),
)

// startHTTPServer registers an HTTP server that starts listening on OnStart and
// shuts down gracefully on OnStop.
func startHTTPServer(lc fx.Lifecycle, router http.Handler, cfg *config.Config, logger *slog.Logger) {
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("binding to %s: %w", addr, err)
			}
			logger.Info("http server starting", slog.String("addr", addr))
			go func() {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error("http server error", slog.Any("error", err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("http server shutting down")
			shutdownCtx, cancel := context.WithTimeout(ctx, httpShutdownTimeout)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		},
	})
}
