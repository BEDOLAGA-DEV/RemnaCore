package aggregate

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

// --- Event payload types ---

// TenantCreatedPayload is the typed payload for EventTenantCreated.
type TenantCreatedPayload struct {
	TenantID    string  `json:"tenant_id"`
	OwnerUserID *string `json:"owner_user_id"`
}

// TenantUpdatedPayload is the typed payload for EventTenantUpdated.
type TenantUpdatedPayload struct {
	TenantID string `json:"tenant_id"`
}

// ResellerCreatedPayload is the typed payload for EventResellerCreated.
type ResellerCreatedPayload struct {
	ResellerID string `json:"reseller_id"`
	TenantID   string `json:"tenant_id"`
	UserID     string `json:"user_id"`
}

// CommissionCreatedPayload is the typed payload for EventCommissionCreated.
type CommissionCreatedPayload struct {
	CommissionID string `json:"commission_id"`
	ResellerID   string `json:"reseller_id"`
	Amount       int64  `json:"amount"`
}

// CommissionPaidPayload is the typed payload for EventCommissionPaid.
type CommissionPaidPayload struct {
	CommissionID string `json:"commission_id"`
	ResellerID   string `json:"reseller_id"`
	Amount       int64  `json:"amount"`
}

// --- EventPayload interface implementations ---

func (TenantCreatedPayload) EventType() domainevent.EventType     { return EventTenantCreated }
func (TenantUpdatedPayload) EventType() domainevent.EventType     { return EventTenantUpdated }
func (ResellerCreatedPayload) EventType() domainevent.EventType   { return EventResellerCreated }
func (CommissionCreatedPayload) EventType() domainevent.EventType { return EventCommissionCreated }
func (CommissionPaidPayload) EventType() domainevent.EventType    { return EventCommissionPaid }

// Compile-time interface checks.
var (
	_ domainevent.EventPayload = TenantCreatedPayload{}
	_ domainevent.EventPayload = TenantUpdatedPayload{}
	_ domainevent.EventPayload = ResellerCreatedPayload{}
	_ domainevent.EventPayload = CommissionCreatedPayload{}
	_ domainevent.EventPayload = CommissionPaidPayload{}
)
