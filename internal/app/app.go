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
//
// Startup ordering (Fx.Invoke hooks execute in declaration order):
//  1. Infrastructure adapters (DB, Valkey, NATS) -- provided by Fx modules
//  2. Plugin loading -- must complete before event consumers start
//  3. Outbox relay + partition manager
//  4. Event consumers -- depend on plugins being loaded
//  5. Health monitor + speed test + subscription proxy
//  6. HTTP server -- last, only accepts traffic after all deps ready
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

		// Domain-scoped wiring (repos, interface bindings, lifecycle hooks).
		// pluginWiring MUST appear before natsWiring so that loadEnabledPlugins
		// runs before startBillingEventConsumer / startPluginAsyncConsumer.
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
