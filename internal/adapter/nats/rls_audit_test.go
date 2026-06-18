package nats

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// handleMethodPrefix is the naming convention every billing-event consumer
// handler follows; the audit parses for methods starting with it.
const handleMethodPrefix = "handle"

// tenantCapturingCheckout records the tenant on the ctx when CompleteCheckout
// fires, so the audit test can assert the charge-completed path is wrapped.
type tenantCapturingCheckout struct{ seenTenant string }

func (c *tenantCapturingCheckout) CompleteCheckout(ctx context.Context, _ string) error {
	c.seenTenant = tenantctx.TenantIDFromContext(ctx)
	return nil
}

func TestHandleChargeCompleted_RunsUnderPlatformSentinel(t *testing.T) {
	co := &tenantCapturingCheckout{}
	c := newTestBillingConsumerWithCheckout(t, co, &passthroughRunner{})

	event := domainevent.NewTyped(
		payment.ChargeCompletedPayload{InvoiceID: "inv1"},
		time.Now(),
		"inv1",
	)
	_ = c.handleChargeCompletedForTest(context.Background(), event)

	assert.Equal(t, tenantctx.PlatformScopeSentinel, co.seenTenant,
		"charge-completed invoice read/update must run under the platform sentinel")
}

// TestRLSAudit_AllHandlersClassified is the completeness checklist (C0.7/C0.7b).
//
// Every handle* method the BillingEventConsumer defines must appear in exactly
// one of the two sets below. A handler that reaches a Tier-2 (tenant-scoped)
// read/write on a bare ctx MUST be wrapped in RunInTx under the platform
// sentinel (or the event's tenant key, once events carry one); a handler that
// performs no Tier-2 read (pure routing, payload-only, or delegation to an
// orchestrator that opens its OWN RunInTx downstream) is recorded as noTier2.
//
// Audit classification (every handle* method in billing_event_handlers.go):
//   - handleMessage          noTier2 — pure subject routing; opens no tx itself,
//     each branch dispatches to a classified handler below.
//   - handleActivated        wrapped — enrichment reads (GetSubscriptionInfo /
//     GetPlanSnapshot / GetFamilyMemberIDs) under the sentinel (fixed in C0.7).
//   - handleChargeCompleted  wrapped — CompleteCheckout reads/updates the
//     invoice + balance under the sentinel (fixed in C0.7b).
//   - handleSimple           noTier2 — payload-only; delegates to the
//     orchestrator (OnSubscription{Cancelled,Paused,Resumed}) which opens its
//     own RunInTx downstream.
//   - handleTrafficExceeded  noTier2 — payload-only; delegates to
//     orchestrator OnBindingTrafficExceeded (owns its tx).
//   - handleTrafficReset     noTier2 — payload-only; delegates to
//     orchestrator OnBindingTrafficReset (owns its tx).
//   - handleTrafficWarning   noTier2 — payload-only; fire-and-forget
//     OnTrafficWarning (owns its tx).
//
// The remaining audited package entry points perform no bare-ctx Tier-2 read:
//   - multisub_events.go        publisher only (Publish/PublishBatch); no read.
//   - plugin_events.go          handleMessage routes async hooks to plugins; no
//     tenant-scoped DB read.
//   - dlq_consumer.go           handleMessage logs the dead letter; no DB read.
//   - dlq_replay.go             replays NATS messages between subjects; no DB.
//   - outbox_relay.go           relay()/cleanup() touch the platform-level
//     outbox infra table inside their own RunInTx; not Tier-2 tenant data.
//   - outbox_reconciliation.go  read-only sequence comparison vs JetStream.
//   - outbox_health_checker.go  outbox health probe; infra, not Tier-2.
//   - health_checker.go         NATS connection probe; no DB.
//   - billing_lookup.go         the lookup ports invoked by handleActivated
//     inside its sentinel-scoped tx; carry no tx of their own.
func TestRLSAudit_AllHandlersClassified(t *testing.T) {
	// Keep these two sets exhaustive: every handle* method must appear in
	// exactly one. A new handler with a Tier-2 read MUST be wrapped (C0.7/C0.7b).
	wrapped := map[string]bool{"handleActivated": true, "handleChargeCompleted": true}
	noTier2 := map[string]bool{
		"handleMessage": true,
		"handleSimple":  true, "handleTrafficExceeded": true,
		"handleTrafficReset": true, "handleTrafficWarning": true,
	}
	all := listHandleMethods(t, "billing_event_handlers.go")
	require.NotEmpty(t, all, "expected at least one handle* method in the source")
	for _, name := range all {
		inWrapped := wrapped[name]
		inNoTier2 := noTier2[name]
		assert.Truef(t, inWrapped || inNoTier2,
			"handler %q is unclassified: add it to wrapped (with RunInTx) or noTier2", name)
		assert.Falsef(t, inWrapped && inNoTier2,
			"handler %q must be in exactly one set, not both", name)
	}
}

// listHandleMethods parses the given source file in the current package and
// returns the names of every method whose name starts with handleMethodPrefix.
// It uses go/parser + go/ast (no DB, no reflection of unexported methods).
func listHandleMethods(t *testing.T, filename string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, parser.SkipObjectResolution)
	require.NoErrorf(t, err, "parse %s", filename)

	var names []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil {
			continue
		}
		if strings.HasPrefix(fn.Name.Name, handleMethodPrefix) {
			names = append(names, fn.Name.Name)
		}
	}
	return names
}
