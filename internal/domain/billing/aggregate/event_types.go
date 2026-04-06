package aggregate

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Event type constants for billing aggregates. Defined in the aggregate package
// so that aggregate methods can record events without circular imports.
const (
	EventSubCreated      domainevent.EventType = "subscription.created"
	EventSubActivated    domainevent.EventType = "subscription.activated"
	EventSubCancelled    domainevent.EventType = "subscription.cancelled"
	EventSubRenewed      domainevent.EventType = "subscription.renewed"
	EventSubPaused       domainevent.EventType = "subscription.paused"
	EventSubResumed      domainevent.EventType = "subscription.resumed"
	EventSubExpired      domainevent.EventType = "subscription.expired"
	EventSubPastDue      domainevent.EventType = "subscription.past_due"
	EventSubUpgraded     domainevent.EventType = "subscription.upgraded"
	EventSubDowngraded   domainevent.EventType = "subscription.downgraded"
	EventSubTrialStarted domainevent.EventType = "subscription.trial_started"
	EventSubTrialEnding  domainevent.EventType = "subscription.trial_ending"
	EventSubUpdated      domainevent.EventType = "subscription.updated"

	EventInvCreated          domainevent.EventType = "invoice.created"
	EventInvPaid             domainevent.EventType = "invoice.paid"
	EventInvFailed           domainevent.EventType = "invoice.failed"
	EventInvRefunded         domainevent.EventType = "invoice.refunded"
	EventInvDiscountApplied  domainevent.EventType = "invoice.discount_applied"
	EventInvTotalOverridden  domainevent.EventType = "invoice.total_overridden"

	EventFamilyCreated       domainevent.EventType = "family.created"
	EventFamilyMemberAdded   domainevent.EventType = "family.member_added"
	EventFamilyMemberRemoved domainevent.EventType = "family.member_removed"

	EventPlanCreated     domainevent.EventType = "plan.created"
	EventPlanUpdated     domainevent.EventType = "plan.updated"
	EventPlanDeactivated domainevent.EventType = "plan.deactivated"
)

// --- Subscription event payloads ---

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

// SubExpiredPayload is the typed payload for EventSubExpired.
type SubExpiredPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
}

