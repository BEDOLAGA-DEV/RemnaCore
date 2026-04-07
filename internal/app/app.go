// Package app wires all Fx modules together into a single application.
//
// Cross-context communication is declared in context_map.go and verified by
// TestContextMapMatchesRealImports and TestContextMapCoversEventCatalogConsumers
// in tests/archtest/.
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

// WiringModule names used in ModuleOrder and WiringConstraints.
const (
	ModuleIdentity = "identity"
	ModuleBilling  = "billing"
	ModuleMultisub = "multisub"
	ModulePayment  = "payment"
	ModuleReseller = "reseller"
	ModulePlugin   = "plugin"
	ModuleNATS     = "nats"
	ModuleInfra    = "infra"
	ModuleHTTP     = "http"
	ModuleTelegram = "telegram"
	ModuleTracing  = "tracing"
)

// ModuleOrder documents the Fx module loading sequence. fx.Invoke hooks
// execute in declaration order, so modules with startup hooks (fx.Invoke)
// MUST appear in the correct position. Constraints are verified by
// TestWiringOrderSatisfiesConstraints.
var ModuleOrder = []string{
	ModuleIdentity,
	ModuleBilling,
	ModuleMultisub,
	ModulePayment,
	ModuleReseller,
	ModulePlugin,
	ModuleNATS,
	ModuleInfra,
	ModuleHTTP,
	ModuleTelegram,
	ModuleTracing,
}

// WiringConstraint documents that the module named Before must appear
// before the module named After in ModuleOrder. Reason explains why.
type WiringConstraint struct {
	Before string
	After  string
	Reason string
}

// WiringConstraints enumerates every ordering invariant between wiring
// modules. Each constraint corresponds to a real runtime dependency:
// fx.Invoke hooks execute in declaration order, so placing After before
// Before would cause a startup failure or silent misbehaviour.
var WiringConstraints = []WiringConstraint{
	{
		Before: ModulePlugin,
		After:  ModuleNATS,
		Reason: "loadEnabledPlugins must complete before startBillingEventConsumer and startPluginAsyncConsumer",
	},
	{
		Before: ModuleNATS,
		After:  ModuleHTTP,
		Reason: "event consumers and outbox relay must be running before HTTP server accepts traffic",
	},
	{
		Before: ModuleInfra,
		After:  ModuleHTTP,
		Reason: "health monitor and speed test must start before HTTP server advertises readiness",
	},
	{
		Before: ModulePlugin,
		After:  ModuleHTTP,
		Reason: "plugins must be loaded before HTTP handlers dispatch hooks",
	},
	{
		Before: ModuleIdentity,
		After:  ModuleHTTP,
		Reason: "identity service and JWT issuer must be available before HTTP auth middleware runs",
	},
	{
		Before: ModuleBilling,
		After:  ModuleNATS,
		Reason: "CheckoutService must be available before BillingEventConsumer starts",
	},
	{
		Before: ModuleMultisub,
		After:  ModuleNATS,
		Reason: "MultiSubOrchestrator must be available before BillingEventConsumer routes events to it",
	},
}

// New constructs the Fx application with all modules wired together.
//
// Startup ordering (Fx.Invoke hooks execute in declaration order):
//  1. Infrastructure adapters (DB, Valkey, NATS) -- provided by Fx modules
//  2. Plugin loading -- must complete before event consumers start
//  3. Outbox relay + partition manager
//  4. Event consumers -- depend on plugins being loaded
//  5. Health monitor + speed test + subscription proxy
//  6. HTTP server -- last, only accepts traffic after all deps ready
//
// See WiringConstraints and TestWiringOrderSatisfiesConstraints.
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
		// Order must satisfy WiringConstraints — see wiring_order_test.go.
		identityWiring,  // ModuleIdentity
		billingWiring,   // ModuleBilling
		multisubWiring,  // ModuleMultisub
		paymentWiring,   // ModulePayment
		resellerWiring,  // ModuleReseller
		pluginWiring,    // ModulePlugin

		// Cross-cutting infrastructure wiring
		natsWiring,      // ModuleNATS
		infraWiring,     // ModuleInfra
		httpWiring,      // ModuleHTTP
		telegramWiring,  // ModuleTelegram
		tracingWiring,   // ModuleTracing
	)
}
