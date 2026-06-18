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

// testingT is the minimal *testing.T surface the wrapper needs; keeping it as
// an interface lets export_test.go avoid importing testing into a non-_test file.
type testingT interface{ Helper() }
