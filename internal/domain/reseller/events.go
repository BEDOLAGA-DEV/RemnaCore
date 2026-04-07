package reseller

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// Reseller-specific event types re-exported from aggregate for backward
// compatibility. New code should prefer the aggregate constants directly.
const (
	EventTenantCreated     = aggregate.EventTenantCreated
	EventTenantUpdated     = aggregate.EventTenantUpdated
	EventResellerCreated   = aggregate.EventResellerCreated
	EventCommissionCreated = aggregate.EventCommissionCreated
	EventCommissionPaid    = aggregate.EventCommissionPaid
)

// Payload type aliases re-exported from aggregate for backward compatibility.
type (
	TenantCreatedPayload     = aggregate.TenantCreatedPayload
	TenantUpdatedPayload     = aggregate.TenantUpdatedPayload
	ResellerCreatedPayload   = aggregate.ResellerCreatedPayload
	CommissionCreatedPayload = aggregate.CommissionCreatedPayload
	CommissionPaidPayload    = aggregate.CommissionPaidPayload
)

// Event is an alias for the shared domainevent.Event so that callers within the
// reseller context can reference reseller.Event without importing pkg/domainevent.
type Event = domainevent.Event

// EventType is an alias for the shared domainevent.EventType.
type EventType = domainevent.EventType
