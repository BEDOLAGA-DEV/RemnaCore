// Package app wires all Fx modules together into a single application.
//
// # Context Map — Cross-Bounded-Context Communication
//
// RemnaCore has five bounded contexts: identity, billing, multisub, payment,
// and reseller. This section documents every cross-context communication path.
// Understanding these connections is critical for future microservice extraction.
//
// ## Event-Driven (via NATS JetStream, through transactional outbox)
//
//   billing → multisub:
//     Events: subscription.activated, subscription.cancelled, subscription.paused,
//             subscription.resumed, subscription.expired
//     Path:   BillingService publishes → outbox → NATS → BillingEventConsumer
//             → MultiSubOrchestrator (Remnawave provisioning/deprovisioning)
//     Wiring: wiring_nats.go (BillingEventConsumer), wiring_multisub.go (orchestrator)
//
//   identity → (all contexts via plugins):
//     Events: user.registered, user.email_verified, user.password_reset_requested
//     Path:   IdentityService publishes → outbox → NATS → PluginAsyncConsumer
//             → notification plugins (email, Telegram)
//     Wiring: wiring_nats.go (PluginAsyncConsumer)
//
//   payment → billing (planned):
//     Events: payment.charge_completed, payment.refund_completed
//     Path:   (currently handled synchronously in webhook handler; future
//             event consumer for async invoice state transitions)
//
// ## Synchronous Calls (via ACL interfaces)
//
//   billing → payment:
//     Interface: billing.PaymentGateway (defined in billing/payment_gateway.go)
//     Method:    CreateCharge (checkout flow only)
//     Adapter:   paymentGatewayAdapter (payment_gateway_adapter.go)
//     Path:      CheckoutService.StartCheckout → PaymentGateway.CreateCharge
//                → paymentGatewayAdapter → payment.PaymentFacade.CreateCharge
//                → plugin: payment.create_charge hook
//     Wiring:    wiring_billing.go (newPaymentGatewayAdapter)
//
//   billing → plugin:
//     Interface: hookdispatch.Dispatcher (defined in pkg/hookdispatch/)
//     Method:    DispatchSync for pricing.calculate hook
//     Path:      CheckoutService → Dispatcher → plugin.HookDispatcher → WASM
//     Wiring:    wiring_plugin.go (HookDispatcher → hookdispatch.Dispatcher)
//
//   gateway → payment + billing (webhook completion):
//     Path:      PaymentWebhookHandler (gateway layer)
//                → payment.PaymentFacade.VerifyWebhook (plugin: payment.verify_webhook)
//                → payment.PaymentFacade.CompletePayment
//                → billing.CheckoutService.CompleteCheckout
//                  → BillingService.PayInvoice → publishes invoice.paid,
//                    subscription.activated
//     Note:      This is a gateway-layer orchestration, not a domain-to-domain
//                call. The gateway handler coordinates two bounded contexts.
//     Wiring:    wiring_nats.go (provideWebhookHandler), wiring_http.go
//
//   multisub → billing (lookup ports, read-only):
//     Interfaces: multisub.PlanProvider, multisub.SubscriptionProvider
//     Path:       MultiSubOrchestrator needs plan/subscription info to
//                 calculate bindings. Adapter reads from billing's DB tables.
//     Adapter:    natsadapter.BillingSubscriptionLookup
//     Wiring:     wiring_multisub.go (PlanProvider, SubscriptionProvider)
//
//   multisub → remnawave (external system):
//     Interface:  multisub.RemnawaveGateway (defined in multisub/gateway.go)
//     Methods:    CreateUser, DeleteUser, EnableUser, DisableUser, GetUser
//     Adapter:    remnawave.GatewayAdapter (internal/adapter/remnawave/)
//     Wiring:     wiring_multisub.go
//
//   multisub → plugin (optional, gated by HooksVPNProviderEnabled):
//     Interface:  multisub.VPNProvider (defined in multisub/vpn_provider.go)
//     Methods:    CreateUser, GetUser, DeleteUser, EnableUser, DisableUser
//     Adapter:    pluginadapter.pluginVPNProvider (internal/adapter/plugin/)
//     Path:       VPNProvider dispatches hooks to WASM plugins via Dispatcher;
//                 HTTP transport handled by resilientVPNExecutor with circuit breaker.
//     Wiring:     wiring_multisub.go (provideVPNProvider)
//     Note:       Returns nil when feature flag is off — sagas use RemnawaveGateway directly.
//
// ## Shared Kernel (via pkg/)
//
//   All contexts:        domainevent (Event, Publisher), clock (Clock),
//                        txmanager (Runner)
//   Identity + gateway:  authutil (JWT issuer, password hashing)
//   Billing + multisub + payment: hookdispatch (Dispatcher interface for WASM plugins)
//
// ## Remnawave Webhooks (external → platform)
//
//   Path: Remnawave panel → /api/webhooks/remnawave → remnawave.WebhookHandler
//         → natsadapter.EventPublisher (publishes as domain events to NATS)
//   Note: These are external-origin events, not cross-context domain events.
//   Wiring: wiring_nats.go (provideWebhookHandler), wiring_http.go
//
// ## Microservice Extraction Guidance
//
// When extracting bounded contexts into separate services:
//   - Event-driven connections need no changes (NATS is already decoupled).
//   - Synchronous ACL calls (PaymentGateway, PlanProvider, SubscriptionProvider)
//     must be replaced with gRPC/HTTP calls to the extracted service.
//   - The gateway-layer webhook orchestration (payment → billing completion)
//     should become an event choreography: payment publishes charge_completed,
//     billing subscribes and completes checkout.
//   - Shared kernel packages (pkg/) move to a shared library or are duplicated
//     per service.
//   - The multisub→billing lookup adapters must become cross-service API calls
//     or be replaced with local read models populated by billing events.
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
