package billing

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Billing-specific event types.
const (
	EventInvoiceCreated      domainevent.EventType = "invoice.created"
	EventInvoicePaid         domainevent.EventType = "invoice.paid"
	EventInvoiceFailed       domainevent.EventType = "invoice.failed"
	EventInvoiceRefunded     domainevent.EventType = "invoice.refunded"
	EventSubCreated          domainevent.EventType = "subscription.created"
	EventSubActivated        domainevent.EventType = "subscription.activated"
	EventSubCancelled        domainevent.EventType = "subscription.cancelled"
	EventSubRenewed          domainevent.EventType = "subscription.renewed"
	EventSubUpgraded         domainevent.EventType = "subscription.upgraded"
	EventSubDowngraded       domainevent.EventType = "subscription.downgraded"
	EventSubTrialStarted     domainevent.EventType = "subscription.trial_started"
	EventSubTrialEnding      domainevent.EventType = "subscription.trial_ending"
	EventSubPaused           domainevent.EventType = "subscription.paused"
	EventSubResumed          domainevent.EventType = "subscription.resumed"
	EventFamilyMemberAdded   domainevent.EventType = "family.member_added"
	EventFamilyMemberRemoved domainevent.EventType = "family.member_removed"
)

// Event is an alias for the shared domainevent.Event so that callers within the
// billing context can reference billing.Event without importing pkg/domainevent.
type Event = domainevent.Event

// EventType is an alias for the shared domainevent.EventType.
type EventType = domainevent.EventType

// --- Invoice event factories ---

// NewInvoiceCreatedEvent creates an event for a newly generated invoice.
func NewInvoiceCreatedEvent(invoiceID, subscriptionID, userID string, amountCents int64) Event {
	return domainevent.New(EventInvoiceCreated, InvoiceCreatedPayload{
		InvoiceID:      invoiceID,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		AmountCents:    amountCents,
	})
}

// NewInvoicePaidEvent creates an event for a successfully paid invoice.
func NewInvoicePaidEvent(invoiceID, subscriptionID, userID string, amountCents int64) Event {
	return domainevent.New(EventInvoicePaid, InvoicePaidPayload{
		InvoiceID:      invoiceID,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		AmountCents:    amountCents,
	})
}

// NewInvoiceFailedEvent creates an event for a failed invoice payment.
func NewInvoiceFailedEvent(invoiceID, subscriptionID, userID, reason string) Event {
	return domainevent.New(EventInvoiceFailed, InvoiceFailedPayload{
		InvoiceID:      invoiceID,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Reason:         reason,
	})
}

// NewInvoiceRefundedEvent creates an event for a refunded invoice.
func NewInvoiceRefundedEvent(invoiceID, subscriptionID, userID string, amountCents int64) Event {
	return domainevent.New(EventInvoiceRefunded, InvoiceRefundedPayload{
		InvoiceID:      invoiceID,
		SubscriptionID: subscriptionID,
		UserID:         userID,
		AmountCents:    amountCents,
	})
}

// --- Subscription event factories ---

// NewSubCreatedEvent creates an event for a newly created subscription.
func NewSubCreatedEvent(subscriptionID, userID, planID string) Event {
	return domainevent.New(EventSubCreated, SubCreatedPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		PlanID:         planID,
	})
}

// NewSubActivatedEvent creates an event for a subscription activation.
func NewSubActivatedEvent(subscriptionID, userID string) Event {
	return domainevent.New(EventSubActivated, SubActivatedPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
	})
}

// NewSubCancelledEvent creates an event for a subscription cancellation.
func NewSubCancelledEvent(subscriptionID, userID, reason string) Event {
	return domainevent.New(EventSubCancelled, SubCancelledPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Reason:         reason,
	})
}

// NewSubRenewedEvent creates an event for a subscription renewal.
func NewSubRenewedEvent(subscriptionID, userID string) Event {
	return domainevent.New(EventSubRenewed, SubRenewedPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
	})
}

// NewSubUpgradedEvent creates an event for a subscription plan upgrade.
func NewSubUpgradedEvent(subscriptionID, userID, fromPlanID, toPlanID string) Event {
	return domainevent.New(EventSubUpgraded, SubUpgradedPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		FromPlanID:     fromPlanID,
		ToPlanID:       toPlanID,
	})
}

// NewSubDowngradedEvent creates an event for a subscription plan downgrade.
func NewSubDowngradedEvent(subscriptionID, userID, fromPlanID, toPlanID string) Event {
	return domainevent.New(EventSubDowngraded, SubDowngradedPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		FromPlanID:     fromPlanID,
		ToPlanID:       toPlanID,
	})
}

// NewSubTrialStartedEvent creates an event for a trial period start.
func NewSubTrialStartedEvent(subscriptionID, userID, planID string) Event {
	return domainevent.New(EventSubTrialStarted, SubTrialStartedPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		PlanID:         planID,
	})
}

// NewSubTrialEndingEvent creates an event for a trial about to expire.
func NewSubTrialEndingEvent(subscriptionID, userID string, daysRemaining int) Event {
	return domainevent.New(EventSubTrialEnding, SubTrialEndingPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		DaysRemaining:  daysRemaining,
	})
}

// NewSubPausedEvent creates an event for a paused subscription.
func NewSubPausedEvent(subscriptionID, userID string) Event {
	return domainevent.New(EventSubPaused, SubPausedPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
	})
}

// NewSubResumedEvent creates an event for a resumed subscription.
func NewSubResumedEvent(subscriptionID, userID string) Event {
	return domainevent.New(EventSubResumed, SubResumedPayload{
		SubscriptionID: subscriptionID,
		UserID:         userID,
	})
}

// --- Family event factories ---

// NewFamilyMemberAddedEvent creates an event for adding a family member.
func NewFamilyMemberAddedEvent(familyGroupID, ownerID, memberID string) Event {
	return domainevent.New(EventFamilyMemberAdded, FamilyMemberAddedPayload{
		FamilyGroupID: familyGroupID,
		OwnerID:       ownerID,
		MemberID:      memberID,
	})
}

// NewFamilyMemberRemovedEvent creates an event for removing a family member.
func NewFamilyMemberRemovedEvent(familyGroupID, ownerID, memberID string) Event {
	return domainevent.New(EventFamilyMemberRemoved, FamilyMemberRemovedPayload{
		FamilyGroupID: familyGroupID,
		OwnerID:       ownerID,
		MemberID:      memberID,
	})
}
