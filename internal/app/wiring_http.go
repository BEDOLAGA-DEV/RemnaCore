package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// httpShutdownTimeout is the maximum time allowed for the HTTP server to
// complete in-flight requests during graceful shutdown. Defined separately
// from ShutdownPhaseHTTP to allow per-environment overrides; both default
// to the same value. See shutdown.go for the full shutdown phase reference.
const httpShutdownTimeout = ShutdownPhaseHTTP

// httpWiring provides the HTTP server lifecycle: starts the chi router on
// OnStart and shuts down gracefully on OnStop.
var httpWiring = fx.Options(
	// Gateway module
	gateway.Module,

	// Rate limiter: middleware.RateLimiter wraps *valkey.ResilientRateLimiter
	fx.Provide(func(r *valkey.ResilientRateLimiter) middleware.RateLimiter { return r }),

	// Auth-endpoint rate limiters with per-endpoint thresholds.
	fx.Provide(newAuthRateLimiters),

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

// newAuthRateLimiters creates per-endpoint auth rate limiters backed by Valkey
// sliding window counters. Each endpoint gets its own limiter instance with
// independent limits read from configuration.
func newAuthRateLimiters(client *redis.Client, cfg *config.Config, clk clock.Clock) *middleware.AuthRateLimiters {
	loginLimit := cfg.RateLimit.LoginMaxPerWindow
	if loginLimit == 0 {
		loginLimit = config.DefaultLoginMaxPerWindow
	}
	loginWindow := time.Duration(cfg.RateLimit.LoginWindowMinutes) * time.Minute
	if cfg.RateLimit.LoginWindowMinutes == 0 {
		loginWindow = time.Duration(config.DefaultLoginWindowMinutes) * time.Minute
	}

	forgotLimit := cfg.RateLimit.ForgotPwdMaxPerWindow
	if forgotLimit == 0 {
		forgotLimit = config.DefaultForgotPwdMaxPerWindow
	}
	forgotWindow := time.Duration(cfg.RateLimit.ForgotPwdWindowMinutes) * time.Minute
	if cfg.RateLimit.ForgotPwdWindowMinutes == 0 {
		forgotWindow = time.Duration(config.DefaultForgotPwdWindowMinutes) * time.Minute
	}

	return &middleware.AuthRateLimiters{
		Login: valkey.NewSlidingWindowRateLimiter(client, loginLimit, loginWindow, clk),
		LoginCfg: middleware.AuthRateLimitConfig{
			Limit:  loginLimit,
			Window: loginWindow,
		},
		ForgotPwd: valkey.NewSlidingWindowRateLimiter(client, forgotLimit, forgotWindow, clk),
		ForgotPwdCfg: middleware.AuthRateLimitConfig{
			Limit:  forgotLimit,
			Window: forgotWindow,
		},
	}
}
