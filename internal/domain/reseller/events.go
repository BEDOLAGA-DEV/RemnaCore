package reseller

import (
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

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
func NewTenantCreatedEvent(tenantID, ownerUserID string, now time.Time) domainevent.Event {
	return domainevent.NewAtWithEntity(EventTenantCreated, TenantCreatedPayload{
		TenantID:    tenantID,
		OwnerUserID: ownerUserID,
	}, now, tenantID)
}

// NewResellerCreatedEvent creates an event for a newly created reseller account.
func NewResellerCreatedEvent(resellerID, tenantID, userID string, now time.Time) domainevent.Event {
	return domainevent.NewAtWithEntity(EventResellerCreated, ResellerCreatedPayload{
		ResellerID: resellerID,
		TenantID:   tenantID,
		UserID:     userID,
	}, now, resellerID)
}

// NewCommissionCreatedEvent creates an event for a newly recorded commission.
func NewCommissionCreatedEvent(commissionID, resellerID string, amount int64, now time.Time) domainevent.Event {
	return domainevent.NewAtWithEntity(EventCommissionCreated, CommissionCreatedPayload{
		CommissionID: commissionID,
		ResellerID:   resellerID,
		Amount:       amount,
	}, now, commissionID)
}
