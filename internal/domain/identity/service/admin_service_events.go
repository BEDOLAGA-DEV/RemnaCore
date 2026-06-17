package service

import (
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// NewUserInvitedEvent creates an event when a user is invited by an admin or
// shop owner. The aggregate entity ID is the invitation ID.
func NewUserInvitedEvent(inv *aggregate.Invitation, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.UserInvitedPayload{
		InvitationID: inv.ID,
		Email:        inv.Email,
		RoleKey:      inv.RoleKey,
		TenantID:     inv.TenantID,
	}, now, inv.ID)
}

// NewUserInvitationAcceptedEvent creates an event when an invited user accepts
// their invitation and sets a password. The aggregate entity ID is the user ID.
func NewUserInvitationAcceptedEvent(userID, email string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.UserInvitationAcceptedPayload{
		UserID: userID,
		Email:  email,
	}, now, userID)
}

// NewRoleAssignedEvent creates an event when a role is granted to a user.
// The aggregate entity ID is the user ID.
func NewRoleAssignedEvent(userID, roleKey string, tenantID *string, grantedBy string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.RoleAssignedPayload{
		UserID:    userID,
		RoleKey:   roleKey,
		TenantID:  tenantID,
		GrantedBy: grantedBy,
	}, now, userID)
}

// NewRoleRevokedEvent creates an event when a role is revoked from a user.
// The aggregate entity ID is the user ID.
func NewRoleRevokedEvent(userID, roleKey string, tenantID *string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.RoleRevokedPayload{
		UserID:   userID,
		RoleKey:  roleKey,
		TenantID: tenantID,
	}, now, userID)
}

// NewShopCreatedEvent creates an event when a new shop (tenant) is provisioned.
// The aggregate entity ID is the tenant ID.
func NewShopCreatedEvent(tenantID string, ownerUserID *string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.ShopCreatedPayload{
		TenantID:    tenantID,
		OwnerUserID: ownerUserID,
	}, now, tenantID)
}
