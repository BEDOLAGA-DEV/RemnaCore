package nats

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// ExtractTraceParent exports extractTraceParent for unit tests in the
// nats_test package.
func ExtractTraceParent(payload []byte) string {
	return extractTraceParent(payload)
}

// newTestBillingConsumerWithSubs builds a consumer wired with only the deps
// handleActivated needs: a SubscriptionProvider, a non-nil PlanProvider, a
// no-op SubscriptionEventHandler (dispatched to after enrichment), and a
// runner. Everything else is nil — handleActivated does not touch it.
func newTestBillingConsumerWithSubs(_ testingT, subs multisub.SubscriptionProvider, runner txmanager.Runner) *BillingEventConsumer {
	return NewBillingEventConsumer(
		nil,                 // subscriber
		&recordingHandler{}, // handler — handleActivated dispatches to it (no-op callback)
		nil,                 // checkout
		stubPlanProvider{},  // plans (must be non-nil: GetPlanSnapshot is called)
		subs,                // subs
		nil,                 // idempotency
		nil,                 // publisher
		nil,                 // schemaRegistry
		nil,                 // logger — handleActivated only logs on family-member failure
		nil,                 // clock
		nil,                 // metrics
		runner,              // runner (new param, immediately before conn)
		nil,                 // conn
	)
}

// handleActivatedForTest exposes the unexported handler to the _test package.
func (c *BillingEventConsumer) handleActivatedForTest(ctx context.Context, event domainevent.Event) error {
	return c.handleActivated(ctx, event)
}

// newTestBillingConsumerWithCheckout builds a consumer wired with only the deps
// handleChargeCompleted needs: a CheckoutCompleter and a runner. Everything
// else is nil — handleChargeCompleted touches only checkout and runner, so the
// other ports are never dereferenced.
func newTestBillingConsumerWithCheckout(_ testingT, checkout CheckoutCompleter, runner txmanager.Runner) *BillingEventConsumer {
	return NewBillingEventConsumer(
		nil,      // subscriber
		nil,      // handler — handleChargeCompleted does not dispatch to it
		checkout, // checkout — handleChargeCompleted delegates to it
		nil,      // plans
		nil,      // subs
		nil,      // idempotency
		nil,      // publisher
		nil,      // schemaRegistry
		nil,      // logger
		nil,      // clock
		nil,      // metrics
		runner,   // runner (wraps CompleteCheckout under the platform sentinel)
		nil,      // conn
	)
}

// handleChargeCompletedForTest exposes the unexported handler to the _test package.
func (c *BillingEventConsumer) handleChargeCompletedForTest(ctx context.Context, event domainevent.Event) error {
	return c.handleChargeCompleted(ctx, event)
}

// testingT is the minimal *testing.T surface the wrapper needs; keeping it as
// an interface lets export_test.go avoid importing testing into a non-_test file.
type testingT interface{ Helper() }
