package archtest

import (
	"testing"

	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	multisubaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests detect drift between Go status constants and the PostgreSQL
// CHECK constraints defined in migrations. If a status is added in Go but
// not in the migration (or vice-versa), the mismatch surfaces here before
// it causes a runtime constraint violation.

// TestBindingStatusesMatchDBSchema verifies that Go BindingStatus constants
// match the CHECK constraint in migration 027_schema_fixes.sql.
func TestBindingStatusesMatchDBSchema(t *testing.T) {
	// Must match: CHECK (status IN ('pending','active','failed','disabled',
	//   'deprovisioned','reconciling','limited'))
	expected := []string{
		"pending", "active", "failed", "disabled",
		"deprovisioned", "reconciling", "limited",
	}
	actual := toStrings(multisubaggregate.AllBindingStatuses())
	assert.ElementsMatch(t, expected, actual,
		"binding statuses in Go must match CHECK constraint in migration 027")
}

// TestSubscriptionStatusesMatchDBSchema verifies that Go SubscriptionStatus
// constants match the expected DB enum values.
func TestSubscriptionStatusesMatchDBSchema(t *testing.T) {
	expected := []string{
		"trial", "active", "past_due",
		"cancelled", "expired", "paused",
	}
	actual := toStrings(billingaggregate.AllSubscriptionStatuses())
	assert.ElementsMatch(t, expected, actual,
		"subscription statuses in Go must match DB CHECK constraint")
}

// TestInvoiceStatusesMatchDBSchema verifies that Go InvoiceStatus constants
// match the expected DB enum values.
func TestInvoiceStatusesMatchDBSchema(t *testing.T) {
	expected := []string{
		"draft", "pending", "paid",
		"failed", "refunded",
	}
	actual := toStrings(billingaggregate.AllInvoiceStatuses())
	assert.ElementsMatch(t, expected, actual,
		"invoice statuses in Go must match DB CHECK constraint")
}

// TestPaymentStatusesMatchDBSchema verifies that Go PaymentStatus constants
// match the expected DB enum values.
func TestPaymentStatusesMatchDBSchema(t *testing.T) {
	expected := []string{
		"pending", "completed", "failed", "refunded",
	}
	actual := toStrings(payment.AllPaymentStatuses())
	assert.ElementsMatch(t, expected, actual,
		"payment statuses in Go must match DB CHECK constraint")
}

// TestAllPaymentStatusesHaveTransitions verifies that every payment status
// has an entry in the transition map.
func TestAllPaymentStatusesHaveTransitions(t *testing.T) {
	err := payment.ValidatePaymentTransitions()
	require.NoError(t, err)
}

// toStrings converts a slice of any ~string type to []string using generics.
func toStrings[S ~string](ss []S) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = string(s)
	}
	return out
}
