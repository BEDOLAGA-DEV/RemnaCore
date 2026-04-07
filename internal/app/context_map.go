package app

// Mechanism describes how two bounded contexts communicate.
type Mechanism string

const (
	// MechanismEvent is an asynchronous dependency via NATS JetStream (outbox
	// relay). The producing context publishes domain events; the consuming
	// context subscribes through a NATS consumer. No compile-time import
	// exists between the two domain packages.
	MechanismEvent Mechanism = "event"

	// MechanismPort is a synchronous dependency through a domain-owned
	// interface (port). The calling context defines the interface in its own
	// package; the wiring layer (internal/app/) supplies an adapter that
	// translates to the target context's types.
	MechanismPort Mechanism = "port"

	// MechanismACL is a synchronous dependency through an Anti-Corruption
	// Layer adapter. Similar to port, but the adapter reads from the target
	// context's persistence directly (e.g. billing DB tables) rather than
	// calling a domain service.
	MechanismACL Mechanism = "acl"

	// MechanismGateway is a gateway-layer orchestration where the HTTP
	// handler coordinates calls to multiple bounded contexts. No domain-level
	// import exists between the contexts; the handler depends on each
	// context's service independently.
	MechanismGateway Mechanism = "gateway"

	// MechanismExternal is a dependency on an external system (e.g.
	// Remnawave VPN panel) accessed through a domain-owned gateway interface.
	MechanismExternal Mechanism = "external"

	// MechanismPlugin is a dependency dispatched through the WASM plugin
	// system via hookdispatch.Dispatcher. The domain defines the port; the
	// wiring layer supplies a plugin-backed adapter.
	MechanismPlugin Mechanism = "plugin"
)

// CrossContextDependency declares an allowed communication path between two
// bounded contexts (or between a context and an external system). Architecture
// tests verify that no undeclared cross-context imports exist.
type CrossContextDependency struct {
	From        string    // source context: "billing", "multisub", etc.
	To          string    // target context or external system
	Mechanism   Mechanism // how they communicate
	Description string    // human-readable explanation
}