// SubPastDuePayload is the typed payload for EventSubPastDue.
type SubPastDuePayload struct {
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

// SubUpdatedPayload is the typed payload for EventSubUpdated.
type SubUpdatedPayload struct {
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
}

// --- Invoice event payloads ---

// InvCreatedPayload is the typed payload for EventInvCreated.
type InvCreatedPayload struct {
	InvoiceID      string `json:"invoice_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	AmountCents    int64  `json:"amount_cents"`
}

// InvPaidPayload is the typed payload for EventInvPaid.
type InvPaidPayload struct {
	InvoiceID      string `json:"invoice_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	AmountCents    int64  `json:"amount_cents"`
}

// InvFailedPayload is the typed payload for EventInvFailed.
type InvFailedPayload struct {
	InvoiceID      string `json:"invoice_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	Reason         string `json:"reason"`
}

// InvRefundedPayload is the typed payload for EventInvRefunded.
type InvRefundedPayload struct {
	InvoiceID      string `json:"invoice_id"`
	SubscriptionID string `json:"subscription_id"`
	UserID         string `json:"user_id"`
	AmountCents    int64  `json:"amount_cents"`
}

// InvDiscountAppliedPayload is the typed payload for EventInvDiscountApplied.
type InvDiscountAppliedPayload struct {
	InvoiceID      string `json:"invoice_id"`
	DiscountCode   string `json:"discount_code"`
	DiscountAmount int64  `json:"discount_amount"`
}

// InvTotalOverriddenPayload is the typed payload for EventInvTotalOverridden.
type InvTotalOverriddenPayload struct {
	InvoiceID string `json:"invoice_id"`
	OldAmount int64  `json:"old_amount"`
	NewAmount int64  `json:"new_amount"`
	Reason    string `json:"reason"`
}

// --- Family event payloads ---

// FamilyCreatedPayload is the typed payload for EventFamilyCreated.
type FamilyCreatedPayload struct {
	FamilyGroupID string `json:"family_group_id"`
	OwnerID       string `json:"owner_id"`
	MaxMembers    int    `json:"max_members"`
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

// --- Plan event payloads ---

// PlanCreatedPayload is the typed payload for EventPlanCreated.
type PlanCreatedPayload struct {
	PlanID string `json:"plan_id"`
	Name   string `json:"name"`
	Tier   string `json:"tier"`
}

// PlanUpdatedPayload is the typed payload for EventPlanUpdated.
type PlanUpdatedPayload struct {
	PlanID string `json:"plan_id"`
	Name   string `json:"name"`
}

// PlanDeactivatedPayload is the typed payload for EventPlanDeactivated.
type PlanDeactivatedPayload struct {
	PlanID string `json:"plan_id"`
}

// --- EventPayload interface implementations ---

func (SubCreatedPayload) EventType() domainevent.EventType        { return EventSubCreated }
func (SubActivatedPayload) EventType() domainevent.EventType      { return EventSubActivated }
func (SubCancelledPayload) EventType() domainevent.EventType      { return EventSubCancelled }
func (SubRenewedPayload) EventType() domainevent.EventType        { return EventSubRenewed }
func (SubPausedPayload) EventType() domainevent.EventType         { return EventSubPaused }
func (SubResumedPayload) EventType() domainevent.EventType        { return EventSubResumed }
func (SubExpiredPayload) EventType() domainevent.EventType        { return EventSubExpired }
func (SubPastDuePayload) EventType() domainevent.EventType        { return EventSubPastDue }
func (SubUpgradedPayload) EventType() domainevent.EventType       { return EventSubUpgraded }
func (SubDowngradedPayload) EventType() domainevent.EventType     { return EventSubDowngraded }
func (SubTrialStartedPayload) EventType() domainevent.EventType   { return EventSubTrialStarted }
func (SubTrialEndingPayload) EventType() domainevent.EventType    { return EventSubTrialEnding }
func (SubUpdatedPayload) EventType() domainevent.EventType        { return EventSubUpdated }
func (InvCreatedPayload) EventType() domainevent.EventType        { return EventInvCreated }
func (InvPaidPayload) EventType() domainevent.EventType           { return EventInvPaid }
func (InvFailedPayload) EventType() domainevent.EventType         { return EventInvFailed }
func (InvRefundedPayload) EventType() domainevent.EventType        { return EventInvRefunded }
func (InvDiscountAppliedPayload) EventType() domainevent.EventType { return EventInvDiscountApplied }
func (InvTotalOverriddenPayload) EventType() domainevent.EventType { return EventInvTotalOverridden }
func (FamilyCreatedPayload) EventType() domainevent.EventType     { return EventFamilyCreated }
func (FamilyMemberAddedPayload) EventType() domainevent.EventType { return EventFamilyMemberAdded }
func (FamilyMemberRemovedPayload) EventType() domainevent.EventType {
	return EventFamilyMemberRemoved
}
func (PlanCreatedPayload) EventType() domainevent.EventType     { return EventPlanCreated }
func (PlanUpdatedPayload) EventType() domainevent.EventType     { return EventPlanUpdated }
func (PlanDeactivatedPayload) EventType() domainevent.EventType { return EventPlanDeactivated }

// Compile-time interface checks.
var (
	_ domainevent.EventPayload = SubCreatedPayload{}
	_ domainevent.EventPayload = SubActivatedPayload{}
	_ domainevent.EventPayload = SubCancelledPayload{}
	_ domainevent.EventPayload = SubRenewedPayload{}
	_ domainevent.EventPayload = SubPausedPayload{}
	_ domainevent.EventPayload = SubResumedPayload{}
	_ domainevent.EventPayload = SubExpiredPayload{}
	_ domainevent.EventPayload = SubPastDuePayload{}
	_ domainevent.EventPayload = SubUpgradedPayload{}
	_ domainevent.EventPayload = SubDowngradedPayload{}
	_ domainevent.EventPayload = SubTrialStartedPayload{}
	_ domainevent.EventPayload = SubTrialEndingPayload{}
	_ domainevent.EventPayload = SubUpdatedPayload{}
	_ domainevent.EventPayload = InvCreatedPayload{}
	_ domainevent.EventPayload = InvPaidPayload{}
	_ domainevent.EventPayload = InvFailedPayload{}
	_ domainevent.EventPayload = InvRefundedPayload{}
	_ domainevent.EventPayload = InvDiscountAppliedPayload{}
	_ domainevent.EventPayload = InvTotalOverriddenPayload{}
	_ domainevent.EventPayload = FamilyCreatedPayload{}
	_ domainevent.EventPayload = FamilyMemberAddedPayload{}
	_ domainevent.EventPayload = FamilyMemberRemovedPayload{}
	_ domainevent.EventPayload = PlanCreatedPayload{}
	_ domainevent.EventPayload = PlanUpdatedPayload{}
	_ domainevent.EventPayload = PlanDeactivatedPayload{}
)
