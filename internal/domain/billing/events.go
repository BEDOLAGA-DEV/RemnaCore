package billing

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// Billing-specific event types re-exported from aggregate for backward
// compatibility. New code should prefer the aggregate constants directly.
const (
	EventInvoiceCreated      = aggregate.EventInvCreated
	EventInvoicePaid         = aggregate.EventInvPaid
	EventInvoiceFailed       = aggregate.EventInvFailed
	EventInvoiceRefunded     = aggregate.EventInvRefunded
	EventSubCreated          = aggregate.EventSubCreated
	EventSubActivated        = aggregate.EventSubActivated
	EventSubCancelled        = aggregate.EventSubCancelled
	EventSubRenewed          = aggregate.EventSubRenewed
	EventSubPastDue          = aggregate.EventSubPastDue
	EventSubExpired          = aggregate.EventSubExpired
	EventSubUpgraded         = aggregate.EventSubUpgraded
	EventSubDowngraded       = aggregate.EventSubDowngraded
	EventSubTrialStarted     = aggregate.EventSubTrialStarted
	EventSubTrialEnding      = aggregate.EventSubTrialEnding
	EventSubPaused           = aggregate.EventSubPaused
	EventSubResumed          = aggregate.EventSubResumed
	EventFamilyMemberAdded   = aggregate.EventFamilyMemberAdded
	EventFamilyMemberRemoved = aggregate.EventFamilyMemberRemoved
	EventPlanCreated         = aggregate.EventPlanCreated
	EventPlanUpdated         = aggregate.EventPlanUpdated
	EventPlanDeactivated     = aggregate.EventPlanDeactivated
)

// Event is an alias for the shared domainevent.Event so that callers within the
// billing context can reference billing.Event without importing pkg/domainevent.
type Event = domainevent.Event

// EventType is an alias for the shared domainevent.EventType.
type EventType = domainevent.EventType

// Payload type aliases re-exported from aggregate for backward compatibility.
type (
	InvoiceCreatedPayload  = aggregate.InvCreatedPayload
	InvoicePaidPayload     = aggregate.InvPaidPayload
	InvoiceFailedPayload   = aggregate.InvFailedPayload
	InvoiceRefundedPayload = aggregate.InvRefundedPayload

	SubCreatedPayload      = aggregate.SubCreatedPayload
	SubActivatedPayload    = aggregate.SubActivatedPayload
	SubCancelledPayload    = aggregate.SubCancelledPayload
	SubRenewedPayload      = aggregate.SubRenewedPayload
	SubPastDuePayload      = aggregate.SubPastDuePayload
	SubExpiredPayload      = aggregate.SubExpiredPayload
	SubUpgradedPayload     = aggregate.SubUpgradedPayload
	SubDowngradedPayload   = aggregate.SubDowngradedPayload
	SubTrialStartedPayload = aggregate.SubTrialStartedPayload
	SubTrialEndingPayload  = aggregate.SubTrialEndingPayload
	SubPausedPayload       = aggregate.SubPausedPayload
	SubResumedPayload      = aggregate.SubResumedPayload

	FamilyMemberAddedPayload   = aggregate.FamilyMemberAddedPayload
	FamilyMemberRemovedPayload = aggregate.FamilyMemberRemovedPayload

	PlanCreatedPayload     = aggregate.PlanCreatedPayload
	PlanUpdatedPayload     = aggregate.PlanUpdatedPayload
	PlanDeactivatedPayload = aggregate.PlanDeactivatedPayload
)
