package remnawave

import (
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
)

// Module provides the Remnawave HTTP client and resilient (circuit-breaker
// wrapped) client to the Fx dependency graph.
var Module = fx.Module("remnawave",
	fx.Provide(func(cfg *config.Config) *Client {
		return NewClient(cfg.Remnawave.URL, cfg.Remnawave.APIToken.Expose())
	}),
	fx.Provide(func(client *Client, cfg *config.Config, logger *slog.Logger) *ResilientClient {
		return NewResilientClient(client, cfg.CircuitBreaker.Remnawave, logger)
	}),
)
