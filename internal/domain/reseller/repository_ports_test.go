package reseller_test

import (
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
)

// TestNewPortsExist pins the C4 domain surface: the value types and the
// CustomerRepository port must exist with the expected shape.
func TestNewPortsExist(t *testing.T) {
	var _ reseller.CustomerRepository
	cs := reseller.CustomerSummary{
		UserID:          "u1",
		Email:           "c@example.com",
		ActiveSubsCount: 2,
	}
	if cs.UserID != "u1" {
		t.Fatalf("CustomerSummary.UserID round-trip failed")
	}
	ds := reseller.DashboardSummary{
		ActiveCustomers:     1,
		ActiveSubscriptions: 3,
		PendingCommission:   4200,
		Currency:            "usd",
	}
	if ds.ActiveSubscriptions != 3 {
		t.Fatalf("DashboardSummary round-trip failed")
	}
}
