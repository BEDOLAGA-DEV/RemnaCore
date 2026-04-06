package reseller

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Reseller-specific event types.
const (
	EventTenantCreated     domainevent.EventType = "reseller.tenant_created"
	EventTenantUpdated     domainevent.EventType = "reseller.tenant_updated"
	EventResellerCreated   domainevent.EventType = "reseller.account_created"
	EventCommissionCreated domainevent.EventType = "reseller.commission_created"
	// EventCommissionPaid is published when a commission is marked as paid.
	EventCommissionPaid domainevent.EventType = "reseller.commission_paid"
)
