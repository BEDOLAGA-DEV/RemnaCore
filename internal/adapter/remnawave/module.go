package remnawave

import (
	"log/slog"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
)

// Module provides the resilient (circuit-breaker wrapped) Remnawave client to
// the Fx dependency graph.
//
// The underlying *Client is deliberately NOT provided here. Its endpoint comes
// from the admin-managed plugin configuration, which lives in the database, and
// this package cannot import the plugin that owns it without a cycle. The
// composition root wires it instead — see provideRemnawaveClient in
// internal/app.
var Module = fx.Module("remnawave",
	fx.Provide(func(client *Client, cfg *config.Config, logger *slog.Logger) *ResilientClient {
		return NewResilientClient(client, cfg.CircuitBreaker.Remnawave, logger)
	}),
)
