package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// IdentityAdminService orchestrates account-management operations: inviting users
// by email, accepting invitations, assigning/revoking roles, direct-creating
// accounts, and provisioning shops. It is the Phase B counterpart to the
// read-path AccessService and the auth-path Service.
type IdentityAdminService struct {
	repo      Repository
	rbacRepo  rbac.Repository
	access    *AccessService
	shops     ShopProvisioner
	sessions  *SessionIssuer
	txRunner  txmanager.Runner
	publisher domainevent.Publisher
	clock     clock.Clock
}

// NewIdentityAdminService constructs an IdentityAdminService with all required
// dependencies injected (fx wires this in Task 12).
func NewIdentityAdminService(
	repo Repository,
	rbacRepo rbac.Repository,
	access *AccessService,
	shops ShopProvisioner,
	sessions *SessionIssuer,
	txRunner txmanager.Runner,
	pub domainevent.Publisher,
	clk clock.Clock,
) *IdentityAdminService {
	return &IdentityAdminService{
		repo:      repo,
		rbacRepo:  rbacRepo,
		access:    access,
		shops:     shops,
		sessions:  sessions,
		txRunner:  txRunner,
		publisher: pub,
		clock:     clk,
	}
}

// InviteInput is the input for InviteUser.
type InviteInput struct {
	Email          string
	RoleKey        string
	TenantID       *string
	CommissionRate *int
}

// CreateUserInput is the input for CreateUserDirect (Task 10).
type CreateUserInput struct {
	Email    string
	Password string
	RoleKey  string
	TenantID *string
}

// toActor converts a resolved EffectiveAccess into the pure rbac.Actor value.
// The field shapes match exactly: EffectiveAccess carries the same bool and map
// types that rbac.Actor expects.
func toActor(acc EffectiveAccess) rbac.Actor {
	return rbac.Actor{
		IsPlatformAdmin: acc.IsPlatformAdmin,
		Permissions:     acc.Permissions,
		AllowedTenants:  acc.AllowedTenants,
	}
}

// authorizeGrant resolves the actor's effective access and verifies that they
// are allowed to grant roleKey scoped to tenantID. It returns the resolved
// rbac.Role on success so that callers can use its ID in AssignRole.
//
// Order:
//  1. Resolve actor's EffectiveAccess via the shared AccessService.
//  2. Look up the role by key (ErrRoleNotFound → ErrGrantNotAllowed so that
//     role-key typos surface as a "not allowed" rather than an internal error).
//  3. Build a GrantTarget with the role's scope and, for system roles, the
//     permission set from the catalog (custom roles carry nil permissions).
//  4. Run rbac.CanGrant (pure; no I/O).
//  5. Run rbac.ValidateBinding (binding-integrity invariant, spec §4.4).
func (s *IdentityAdminService) authorizeGrant(
	ctx context.Context,
	actorUserID, roleKey string,
	tenantID *string,
) (rbac.Role, error) {
	acc, err := s.access.Resolve(ctx, actorUserID, tenantID)
	if err != nil {
		return rbac.Role{}, fmt.Errorf("resolving actor access: %w", err)
	}

	role, err := s.rbacRepo.GetRole(ctx, roleKey)
	if err != nil {
		if errors.Is(err, rbac.ErrRoleNotFound) {
			return rbac.Role{}, ErrGrantNotAllowed
		}
		return rbac.Role{}, fmt.Errorf("getting role %q: %w", roleKey, err)
	}

	var perms []rbac.Permission
	if sr, ok := rbac.SystemRoleByKey(role.Key); ok {
		perms = sr.Permissions
	}

	target := rbac.GrantTarget{
		RoleKey:     role.Key,
		ScopeKind:   role.ScopeKind,
		Permissions: perms,
	}

	if err := rbac.CanGrant(toActor(acc), target, tenantID); err != nil {
		return rbac.Role{}, fmt.Errorf("%w: %w", ErrGrantNotAllowed, err)
	}
	if err := rbac.ValidateBinding(role, tenantID); err != nil {
		return rbac.Role{}, fmt.Errorf("binding validation: %w", err)
	}
	return role, nil
}

