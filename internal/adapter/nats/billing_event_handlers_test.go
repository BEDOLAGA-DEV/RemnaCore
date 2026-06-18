package nats

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// tenantCapturingSubs records the tenant ID present on the ctx when the
// subscription-info enrichment read fires. It embeds the port so any method
// we don't override panics rather than silently returning a zero value, and
// overrides the two methods handleActivated actually calls.
type tenantCapturingSubs struct {
	multisub.SubscriptionProvider
	seenTenant string
}

func (p *tenantCapturingSubs) GetSubscriptionInfo(ctx context.Context, id string) (multisub.SubscriptionInfo, error) {
	p.seenTenant = tenantctx.TenantIDFromContext(ctx)
	return multisub.SubscriptionInfo{ID: id, UserID: "u1", PlanID: "p1"}, nil
}

// GetFamilyMemberIDs must be a real (no-op) override: handleActivated calls it
// on the enrichment ctx, and the embedded nil port would panic otherwise.
func (p *tenantCapturingSubs) GetFamilyMemberIDs(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

// stubPlanProvider is a non-nil PlanProvider so handleActivated's
// GetPlanSnapshot call does not nil-panic; it returns a zero snapshot.
type stubPlanProvider struct{}

func (stubPlanProvider) GetPlanSnapshot(_ context.Context, _ string) (multisub.PlanSnapshot, error) {
	return multisub.PlanSnapshot{}, nil
}

func TestHandleActivated_EnrichesUnderPlatformScope(t *testing.T) {
	subs := &tenantCapturingSubs{}
	c := newTestBillingConsumerWithSubs(t, subs, &passthroughRunner{})

	// NewTyped(payload, ts, entityID) — all three args are required.
	event := domainevent.NewTyped(
		billingaggregate.SubActivatedPayload{SubscriptionID: "s1", UserID: "u1"},
		time.Now(),
		"s1",
	)
	_ = c.handleActivatedForTest(context.Background(), event)

	assert.Equal(t, tenantctx.PlatformScopeSentinel, subs.seenTenant,
		"background enrichment must read under the platform sentinel")
}

type passthroughRunner struct{}

func (passthroughRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
