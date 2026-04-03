package reseller

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Reseller-specific event types.
const (
	EventTenantCreated     domainevent.EventType = "reseller.tenant_created"
	EventTenantUpdated     domainevent.EventType = "reseller.tenant_updated"
	EventResellerCreated   domainevent.EventType = "reseller.account_created"
	EventCommissionCreated domainevent.EventType = "reseller.commission_created"
	EventCommissionPaid    domainevent.EventType = "reseller.commission_paid"
)

// NewTenantCreatedEvent creates an event for a newly created tenant.
func NewTenantCreatedEvent(tenantID, ownerUserID string) domainevent.Event {
	return domainevent.New(EventTenantCreated, map[string]any{
		"tenant_id":     tenantID,
		"owner_user_id": ownerUserID,
	})
}

// NewResellerCreatedEvent creates an event for a newly created reseller account.
func NewResellerCreatedEvent(resellerID, tenantID, userID string) domainevent.Event {
	return domainevent.New(EventResellerCreated, map[string]any{
		"reseller_id": resellerID,
		"tenant_id":   tenantID,
		"user_id":     userID,
	})
}

// NewCommissionCreatedEvent creates an event for a newly recorded commission.
func NewCommissionCreatedEvent(commissionID, resellerID string, amount int64) domainevent.Event {
	return domainevent.New(EventCommissionCreated, map[string]any{
		"commission_id": commissionID,
		"reseller_id":   resellerID,
		"amount":        amount,
	})
}