// ContextMap is the authoritative list of allowed cross-context communication
// in RemnaCore. Architecture tests (TestContextMapMatchesRealImports) verify
// that actual adapter-level wiring matches this declaration.
//
// When adding a new cross-context communication path:
//  1. Add an entry to this slice.
//  2. Run `go test ./tests/archtest/ -v` to verify.
//  3. If the test fails, either the entry is wrong or the wiring is wrong.
var ContextMap = []CrossContextDependency{
	// =====================================================================
	// Event-driven (async via transactional outbox -> NATS JetStream)
	// =====================================================================

	// billing -> multisub: subscription lifecycle triggers VPN provisioning.
	// Events: subscription.activated, .cancelled, .paused, .resumed, .expired,
	//         .upgraded, .downgraded, family_member.added, family_member.removed
	// Path: BillingService publishes -> outbox -> NATS -> BillingEventConsumer
	//       -> MultiSubOrchestrator (Remnawave provisioning/deprovisioning)
	{From: "billing", To: "multisub", Mechanism: MechanismEvent,
		Description: "subscription lifecycle events trigger VPN provisioning/deprovisioning"},

	// payment -> billing: charge completion triggers invoice payment.
	// Events: payment.charge_completed, payment.refund_completed
	// Path: PaymentFacade publishes -> outbox -> NATS -> BillingEventConsumer
	//       -> CheckoutService.CompleteCheckout
	{From: "payment", To: "billing", Mechanism: MechanismEvent,
		Description: "charge/refund completion events trigger invoice state transitions"},

	// multisub -> billing: traffic exceeded notification.
	// Events: binding.traffic_exceeded
	// Path: TrafficLifecycleService publishes -> outbox -> NATS -> (future consumer)
	{From: "multisub", To: "billing", Mechanism: MechanismEvent,
		Description: "traffic exceeded events notify billing for potential action"},

	// identity -> plugins (notification): user lifecycle events.
	// Events: user.registered, user.email_verified, user.password_reset_requested
	// Path: IdentityService publishes -> outbox -> NATS -> PluginAsyncConsumer
	//       -> notification plugins (email, Telegram)
	{From: "identity", To: "plugin", Mechanism: MechanismEvent,
		Description: "user lifecycle events dispatched to notification plugins"},

	// billing -> plugins (notification): subscription/invoice events.
	// Path: BillingService publishes -> outbox -> NATS -> PluginAsyncConsumer
	{From: "billing", To: "plugin", Mechanism: MechanismEvent,
		Description: "subscription and invoice events dispatched to notification plugins"},

	// multisub -> plugins (notification): binding lifecycle events.
	// Path: MultiSubOrchestrator publishes -> outbox -> NATS -> PluginAsyncConsumer
	{From: "multisub", To: "plugin", Mechanism: MechanismEvent,
		Description: "binding lifecycle events dispatched to notification plugins"},

	// =====================================================================
	// Synchronous ports (domain-level interfaces)
	// =====================================================================

	// billing -> payment: PaymentGateway ACL port for charge creation.
	// Interface: billing.PaymentGateway (billing/payment_gateway.go)
	// Adapter: PaymentGatewayAdapter (internal/adapter/billing/)
	// Path: CheckoutService.StartCheckout -> PaymentGateway.CreateCharge
	//       -> PaymentGatewayAdapter -> payment.PaymentFacade.CreateCharge
	{From: "billing", To: "payment", Mechanism: MechanismPort,
		Description: "PaymentGateway ACL port for charge creation during checkout"},

	// billing -> plugin: PricingModifier + BillingService hook dispatch.
	// Interfaces: billing.PricingModifier (pricing.calculate), BillingService (subscription hooks)
	// Adapters: pluginPricingModifier (internal/app/), hookdispatch.Dispatcher
	// Path: CheckoutService -> PricingModifier.ModifyPricing -> Dispatcher -> WASM
	//       BillingService -> Dispatcher -> WASM
	{From: "billing", To: "plugin", Mechanism: MechanismPlugin,
		Description: "PricingModifier + BillingService dispatch pricing and subscription hooks to WASM plugins"},

	// multisub -> plugin: VPNProvider + orchestrator hook dispatch.
	// Interfaces: multisub.VPNProvider (VPN hooks), MultiSubOrchestrator (lifecycle hooks)
	// Adapters: pluginVPNProvider (internal/adapter/plugin/), hookdispatch.Dispatcher
	// Path: ProvisioningSaga -> VPNProvider.CreateUser -> hookdispatch -> WASM
	//       MultiSubOrchestrator -> Dispatcher -> WASM
	{From: "multisub", To: "plugin", Mechanism: MechanismPlugin,
		Description: "VPNProvider + orchestrator dispatch VPN and lifecycle hooks to WASM plugins"},

	// =====================================================================
	// ACL adapters (read-only cross-context data access)
	// =====================================================================

	// multisub -> billing: PlanProvider and SubscriptionProvider read billing data.
	// Interfaces: multisub.PlanProvider, multisub.SubscriptionProvider (multisub/ports.go)
	// Adapter: natsadapter.BillingSubscriptionLookup
	// Path: MultiSubOrchestrator needs plan/subscription info to calculate bindings.
	//       Adapter reads from billing's DB tables (PlanSnapshot ACL type).
	{From: "multisub", To: "billing", Mechanism: MechanismACL,
		Description: "PlanProvider + SubscriptionProvider ACL reads plan/subscription data for binding calculation"},

	// =====================================================================
	// External system dependencies
	// =====================================================================

	// multisub -> remnawave: VPN panel operations.
	// Interface: multisub.RemnawaveGateway (multisub/gateway.go)
	// Adapter: remnawave.GatewayAdapter (internal/adapter/remnawave/)
	// Methods: CreateUser, DeleteUser, EnableUser, DisableUser, GetUser, AssignToSquad
	{From: "multisub", To: "remnawave", Mechanism: MechanismExternal,
		Description: "RemnawaveGateway for VPN user CRUD via Remnawave panel API"},

	// =====================================================================
	// Gateway-layer orchestration (NOT domain-to-domain imports)
	// =====================================================================

	// gateway -> payment + billing: webhook completion flow.
	// Path: PaymentWebhookHandler -> payment.PaymentFacade.VerifyWebhook
	//       -> payment.PaymentFacade.CompletePayment (publishes charge_completed)
	//       -> billing picks up event via BillingEventConsumer
	// Note: handler talks only to payment; billing completion is event-driven.
	{From: "gateway", To: "payment", Mechanism: MechanismGateway,
		Description: "PaymentWebhookHandler verifies and completes payments via PaymentFacade"},
}

// AllowedSyncImports returns the set of allowed synchronous imports between
// domain packages, derived from ContextMap. Only port and ACL mechanisms
// represent compile-time dependencies between domain packages.
//
// The returned map keys are source context names; values are slices of target
// context names that the source is allowed to import.
//
// Note: currently no domain package directly imports another domain package
// (all cross-context wiring happens at the adapter/app layer). This function
// exists to catch future violations where someone accidentally adds a direct
// domain-to-domain import.
func AllowedSyncImports() map[string][]string {
	allowed := make(map[string][]string)
	for _, dep := range ContextMap {
		if dep.Mechanism == MechanismPort || dep.Mechanism == MechanismACL {
			allowed[dep.From] = append(allowed[dep.From], dep.To)
		}
	}
	return allowed
}

// EventFlows returns the subset of ContextMap entries that use event-driven
// communication. Useful for documentation generation and architecture
// visualization tools.
func EventFlows() []CrossContextDependency {
	var flows []CrossContextDependency
	for _, dep := range ContextMap {
		if dep.Mechanism == MechanismEvent {
			flows = append(flows, dep)
		}
	}
	return flows
}
