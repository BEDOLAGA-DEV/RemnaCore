package reseller

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// TenantCreatedPayload is the typed payload for EventTenantCreated.
type TenantCreatedPayload struct {
	TenantID    string `json:"tenant_id"`
	OwnerUserID string `json:"owner_user_id"`
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

// --- EventPayload interface implementations ---

func (TenantCreatedPayload) EventType() domainevent.EventType     { return EventTenantCreated }
func (ResellerCreatedPayload) EventType() domainevent.EventType   { return EventResellerCreated }
func (CommissionCreatedPayload) EventType() domainevent.EventType { return EventCommissionCreated }

// Compile-time interface check.
var _ domainevent.EventPayload = TenantCreatedPayload{}
