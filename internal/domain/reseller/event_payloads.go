package reseller

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
