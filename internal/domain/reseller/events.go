package reseller

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Reseller-specific event types.
const (
	EventTenantCreated     domainevent.EventType = "reseller.tenant_created"
	// EventTenantUpdated is reserved for future use.
	EventTenantUpdated domainevent.EventType = "reseller.tenant_updated"
	EventResellerCreated   domainevent.EventType = "reseller.account_created"
	EventCommissionCreated domainevent.EventType = "reseller.commission_created"
	// EventCommissionPaid is reserved for future use.
	EventCommissionPaid domainevent.EventType = "reseller.commission_paid"
)

// NewTenantCreatedEvent creates an event for a newly created tenant.
func NewTenantCreatedEvent(tenantID, ownerUserID string) domainevent.Event {
	return domainevent.NewWithEntity(EventTenantCreated, TenantCreatedPayload{
		TenantID:    tenantID,
		OwnerUserID: ownerUserID,
	}, tenantID)
}

// NewResellerCreatedEvent creates an event for a newly created reseller account.
func NewResellerCreatedEvent(resellerID, tenantID, userID string) domainevent.Event {
	return domainevent.NewWithEntity(EventResellerCreated, ResellerCreatedPayload{
		ResellerID: resellerID,
		TenantID:   tenantID,
		UserID:     userID,
	}, resellerID)
}

// NewCommissionCreatedEvent creates an event for a newly recorded commission.
func NewCommissionCreatedEvent(commissionID, resellerID string, amount int64) domainevent.Event {
	return domainevent.NewWithEntity(EventCommissionCreated, CommissionCreatedPayload{
		CommissionID: commissionID,
		ResellerID:   resellerID,
		Amount:       amount,
	}, commissionID)
}