// InviteUser creates a pending account-creation invitation for the given email,
// scoped to the role and tenant specified in in. The actor must hold a grant
// right for the target role (enforced by authorizeGrant). If a user with that
// email already exists, ErrEmailAlreadyUser is returned.
func (s *IdentityAdminService) InviteUser(
	ctx context.Context,
	actorUserID string,
	in InviteInput,
) (*aggregate.Invitation, error) {
	if _, err := s.authorizeGrant(ctx, actorUserID, in.RoleKey, in.TenantID); err != nil {
		return nil, err
	}

	// Guard: reject if the email is already a registered user.
	existing, err := s.repo.GetUserByEmail(ctx, in.Email)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("checking email: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailAlreadyUser
	}

	now := s.clock.Now()
	inv, err := aggregate.NewInvitation(in.Email, in.RoleKey, in.TenantID, in.CommissionRate, actorUserID, now)
	if err != nil {
		return nil, fmt.Errorf("creating invitation: %w", err)
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateInvitation(txCtx, inv); err != nil {
			return fmt.Errorf("persisting invitation: %w", err)
		}
		if err := s.publisher.Publish(txCtx, NewUserInvitedEvent(inv, now)); err != nil {
			return fmt.Errorf("publishing %s: %w", aggregate.EventUserInvited, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return inv, nil
}

// AcceptInvitation processes a user's acceptance of an invitation. It:
//  1. Looks up the invitation by token.
//  2. Validates that it has not expired.
//  3. Constructs an email-verified PlatformUser (email_verified=true is
//     intentional — spec §6: the invite token was delivered to that address).
//  4. Within a single transaction: creates the user, assigns the role binding,
//     handles shop_owner-specific provisioning, deletes the invitation, issues a
//     session, and publishes the InvitationAccepted event.
//  5. Invalidates the actor's cached access after the transaction.
func (s *IdentityAdminService) AcceptInvitation(
	ctx context.Context,
	token, password, ip, userAgent string,
) (*LoginResult, error) {
	inv, err := s.repo.GetInvitationByToken(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, fmt.Errorf("getting invitation: %w", err)
	}

	now := s.clock.Now()
	if inv.IsExpiredAt(now) {
		return nil, ErrInvitationExpired
	}

	// email_verified=true on accept is intentional — spec §6.
	user, err := aggregate.NewInvitedUser(inv.Email, password, now)
	if err != nil {
		return nil, fmt.Errorf("creating user from invitation: %w", err)
	}

	role, err := s.rbacRepo.GetRole(ctx, inv.RoleKey)
	if err != nil {
		return nil, fmt.Errorf("resolving invited role %q: %w", inv.RoleKey, err)
	}

	var result *LoginResult
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateUser(txCtx, user); err != nil {
			return fmt.Errorf("persisting user: %w", err)
		}

		if err := s.rbacRepo.AssignRole(txCtx, user.ID, role.ID, inv.TenantID, inv.InvitedBy); err != nil {
			return fmt.Errorf("assigning role: %w", err)
		}

		if inv.RoleKey == rbac.RoleShopOwner && inv.TenantID != nil {
			user.TenantID = inv.TenantID
			if err := s.repo.UpdateUser(txCtx, user); err != nil {
				return fmt.Errorf("updating user tenant_id: %w", err)
			}
			if err := s.shops.SetTenantOwner(txCtx, *inv.TenantID, user.ID); err != nil {
				return fmt.Errorf("setting tenant owner: %w", err)
			}
			rate := 0
			if inv.CommissionRate != nil {
				rate = *inv.CommissionRate
			}
			if err := s.shops.CreateResellerAccount(txCtx, *inv.TenantID, user.ID, rate); err != nil {
				return fmt.Errorf("creating reseller account: %w", err)
			}
		}

		if err := s.repo.DeleteInvitation(txCtx, inv.ID); err != nil {
			return fmt.Errorf("deleting invitation: %w", err)
		}

		r, err := s.sessions.Issue(txCtx, user, ip, userAgent, now)
		if err != nil {
			return fmt.Errorf("issuing session: %w", err)
		}
		result = r

		if err := s.publisher.Publish(txCtx, NewUserInvitationAcceptedEvent(user.ID, user.Email, now)); err != nil {
			return fmt.Errorf("publishing %s: %w", aggregate.EventUserInvitationAccepted, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	s.access.Invalidate(user.ID)
	return result, nil
}
