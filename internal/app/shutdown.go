package app

import "time"

// Graceful Shutdown Ordering
//
// Fx tears down OnStop hooks in LIFO order (reverse of registration). Since
// infrastructure adapters are registered first (in postgres.Module,
// valkey.Module, natsadapter.Module) and application lifecycle hooks are
// registered later (in natsWiring, infraWiring, httpWiring), the natural
// teardown order is:
//
//	Phase 1: HTTP server       — stop accepting requests, drain in-flight
//	Phase 2: Infra services    — stop health monitor, speed test, proxy
//	Phase 3: NATS consumers    — cancel context + drain in-flight handlers
//	Phase 4: Outbox relay      — cancel context, wait for workers to finish
//	Phase 5: Partition manager — cancel background partition maintenance
//	Phase 6: NATS connection   — close (adapter OnStop hook)
//	Phase 7: Valkey client     — close (adapter OnStop hook)
//	Phase 8: PG pool           — close (adapter OnStop hook, must be last)
//
// This ordering is enforced by:
//   - Module declaration order in app.go (infrastructure adapters first)
//   - WiringConstraints (verified by TestWiringOrderSatisfiesConstraints)
//   - ShutdownConstraints (verified by TestShutdownOrderMatchesPhases)
//
// IMPORTANT: Start functions that spawn goroutines MUST wait for goroutine
// completion during their OnStop hook. Cancelling the context alone is
// insufficient because Fx proceeds to the next hook immediately.

// Shutdown phase timeouts. Each phase has a dedicated budget; Fx proceeds
// to the next OnStop hook after the current one returns (or its timeout
// expires).
const (
	// ShutdownPhaseHTTP is the maximum time for the HTTP server to drain
	// in-flight requests.
	ShutdownPhaseHTTP = 15 * time.Second

	// ShutdownPhaseConsumers is the maximum time for NATS consumers to
	// drain in-flight message handlers.
	ShutdownPhaseConsumers = 10 * time.Second

	// ShutdownPhaseRelay is the maximum time for the outbox relay workers
	// to finish their current batch after context cancellation.
	ShutdownPhaseRelay = 10 * time.Second
)

// ShutdownPhase documents a single step in the graceful shutdown sequence.
type ShutdownPhase struct {
	// Name identifies the phase for logging and testing.
	Name string

	// Order is the 1-based position in the shutdown sequence.
	Order int

	// Timeout is the maximum duration for this phase. Zero means the phase
	// is handled by its own Fx OnStop hook with the default Fx timeout.
	Timeout time.Duration

	// Reason explains why this phase exists at this position.
	Reason string
}

// ShutdownPhases is the authoritative reference for the graceful shutdown
// sequence. Tests verify that actual Fx hook ordering matches this.
var ShutdownPhases = []ShutdownPhase{
	{
		Name:    "http_server",
		Order:   1,
		Timeout: ShutdownPhaseHTTP,
		Reason:  "Stop accepting new requests and drain in-flight HTTP handlers before any backend shuts down",
	},
	{
		Name:    "infra_services",
		Order:   2,
		Timeout: 0,
		Reason:  "Stop health monitor, speed test, and subscription proxy",
	},
	{
		Name:    "nats_consumers",
		Order:   3,
		Timeout: ShutdownPhaseConsumers,
		Reason:  "Stop reading new NATS messages and drain in-flight event handlers",
	},
	{
		Name:    "outbox_relay",
		Order:   4,
		Timeout: ShutdownPhaseRelay,
		Reason:  "Wait for in-flight outbox batches to complete before closing NATS",
	},
	{
		Name:    "partition_manager",
		Order:   5,
		Timeout: 0,
		Reason:  "Cancel background partition maintenance",
	},
	{
		Name:    "nats_connection",
		Order:   6,
		Timeout: 0,
		Reason:  "Close NATS connection after all producers and consumers have stopped",
	},
	{
		Name:    "valkey_client",
		Order:   7,
		Timeout: 0,
		Reason:  "Close Valkey after HTTP server no longer accepts rate-limited requests",
	},
	{
		Name:    "pg_pool",
		Order:   8,
		Timeout: 0,
		Reason:  "Close database pool last — all DB-dependent components must have stopped",
	},
}

// ShutdownConstraint documents that one phase must complete before another
// begins. Verified by TestShutdownConstraintsAreSatisfied.
type ShutdownConstraint struct {
	Before string
	After  string
	Reason string
}

// ShutdownConstraints enumerates every shutdown ordering invariant.
var ShutdownConstraints = []ShutdownConstraint{
	{
		Before: "http_server",
		After:  "nats_consumers",
		Reason: "HTTP server must stop accepting requests before consumers stop processing events",
	},
	{
		Before: "nats_consumers",
		After:  "outbox_relay",
		Reason: "Consumers must drain before relay stops to avoid event reprocessing",
	},
	{
		Before: "outbox_relay",
		After:  "nats_connection",
		Reason: "Relay must finish publishing before NATS connection closes",
	},
	{
		Before: "nats_connection",
		After:  "pg_pool",
		Reason: "NATS must close before PG pool to avoid relay querying a closed pool",
	},
	{
		Before: "http_server",
		After:  "valkey_client",
		Reason: "HTTP server must stop before Valkey closes to avoid rate limiter errors",
	},
	{
		Before: "http_server",
		After:  "pg_pool",
		Reason: "HTTP server must stop before PG pool closes to avoid handler query errors",
	},
}
