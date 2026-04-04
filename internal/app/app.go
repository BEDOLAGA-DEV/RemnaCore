// Package app wires all Fx modules together into a single application.
package app

import (
	"go.uber.org/fx"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
)

// New constructs the Fx application with all modules wired together.
func New() *fx.App {
	return fx.New(
		// Config
		fx.Provide(config.Load),

		// Observability
		observability.Module,

		// Infrastructure adapters
		postgres.Module,
		valkey.Module,
		natsadapter.Module,
		remnawave.Module,

		// Domain-scoped wiring (repos, interface bindings, lifecycle hooks)
		identityWiring,
		billingWiring,
		multisubWiring,
		paymentWiring,
		resellerWiring,
		pluginWiring,

		// Cross-cutting infrastructure wiring
		natsWiring,
		infraWiring,
		httpWiring,
		telegramWiring,
		tracingWiring,
	)
}
