package billing

// InvoiceCreatedPayload is the typed payload for EventInvoiceCreated.
type InvoiceCreatedPayload struct {
	InvoiceID      string `json:"invoice_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	AmountCents    int64  `json:"amount_cents"`
}

// InvoicePaidPayload is the typed payload for EventInvoicePaid.
type InvoicePaidPayload struct {
	InvoiceID      string `json:"invoice_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	AmountCents    int64  `json:"amount_cents"`
}

// InvoiceFailedPayload is the typed payload for EventInvoiceFailed.
type InvoiceFailedPayload struct {
	InvoiceID      string `json:"invoice_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	Reason         string `json:"reason"`
}

// InvoiceRefundedPayload is the typed payload for EventInvoiceRefunded.
type InvoiceRefundedPayload struct {
	InvoiceID      string `json:"invoice_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	AmountCents    int64  `json:"amount_cents"`
}

// SubCreatedPayload is the typed payload for EventSubCreated.
type SubCreatedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
}

// SubActivatedPayload is the typed payload for EventSubActivated.
type SubActivatedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
}

// SubCancelledPayload is the typed payload for EventSubCancelled.
type SubCancelledPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	Reason         string `json:"reason,omitempty"`
}

// SubRenewedPayload is the typed payload for EventSubRenewed.
type SubRenewedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
}

// SubUpgradedPayload is the typed payload for EventSubUpgraded.
type SubUpgradedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	FromPlanID     string `json:"from_plan_id"`
	ToPlanID       string `json:"to_plan_id"`
}

// SubDowngradedPayload is the typed payload for EventSubDowngraded.
type SubDowngradedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	FromPlanID     string `json:"from_plan_id"`
	ToPlanID       string `json:"to_plan_id"`
}

// SubTrialStartedPayload is the typed payload for EventSubTrialStarted.
type SubTrialStartedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	PlanID         string `json:"plan_id"`
}

// SubTrialEndingPayload is the typed payload for EventSubTrialEnding.
type SubTrialEndingPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	DaysRemaining  int    `json:"days_remaining"`
}

// SubPausedPayload is the typed payload for EventSubPaused.
type SubPausedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
}

// SubResumedPayload is the typed payload for EventSubResumed.
type SubResumedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
}

// FamilyMemberAddedPayload is the typed payload for EventFamilyMemberAdded.
type FamilyMemberAddedPayload struct {
	FamilyGroupID string `json:"family_group_id"`
	OwnerID       string `json:"owner_id"`
	MemberID      string `json:"member_id"`
}

// FamilyMemberRemovedPayload is the typed payload for EventFamilyMemberRemoved.
type FamilyMemberRemovedPayload struct {
	FamilyGroupID string `json:"family_group_id"`
	OwnerID       string `json:"owner_id"`
	MemberID      string `json:"member_id"`
}
