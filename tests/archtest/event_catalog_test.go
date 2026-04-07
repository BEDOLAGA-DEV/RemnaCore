package archtest

import (
	"slices"
	"testing"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	multisubaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/health"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/app"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// totalExpectedEvents is the expected number of event types across all bounded
// contexts. Update this constant when adding or removing event types.
const totalExpectedEvents = 64

// TestAllEventTypesInCatalog verifies that every EventType constant defined
// across all bounded contexts is registered in the event catalog. Adding a
// new event constant without registering it in BuildEventCatalog will fail
// this test.
func TestAllEventTypesInCatalog(t *testing.T) {
	catalog := app.BuildEventCatalog()
	allTypes := collectAllEventTypes()

	for _, et := range allTypes {
		t.Run(string(et), func(t *testing.T) {
			_, ok := catalog.Get(et)
			assert.True(t, ok, "event type %q not registered in catalog", et)
		})
	}
}

// TestCatalogContainsExactCount verifies the catalog has exactly the expected
// number of entries. If a new event is registered in the catalog but not added
// to collectAllEventTypes, this test catches the asymmetry.
func TestCatalogContainsExactCount(t *testing.T) {
	catalog := app.BuildEventCatalog()
	allTypes := collectAllEventTypes()

	require.Equal(t, len(allTypes), catalog.Len(),
		"catalog entry count does not match collectAllEventTypes count; "+
			"update both the catalog and the test when adding events")
	require.Equal(t, totalExpectedEvents, catalog.Len(),
		"catalog entry count does not match totalExpectedEvents constant; "+
			"update the constant when adding or removing events")
}

// TestCatalogEntryFields validates that every catalog entry has required
// fields populated.
func TestCatalogEntryFields(t *testing.T) {
	catalog := app.BuildEventCatalog()

	for eventType, entry := range catalog.All() {
		t.Run(string(eventType), func(t *testing.T) {
			assert.NotEmpty(t, entry.Type, "entry Type must not be empty")
			assert.NotEmpty(t, entry.Producer, "entry Producer must not be empty for %s", eventType)
			assert.Greater(t, entry.Version, 0, "entry Version must be positive for %s", eventType)
			assert.NotNil(t, entry.Consumers, "entry Consumers must not be nil for %s (use empty slice)", eventType)
		})
	}
}

// TestNoDuplicateEventStrings checks that no two EventType constants resolve
// to the same string value. This catches copy-paste errors where a new
// constant accidentally duplicates an existing event string.
func TestNoDuplicateEventStrings(t *testing.T) {
	allTypes := collectAllEventTypes()
	seen := make(map[domainevent.EventType]bool, len(allTypes))

	for _, et := range allTypes {
		if seen[et] {
			t.Errorf("duplicate event type string %q found in collectAllEventTypes", et)
		}
		seen[et] = true
	}
}

// collectAllEventTypes returns every EventType constant from all bounded
// contexts. This is the authoritative list of all domain events in RemnaCore.
//
// When adding a new event type:
//  1. Define the EventType constant in the appropriate domain package
//  2. Add it to this list
//  3. Register it in internal/app/event_catalog.go via BuildEventCatalog
//  4. Update totalExpectedEvents
func collectAllEventTypes() []domainevent.EventType {
	return []domainevent.EventType{
		// =====================================================================
		// Billing — Subscription events (13)
		// =====================================================================
		billingaggregate.EventSubCreated,
		billingaggregate.EventSubActivated,
		billingaggregate.EventSubCancelled,
		billingaggregate.EventSubRenewed,
		billingaggregate.EventSubPaused,
		billingaggregate.EventSubResumed,
		billingaggregate.EventSubExpired,
		billingaggregate.EventSubPastDue,
		billingaggregate.EventSubUpgraded,
		billingaggregate.EventSubDowngraded,
		billingaggregate.EventSubTrialStarted,
		billingaggregate.EventSubTrialEnding,
		billingaggregate.EventSubUpdated,

		// =====================================================================
		// Billing — Invoice events (6)
		// =====================================================================
		billingaggregate.EventInvCreated,
		billingaggregate.EventInvPaid,
		billingaggregate.EventInvFailed,
		billingaggregate.EventInvRefunded,
		billingaggregate.EventInvDiscountApplied,
		billingaggregate.EventInvTotalOverridden,

		// =====================================================================
		// Billing — Family events (3)
		// =====================================================================
		billingaggregate.EventFamilyCreated,
		billingaggregate.EventFamilyMemberAdded,
		billingaggregate.EventFamilyMemberRemoved,

		// =====================================================================
		// Billing — Plan events (3)
		// =====================================================================
		billingaggregate.EventPlanCreated,
		billingaggregate.EventPlanUpdated,
		billingaggregate.EventPlanDeactivated,

		// =====================================================================
		// MultiSub — Binding events (10)
		// =====================================================================
		multisubaggregate.EventBindingProvisioned,
		multisubaggregate.EventBindingDeprovisioned,
		multisubaggregate.EventBindingSyncFailed,
		multisubaggregate.EventBindingSyncCompleted,
		multisubaggregate.EventBindingTrafficExceeded,
		multisubaggregate.EventBindingLimited,
		multisubaggregate.EventBindingUnlimited,
		multisubaggregate.EventBindingDisabled,
		multisubaggregate.EventBindingEnabled,
		multisubaggregate.EventBindingFailed,

		// =====================================================================
		// Payment events (5)
		// =====================================================================
		payment.EventChargeCreated,
		payment.EventChargeCompleted,
		payment.EventChargeFailed,
		payment.EventRefundCompleted,
		payment.EventWebhookReceived,

		// =====================================================================
		// Identity events (10)
		// =====================================================================
		identity.EventUserRegistered,
		identity.EventEmailVerified,
		identity.EventUserLoggedIn,
		identity.EventProfileUpdated,
		identity.EventTokenRefreshed,
		identity.EventPasswordResetRequested,
		identity.EventPasswordReset,
		identity.EventPasswordChanged,
		identity.EventTelegramLinked,
		identity.EventTelegramUnlinked,

		// =====================================================================
		// Reseller events (5)
		// =====================================================================
		reseller.EventTenantCreated,
		reseller.EventTenantUpdated,
		reseller.EventResellerCreated,
		reseller.EventCommissionCreated,
		reseller.EventCommissionPaid,

		// =====================================================================
		// Plugin events (8)
		// =====================================================================
		plugin.EventPluginInstalled,
		plugin.EventPluginEnabled,
		plugin.EventPluginDisabled,
		plugin.EventPluginUninstalled,
		plugin.EventPluginHotReloaded,
		plugin.EventPluginError,
		plugin.EventHookExecuted,
		plugin.EventHookFailed,

		// =====================================================================
		// Infra events (1)
		// =====================================================================
		health.EventNodeHealthChanged,
	}
}

// multsubConsumerName is the consumer name used in the event catalog for the
// multisub orchestrator, which is the target of the BillingEventConsumer.
const multisubConsumerName = "multisub"

// TestBillingConsumerSubscribesToAllDeclaredEvents verifies that every event in
// the catalog that declares "multisub" as a consumer has a corresponding NATS
// subject in BillingSubscriptionSubjects. This catches the case where a new
// event is added to the catalog with multisub as consumer but the consumer's
// subject list is not updated.
func TestBillingConsumerSubscribesToAllDeclaredEvents(t *testing.T) {
	catalog := app.BuildEventCatalog()
	subscribed := natsadapter.BillingSubscriptionSubjects()

	for eventType, entry := range catalog.All() {
		if slices.Contains(entry.Consumers, multisubConsumerName) {
			t.Run(string(eventType), func(t *testing.T) {
				assert.True(t, slices.Contains(subscribed, string(entry.Type)),
					"event %s declares %s as consumer but is not in BillingSubscriptionSubjects",
					entry.Type, multisubConsumerName)
			})
		}
	}
}

// TestBillingConsumerSubjectsInCatalog verifies the reverse: every subject the
// BillingEventConsumer subscribes to that is a registered event type in the
// catalog must declare a consumer. Webhook-originated subjects (e.g.,
// subscription.traffic_cycle_reset) that are not formal domain events are
// excluded — they come from the Remnawave ACL and have no catalog entry.
func TestBillingConsumerSubjectsInCatalog(t *testing.T) {
	catalog := app.BuildEventCatalog()
	subscribed := natsadapter.BillingSubscriptionSubjects()

	for _, subject := range subscribed {
		entry, ok := catalog.Get(domainevent.EventType(subject))
		if !ok {
			// Subject is a webhook-originated NATS subject without a formal
			// catalog entry (e.g., subscription.traffic_cycle_reset). Skip.
			continue
		}
		t.Run(subject, func(t *testing.T) {
			assert.NotEmpty(t, entry.Consumers,
				"BillingSubscriptionSubjects contains %q which has no consumers in the catalog",
				subject)
		})
	}
}
