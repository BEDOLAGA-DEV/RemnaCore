package service

import (
	"context"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
)

// customRoleGrantKey is the placeholder role key used when authorizing custom
// roles through rbac.CanGrant. It is deliberately not a system role key, so
// CanGrant's platform_admin/shop_owner blocks never false-trip; the
// no-escalation, scope, and tenant checks still apply against the role's
// requested permission set.
const customRoleGrantKey = "custom"

// CreateCustomRoleInput is the input for creating a custom role.
type CreateCustomRoleInput struct {
	Name        string
	Description string
	ScopeKind   string // rbac.ScopeGlobal | rbac.ScopeShop
	TenantID    *string
	Permissions []rbac.Permission
}

// authorizeCustomGrant resolves the actor's effective access (scoped to
// tenantID) and verifies they may produce a custom role carrying perms at the
// given scope. Global-scope custom roles require platform admin; a non-admin may
// only produce shop-scoped roles within an allowed tenant, with permissions they
// themselves hold (rbac.CanGrant enforces all of this).
func (s *IdentityAdminService) authorizeCustomGrant(ctx context.Context, actorUserID, scopeKind string, perms []rbac.Permission, tenantID *string) error {
	acc, err := s.access.Resolve(ctx, actorUserID, tenantID)
	if err != nil {
		return fmt.Errorf("resolving actor access: %w", err)
	}
	target := rbac.GrantTarget{RoleKey: customRoleGrantKey, ScopeKind: scopeKind, Permissions: perms}
	if err := rbac.CanGrant(toActor(acc), target, tenantID); err != nil {
		return fmt.Errorf("%w: %w", ErrGrantNotAllowed, err)
	}
	return nil
}

// CreateCustomRole creates a tenant-defined role after authorizing the actor.
// Returns the new role ID.
func (s *IdentityAdminService) CreateCustomRole(ctx context.Context, actorUserID string, in CreateCustomRoleInput) (string, error) {
	if err := s.authorizeCustomGrant(ctx, actorUserID, in.ScopeKind, in.Permissions, in.TenantID); err != nil {
		return "", err
	}
	var roleID string
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		var e error
		roleID, e = s.rbacRepo.CreateCustomRole(txCtx, in.Name, in.Description, in.ScopeKind, in.TenantID, in.Permissions)
		return e
	}); err != nil {
		return "", err
	}
	return roleID, nil
}

// ListCustomRoles returns the custom roles for the given scope. A non-admin may
// only list roles of a tenant they belong to; a platform admin may list any
// tenant's (or global via tenantID=nil).
func (s *IdentityAdminService) ListCustomRoles(ctx context.Context, actorUserID string, tenantID *string) ([]rbac.CustomRole, error) {
	acc, err := s.access.Resolve(ctx, actorUserID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("resolving actor access: %w", err)
	}
	if !acc.IsPlatformAdmin {
		if tenantID == nil {
			return nil, ErrGrantNotAllowed
		}
		if _, ok := acc.AllowedTenants[*tenantID]; !ok {
			return nil, ErrGrantNotAllowed
		}
	}
	roles, err := s.rbacRepo.ListCustomRoles(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("listing custom roles: %w", err)
	}
	return roles, nil
}

// DeleteCustomRole deletes a custom role after authorizing the actor against the
// role's own scope and permission set (so a non-admin can only delete a role
// they could have created).
func (s *IdentityAdminService) DeleteCustomRole(ctx context.Context, actorUserID, roleID string) error {
	role, err := s.rbacRepo.GetCustomRole(ctx, roleID)
	if err != nil {
		return err // ErrRoleNotFound
	}
	if err := s.authorizeCustomGrant(ctx, actorUserID, role.ScopeKind, role.Permissions, role.TenantID); err != nil {
		return err
	}
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		_, e := s.rbacRepo.DeleteCustomRole(txCtx, roleID)
		return e
	}); err != nil {
		return err
	}
	// Deleting a role that users hold invalidates their cached effective access.
	// The access cache has no role→user reverse index, so flush the whole cache
	// (matches the AccessService.Flush contract: "call on any role/role_permission
	// mutation"). Without this, holders keep the deleted role's permissions until
	// the cache TTL expires.
	s.access.Flush()
	return nil
}

// AssignCustomRole grants a custom role (by ID) to a user. The actor is
// authorized against the role's resolved permission set (no escalation) and the
// binding is validated for scope/tenant integrity.
func (s *IdentityAdminService) AssignCustomRole(ctx context.Context, actorUserID, targetUserID, roleID string, tenantID *string) error {
	role, err := s.rbacRepo.GetCustomRole(ctx, roleID)
	if err != nil {
		return err // ErrRoleNotFound
	}
	if err := s.authorizeCustomGrant(ctx, actorUserID, role.ScopeKind, role.Permissions, tenantID); err != nil {
		return err
	}
	if err := rbac.ValidateBinding(rbac.Role{ID: role.ID, ScopeKind: role.ScopeKind, TenantID: role.TenantID}, tenantID); err != nil {
		return fmt.Errorf("binding validation: %w", err)
	}
	now := s.clock.Now()
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.rbacRepo.AssignRole(txCtx, targetUserID, role.ID, tenantID, actorUserID); err != nil {
			return fmt.Errorf("assigning custom role: %w", err)
		}
		// roleKey is empty for custom roles.
		if err := s.publisher.Publish(txCtx, NewRoleAssignedEvent(targetUserID, "", tenantID, actorUserID, now)); err != nil {
			return fmt.Errorf("publishing role assigned: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	s.access.Invalidate(targetUserID)
	return nil
}
