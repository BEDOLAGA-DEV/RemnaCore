package service

import (
	"errors"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/apierror"
)

// IAM sentinel errors for IdentityAdminService operations.
var (
	// ErrGrantNotAllowed is the service-layer alias for rbac.ErrGrantNotAllowed.
	// The service may return either this or rbac.ErrGrantNotAllowed directly from
	// CanGrant — MapIAMError catches both.
	ErrGrantNotAllowed    = errors.New("grant not allowed")
	ErrNotPlatformAdmin   = errors.New("platform admin required")
	ErrLastAdmin          = errors.New("cannot revoke the last platform admin")
	ErrInvitationExpired  = errors.New("invitation expired")
	ErrInvitationNotFound = errors.New("invitation not found")
	ErrEmailAlreadyUser   = errors.New("email already belongs to an existing user")
	// ErrBindingNotFound is returned by RevokeRole when no matching binding exists.
	ErrBindingNotFound = errors.New("role binding not found")
	// ErrOwnerNotSpecified is returned by CreateShop when neither ExistingUserID
	// nor InviteEmail is set in OwnerSpec.
	ErrOwnerNotSpecified = errors.New("shop owner not specified")
	// ErrOwnerAmbiguous is returned by CreateShop when both ExistingUserID and
	// InviteEmail are set in OwnerSpec.
	ErrOwnerAmbiguous = errors.New("shop owner over-specified")
)

// MapIAMError maps IdentityAdminService errors to API error codes.
// Returns nil if the error does not belong to the IAM domain.
// Also catches rbac.ErrGrantNotAllowed returned directly from CanGrant.
func MapIAMError(err error) *apierror.Error {
	switch {
	case errors.Is(err, ErrGrantNotAllowed), errors.Is(err, rbac.ErrGrantNotAllowed):
		return apierror.IAMGrantNotAllowed
	case errors.Is(err, ErrNotPlatformAdmin):
		return apierror.IAMNotPlatformAdmin
	case errors.Is(err, ErrLastAdmin):
		return apierror.IAMLastAdmin
	case errors.Is(err, ErrInvitationExpired):
		return apierror.IAMInvitationExpired
	case errors.Is(err, ErrInvitationNotFound):
		return apierror.IAMInvitationNotFound
	case errors.Is(err, ErrEmailAlreadyUser):
		return apierror.IAMEmailAlreadyUser
	case errors.Is(err, ErrBindingNotFound):
		return apierror.IAMBindingNotFound
	case errors.Is(err, ErrOwnerNotSpecified):
		return apierror.IAMOwnerNotSpecified
	case errors.Is(err, ErrOwnerAmbiguous):
		return apierror.IAMOwnerAmbiguous
	default:
		return nil
	}
}
