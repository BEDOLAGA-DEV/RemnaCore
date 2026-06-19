package postgres

import (
	"strings"
	"testing"
)

// tenantPredicate is the explicit application-layer reaffirmation of the
// RLS policy on the by-id / by-external-id FOR UPDATE lookups (spec §5.3).
const tenantPredicate = "(tenant_id::text = current_setting('app.tenant_id', true) OR current_setting('app.tenant_id', true) = '*')"

func TestPaymentByIDForUpdate_HasTenantPredicate(t *testing.T) {
	if !strings.Contains(getPaymentRecordByIDForUpdateSQL, tenantPredicate) {
		t.Fatalf("getPaymentRecordByIDForUpdateSQL is missing the explicit tenant predicate %q:\n%s",
			tenantPredicate, getPaymentRecordByIDForUpdateSQL)
	}
}

func TestPaymentByExternalIDForUpdate_HasTenantPredicate(t *testing.T) {
	if !strings.Contains(getPaymentRecordByExternalIDForUpdateSQL, tenantPredicate) {
		t.Fatalf("getPaymentRecordByExternalIDForUpdateSQL is missing the explicit tenant predicate %q:\n%s",
			tenantPredicate, getPaymentRecordByExternalIDForUpdateSQL)
	}
}

func TestSubscriptionByIDForUpdate_HasTenantPredicate(t *testing.T) {
	if !strings.Contains(getSubscriptionByIDForUpdateGuardedSQL, tenantPredicate) {
		t.Fatalf("getSubscriptionByIDForUpdateGuardedSQL is missing the explicit tenant predicate %q:\n%s",
			tenantPredicate, getSubscriptionByIDForUpdateGuardedSQL)
	}
}
