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

// TestRLSAudit_AllHandlersClassified is the completeness checklist (C0.7/C0.7b/C0.7c).
//
// Every handle* method the BillingEventConsumer defines must appear in exactly
// one of the three sets below.
//
//   - wrapped      — a single-step handler that reaches a Tier-2 (tenant-scoped)
//     read/write and opens its OWN outer RunInTx under the platform sentinel
//     (or the event's tenant key, once events carry one) so the RLS GUC covers
//     the whole operation.
//   - scopeCarried — a multi-step orchestrator (deprovisioning saga / binding-
//     lifecycle loop). It does NOT open one wrapping tx; instead it ANNOTATES
//     the ctx with the platform sentinel (tenantctx.WithPlatformScope) and lets
//     each downstream Tier-2 step open its OWN narrow RunInTx, which sets the
//     GUC per step from the carried scope. This preserves per-step crash
//     durability (saga checkpoints commit independently) and keeps external
//     Remnawave HTTP calls OUTSIDE any open DB tx — both of which a single
//     wrapping tx would violate (regression from 1289083, fixed in C0.7c).
//   - noTier2      — pure routing / payload-only handler that performs no Tier-2
//     read at all (it dispatches to a classified handler or only fires an async
//     hook), so there is nothing to scope.
//
// Audit classification (every handle* method in billing_event_handlers.go):
//   - handleMessage          noTier2 — pure subject routing; opens no tx itself,
//     each branch dispatches to a classified handler below.
//   - handleActivated        wrapped — single-step enrichment reads
//     (GetSubscriptionInfo / GetPlanSnapshot / GetFamilyMemberIDs) inside one
//     outer RunInTx under the sentinel (C0.7).
//   - handleChargeCompleted  wrapped — single-step CompleteCheckout reads/updates
//     the invoice + balance inside one outer RunInTx under the sentinel (C0.7b).
//   - handleSimple           scopeCarried — delegates to the multi-step
//     OnSubscription{Cancelled,Paused,Resumed} orchestrators. The sentinel is
//     annotated on the ctx (no outer tx); each Tier-2 read/update opens its own
//     narrow RunInTx (per-step durability) and Remnawave Disable/Enable HTTP
//     calls run between those step-txs, outside any tx (C0.7c).
//   - handleTrafficExceeded  scopeCarried — sentinel annotated on ctx; the
//     OnBindingTrafficExceeded -> LimitBinding step reads/updates the binding in
//     its own narrow RunInTx (no gateway call on this path) (C0.7c).
//   - handleTrafficReset     scopeCarried — sentinel annotated on ctx; the
//     OnBindingTrafficReset -> UnlimitBinding step reads/updates the binding in
//     its own narrow RunInTx (no gateway call on this path) (C0.7c).
//   - handleTrafficWarning   noTier2 — sentinel annotated on ctx for consistency,
//     but OnTrafficWarning only fires an async hook; it performs no Tier-2 DB
//     read, so there is no tx and nothing to wrap (C0.7c).
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
	// Keep these three sets exhaustive: every handle* method must appear in
	// exactly one. A new single-step handler with a Tier-2 read MUST be wrapped
	// (C0.7/C0.7b); a new multi-step orchestrator handler MUST carry the scope on
	// ctx and rely on per-step RunInTx (C0.7c).
	wrapped := map[string]bool{
		"handleActivated": true, "handleChargeCompleted": true,
	}
	scopeCarried := map[string]bool{
		"handleSimple": true, "handleTrafficExceeded": true,
		"handleTrafficReset": true,
	}
	noTier2 := map[string]bool{
		"handleMessage": true, "handleTrafficWarning": true,
	}
	all := listHandleMethods(t, "billing_event_handlers.go")
	require.NotEmpty(t, all, "expected at least one handle* method in the source")
	for _, name := range all {
		classifications := 0
		if wrapped[name] {
			classifications++
		}
		if scopeCarried[name] {
			classifications++
		}
		if noTier2[name] {
			classifications++
		}
		assert.Equalf(t, 1, classifications,
			"handler %q must be in exactly one set (wrapped / scopeCarried / noTier2), found %d", name, classifications)
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
