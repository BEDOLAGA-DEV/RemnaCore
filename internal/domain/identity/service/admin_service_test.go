package service_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// ============================================================
// Hand-written stubs
// ============================================================

// adminStubRepo is a minimal hand-written stub for service.Repository used by
// IdentityAdminService tests. Only the methods exercised by InviteUser and
// AcceptInvitation are given real behaviour; the rest panic so a test that
// calls an unexpected method fails loudly.
type adminStubRepo struct {
	// pre-loaded data
	invitations  map[string]*aggregate.Invitation // keyed by token
	usersByEmail map[string]*aggregate.PlatformUser
	usersByID    map[string]*aggregate.PlatformUser
	sessions     []*aggregate.Session

	// recorded calls
	createdInvitations []*aggregate.Invitation
	createdUsers       []*aggregate.PlatformUser
	updatedUsers       []*aggregate.PlatformUser
	deletedInvitations []string
}

func newAdminStubRepo() *adminStubRepo {
	return &adminStubRepo{
		invitations:  map[string]*aggregate.Invitation{},
		usersByEmail: map[string]*aggregate.PlatformUser{},
		usersByID:    map[string]*aggregate.PlatformUser{},
	}
}

func (r *adminStubRepo) CreateUser(_ context.Context, u *aggregate.PlatformUser) error {
	r.createdUsers = append(r.createdUsers, u)
	return nil
}
func (r *adminStubRepo) GetUserByID(_ context.Context, _ string) (*aggregate.PlatformUser, error) {
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) GetUserByIDForUpdate(_ context.Context, id string) (*aggregate.PlatformUser, error) {
	if u, ok := r.usersByID[id]; ok {
		return u, nil
	}
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) GetUserByEmail(_ context.Context, email string) (*aggregate.PlatformUser, error) {
	if u, ok := r.usersByEmail[email]; ok {
		return u, nil
	}
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) GetUserByTelegramID(_ context.Context, _ int64) (*aggregate.PlatformUser, error) {
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) UpdateUser(_ context.Context, u *aggregate.PlatformUser) error {
	r.updatedUsers = append(r.updatedUsers, u)
	return nil
}
func (r *adminStubRepo) ListUsers(_ context.Context, _, _ int) ([]*aggregate.PlatformUser, error) {
	return nil, nil
}
func (r *adminStubRepo) CountAdmins(_ context.Context) (int64, error) { return 0, nil }
func (r *adminStubRepo) AcquireBootstrapLock(_ context.Context) error { return nil }
func (r *adminStubRepo) CreateSession(_ context.Context, s *aggregate.Session) error {
	r.sessions = append(r.sessions, s)
	return nil
}
func (r *adminStubRepo) GetSessionByRefreshToken(_ context.Context, _ string) (*aggregate.Session, error) {
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) DeleteSession(_ context.Context, _ string) error      { return nil }
func (r *adminStubRepo) DeleteUserSessions(_ context.Context, _ string) error { return nil }
func (r *adminStubRepo) CreateEmailVerification(_ context.Context, _ *aggregate.EmailVerification) error {
	return nil
}
func (r *adminStubRepo) GetEmailVerification(_ context.Context, _ string) (*aggregate.EmailVerification, error) {
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) DeleteEmailVerification(_ context.Context, _ string) error { return nil }
func (r *adminStubRepo) CreatePasswordReset(_ context.Context, _ *aggregate.PasswordReset) error {
	return nil
}
func (r *adminStubRepo) GetPasswordResetByToken(_ context.Context, _ string) (*aggregate.PasswordReset, error) {
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) DeletePasswordReset(_ context.Context, _ string) error        { return nil }
func (r *adminStubRepo) DeleteUserPasswordResets(_ context.Context, _ string) error   { return nil }
func (r *adminStubRepo) DeleteExpiredSessions(_ context.Context) (int64, error)       { return 0, nil }
func (r *adminStubRepo) DeleteExpiredVerifications(_ context.Context) (int64, error)  { return 0, nil }
func (r *adminStubRepo) DeleteExpiredPasswordResets(_ context.Context) (int64, error) { return 0, nil }
func (r *adminStubRepo) CreateInvitation(_ context.Context, inv *aggregate.Invitation) error {
	r.createdInvitations = append(r.createdInvitations, inv)
	r.invitations[inv.Token] = inv
	return nil
}
func (r *adminStubRepo) GetInvitationByToken(_ context.Context, token string) (*aggregate.Invitation, error) {
	if inv, ok := r.invitations[token]; ok {
		return inv, nil
	}
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) GetInvitationByID(_ context.Context, id string) (*aggregate.Invitation, error) {
	for _, inv := range r.invitations {
		if inv.ID == id {
			return inv, nil
		}
	}
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) DeleteInvitation(_ context.Context, id string) error {
	r.deletedInvitations = append(r.deletedInvitations, id)
	for token, inv := range r.invitations {
		if inv.ID == id {
			delete(r.invitations, token)
			return nil
		}
	}
	return nil
}
func (r *adminStubRepo) ListInvitations(_ context.Context, tenantIDs []string, all bool) ([]*aggregate.Invitation, error) {
	if all {
		out := make([]*aggregate.Invitation, 0, len(r.invitations))
		for _, inv := range r.invitations {
			out = append(out, inv)
		}
		return out, nil
	}
	// Return only invitations whose tenant_id is in the provided list.
	set := make(map[string]struct{}, len(tenantIDs))
	for _, id := range tenantIDs {
		set[id] = struct{}{}
	}
	var out []*aggregate.Invitation
	for _, inv := range r.invitations {
		if inv.TenantID != nil {
			if _, ok := set[*inv.TenantID]; ok {
				out = append(out, inv)
			}
		}
	}
	return out, nil
}
func (r *adminStubRepo) DeleteExpiredInvitations(_ context.Context) (int64, error) { return 0, nil }

// ---- rbac stub ----

// adminStubRBACRepo is a hand-written stub for rbac.Repository.
// It stores roles in a map keyed by key string and records AssignRole calls.
type adminStubRBACRepo struct {
	roles              map[string]rbac.Role
	bindings           map[string][]rbac.Binding // keyed by userID
	assignedRoles      []assignedRoleRecord
	revokedRoles       []revokedRoleRecord
	platformAdminCount int   // controlled per-test; defaults to 1
	revokeRowsAffected int64 // rows returned by RevokeRole; defaults to 1
	customRoles        map[string]rbac.CustomRole
}

type assignedRoleRecord struct {
	UserID    string
	RoleID    string
	TenantID  *string
	GrantedBy string
}

type revokedRoleRecord struct {
	UserID   string
	RoleID   string
	TenantID *string
}

func newAdminStubRBACRepo() *adminStubRBACRepo {
	return &adminStubRBACRepo{
		roles:              map[string]rbac.Role{},
		bindings:           map[string][]rbac.Binding{},
		platformAdminCount: 1, // safe default: there is at least one admin
		revokeRowsAffected: 1, // default: binding exists and is removed
	}
}

func (r *adminStubRBACRepo) addRole(role rbac.Role) {
	r.roles[role.Key] = role
}

func (r *adminStubRBACRepo) ListBindingsForUser(_ context.Context, userID string) ([]rbac.Binding, error) {
	return r.bindings[userID], nil
}

func (r *adminStubRBACRepo) PermissionsForRoles(_ context.Context, roleIDs []string) (map[string][]rbac.Permission, error) {
	out := map[string][]rbac.Permission{}
	for _, id := range roleIDs {
		for _, role := range r.roles {
			if role.ID == id {
				if sr, ok := rbac.SystemRoleByKey(role.Key); ok {
					out[id] = sr.Permissions
				}
			}
		}
	}
	return out, nil
}

func (r *adminStubRBACRepo) SyncCatalog(_ context.Context, _ []rbac.Definition, _ []rbac.SystemRole) error {
	return nil
}

func (r *adminStubRBACRepo) AssignRole(_ context.Context, userID, roleID string, tenantID *string, grantedBy string) error {
	r.assignedRoles = append(r.assignedRoles, assignedRoleRecord{
		UserID: userID, RoleID: roleID, TenantID: tenantID, GrantedBy: grantedBy,
	})
	return nil
}

func (r *adminStubRBACRepo) RevokeRole(_ context.Context, userID, roleID string, tenantID *string) (int64, error) {
	r.revokedRoles = append(r.revokedRoles, revokedRoleRecord{UserID: userID, RoleID: roleID, TenantID: tenantID})
	return r.revokeRowsAffected, nil
}

func (r *adminStubRBACRepo) GetRole(_ context.Context, key string) (rbac.Role, error) {
	if role, ok := r.roles[key]; ok {
		return role, nil
	}
	return rbac.Role{}, rbac.ErrRoleNotFound
}

func (r *adminStubRBACRepo) CountPlatformAdmins(_ context.Context) (int, error) {
	return r.platformAdminCount, nil
}

// ---- custom-role CRUD stub (Phase D) ----

func (r *adminStubRBACRepo) CreateCustomRole(_ context.Context, name, description, scopeKind string, tenantID *string, perms []rbac.Permission) (string, error) {
	if r.customRoles == nil {
		r.customRoles = map[string]rbac.CustomRole{}
	}
	id := fmt.Sprintf("custom-%d", len(r.customRoles)+1)
	r.customRoles[id] = rbac.CustomRole{
		ID: id, Name: name, Description: description, ScopeKind: scopeKind, TenantID: tenantID, Permissions: perms,
	}
	return id, nil
}

func (r *adminStubRBACRepo) ListCustomRoles(_ context.Context, tenantID *string) ([]rbac.CustomRole, error) {
	var out []rbac.CustomRole
	for _, cr := range r.customRoles {
		switch {
		case tenantID == nil && cr.TenantID == nil:
			out = append(out, cr)
		case tenantID != nil && cr.TenantID != nil && *cr.TenantID == *tenantID:
			out = append(out, cr)
		}
	}
	return out, nil
}

func (r *adminStubRBACRepo) GetCustomRole(_ context.Context, roleID string) (rbac.CustomRole, error) {
	if cr, ok := r.customRoles[roleID]; ok {
		return cr, nil
	}
	return rbac.CustomRole{}, rbac.ErrRoleNotFound
}

func (r *adminStubRBACRepo) DeleteCustomRole(_ context.Context, roleID string) (int64, error) {
	if _, ok := r.customRoles[roleID]; !ok {
		return 0, nil
	}
	delete(r.customRoles, roleID)
	return 1, nil
}

// ---- shop provisioner stub ----

type adminStubShops struct {
	createTenantCalls   []createTenantCall
	setTenantOwnerCalls []setTenantOwnerCall
	createResellerCalls []createResellerCall
}

type createTenantCall struct {
	Name        string
	Domain      string
	OwnerUserID *string
}

type setTenantOwnerCall struct {
	TenantID string
	UserID   string
}

type createResellerCall struct {
	TenantID       string
	UserID         string
	CommissionRate int
}

func (s *adminStubShops) CreateTenant(_ context.Context, name, domain string, ownerUserID *string) (string, string, error) {
	s.createTenantCalls = append(s.createTenantCalls, createTenantCall{Name: name, Domain: domain, OwnerUserID: ownerUserID})
	return "tenant-1", "api-key-1", nil
}

func (s *adminStubShops) SetTenantOwner(_ context.Context, tenantID, userID string) error {
	s.setTenantOwnerCalls = append(s.setTenantOwnerCalls, setTenantOwnerCall{tenantID, userID})
	return nil
}

func (s *adminStubShops) CreateResellerAccount(_ context.Context, tenantID, userID string, commissionRate int) error {
	s.createResellerCalls = append(s.createResellerCalls, createResellerCall{tenantID, userID, commissionRate})
	return nil
}

// ---- noop publisher stub ----

type noopPublisher struct {
	events []domainevent.Event
}

func (p *noopPublisher) Publish(_ context.Context, e domainevent.Event) error {
	p.events = append(p.events, e)
	return nil
}

func (p *noopPublisher) PublishBatch(_ context.Context, events []domainevent.Event) error {
	p.events = append(p.events, events...)
	return nil
}

// ============================================================
// Test helpers
// ============================================================

// newTestJWT generates a throw-away ECDSA key pair and wraps it in a JWTIssuer.
func newTestJWT(t *testing.T) *authutil.JWTIssuer {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return authutil.NewJWTIssuer(priv, &priv.PublicKey)
}

// shopOwnerBindings returns the bindings for a shop_owner of shopID.
func shopOwnerBindings(userID, shopID string) map[string][]rbac.Binding {
	return map[string][]rbac.Binding{
		userID: {
			{
				RoleID:    "role-owner-id",
				RoleKey:   rbac.RoleShopOwner,
				ScopeKind: rbac.ScopeShop,
				TenantID:  &shopID,
			},
		},
	}
}

// platformAdminBindings returns the bindings for a platform admin.
func platformAdminBindings(userID string) map[string][]rbac.Binding {
	return map[string][]rbac.Binding{
		userID: {
			{
				RoleID:    "role-admin-id",
				RoleKey:   rbac.RolePlatformAdmin,
				ScopeKind: rbac.ScopeGlobal,
			},
		},
	}
}

// buildAdminSvcWith creates a ready-to-use IdentityAdminService using the
// provided Repository implementation (any type satisfying the interface).
func buildAdminSvcWith(
	t *testing.T,
	repo service.Repository,
	rbacRepo *adminStubRBACRepo,
	shops *adminStubShops,
	pub domainevent.Publisher,
	fixedNow time.Time,
) *service.IdentityAdminService {
	t.Helper()

	// SessionIssuer only needs CreateSession; use the plain adminStubRepo if
	// the caller passed a wrapper. Fall back to a bare stub for session storage.
	sessionStore, ok := repo.(interface {
		CreateSession(context.Context, *aggregate.Session) error
	})
	if !ok {
		sessionStore = newAdminStubRepo()
	}
	jwtIssuer := newTestJWT(t)
	issuer := service.NewSessionIssuer(sessionStore, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
	access := service.NewAccessService(rbacRepo, func() time.Time { return fixedNow }, time.Minute)
	clk := clock.NewMock(fixedNow)

	return service.NewIdentityAdminService(
		repo,
		rbacRepo,
		access,
		shops,
		issuer,
		txmanagertest.NoopTxRunner{},
		pub,
		clk,
	)
}

// buildAdminSvc creates a ready-to-use IdentityAdminService with the given
// stubs wired together. Convenience wrapper over buildAdminSvcWith.
func buildAdminSvc(
	t *testing.T,
	repo *adminStubRepo,
	rbacRepo *adminStubRBACRepo,
	shops *adminStubShops,
	pub domainevent.Publisher,
	fixedNow time.Time,
) *service.IdentityAdminService {
	t.Helper()
	return buildAdminSvcWith(t, repo, rbacRepo, shops, pub, fixedNow)
}

// ============================================================
// Tests: InviteUser
// ============================================================

// TestInviteUser_HappyPath verifies that a platform admin can invite a staff
// member. The returned invitation must have a non-empty token and the
// UserInvited event must be published.
func TestInviteUser_HappyPath(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	shopID := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	inv, err := svc.InviteUser(context.Background(), actorID, service.InviteInput{
		Email:    "staff@example.com",
		RoleKey:  rbac.RoleShopStaff,
		TenantID: &shopID,
	})

	require.NoError(t, err)
	require.NotNil(t, inv)
	assert.NotEmpty(t, inv.Token)
	assert.Equal(t, "staff@example.com", inv.Email)
	assert.Equal(t, rbac.RoleShopStaff, inv.RoleKey)
	assert.Equal(t, &shopID, inv.TenantID)

	// Invitation must be persisted.
	require.Len(t, repo.createdInvitations, 1)
	assert.Equal(t, inv.ID, repo.createdInvitations[0].ID)

	// UserInvited event must be published.
	require.Len(t, pub.events, 1)
	assert.Equal(t, aggregate.EventUserInvited, pub.events[0].Type)
}

// TestInviteUser_GrantNotAllowed verifies that a shop_owner inviting into a
// shop they do not belong to gets ErrGrantNotAllowed.
func TestInviteUser_GrantNotAllowed(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222" // actor is NOT a member of B

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	_, err := svc.InviteUser(context.Background(), actorID, service.InviteInput{
		Email:    "staff@other-shop.com",
		RoleKey:  rbac.RoleShopStaff,
		TenantID: &shopB,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, rbac.ErrGrantNotAllowed)
	assert.ErrorIs(t, err, service.ErrGrantNotAllowed)
}

// ============================================================
// Tests: AcceptInvitation
// ============================================================

// TestAcceptInvitation_Valid verifies the happy path: a valid, non-expired token
// creates an email-verified user, assigns the role binding, deletes the
// invitation, and issues a session (returns a LoginResult).
func TestAcceptInvitation_Valid(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	shopID := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})

	// Pre-load a valid invitation.
	inv, err := aggregate.NewInvitation(
		"staff@example.com",
		rbac.RoleShopStaff,
		&shopID,
		nil,
		"inviter-id",
		now,
	)
	require.NoError(t, err)
	repo.invitations[inv.Token] = inv

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	result, err := svc.AcceptInvitation(
		context.Background(),
		inv.Token,
		"StrongP4ss",
		"127.0.0.1",
		"test-agent",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.NotNil(t, result.User)
	assert.True(t, result.User.EmailVerified, "invited user must be email-verified")

	// User must be created with EmailVerified=true (invite token proves ownership).
	require.Len(t, repo.createdUsers, 1)
	assert.Equal(t, "staff@example.com", repo.createdUsers[0].Email)
	assert.True(t, repo.createdUsers[0].EmailVerified, "persisted user must have EmailVerified=true")

	// Role binding must be assigned.
	require.Len(t, rbacRepo.assignedRoles, 1)
	assert.Equal(t, "role-staff-id", rbacRepo.assignedRoles[0].RoleID)
	assert.Equal(t, &shopID, rbacRepo.assignedRoles[0].TenantID)
	assert.Equal(t, "inviter-id", rbacRepo.assignedRoles[0].GrantedBy)

	// Invitation must be deleted.
	require.Len(t, repo.deletedInvitations, 1)
	assert.Equal(t, inv.ID, repo.deletedInvitations[0])

	// Session must be created.
	require.Len(t, repo.sessions, 1)

	// InvitationAccepted event must be published.
	hasAccepted := false
	for _, e := range pub.events {
		if e.Type == aggregate.EventUserInvitationAccepted {
			hasAccepted = true
		}
	}
	assert.True(t, hasAccepted, "UserInvitationAccepted event must be published")
}

// TestAcceptInvitation_Expired verifies that an expired invitation token
// returns ErrInvitationExpired without creating any side-effects.
func TestAcceptInvitation_Expired(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	// Create the invitation at time zero, then "accept" far in the future.
	past := now.Add(-aggregate.InvitationTTL - time.Second)
	shopID := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})

	// Invitation was created in the past and is therefore expired at `now`.
	inv, err := aggregate.NewInvitation(
		"staff@example.com",
		rbac.RoleShopStaff,
		&shopID,
		nil,
		"inviter-id",
		past,
	)
	require.NoError(t, err)
	repo.invitations[inv.Token] = inv

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	_, err = svc.AcceptInvitation(
		context.Background(),
		inv.Token,
		"StrongP4ss",
		"127.0.0.1",
		"test-agent",
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrInvitationExpired)
	assert.Empty(t, repo.createdUsers, "no user must be created for an expired invitation")
}

// TestAcceptInvitation_ShopOwner verifies the shop_owner-specific wiring:
// SetTenantOwner and CreateResellerAccount must be called, and the new user's
// TenantID must be populated.
func TestAcceptInvitation_ShopOwner(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tenantID := "aaaa0000-0000-0000-0000-000000000000"
	commissionRate := 15

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.addRole(rbac.Role{
		ID:        "role-owner-id",
		Key:       rbac.RoleShopOwner,
		ScopeKind: rbac.ScopeShop,
	})

	inv, err := aggregate.NewInvitation(
		"owner@example.com",
		rbac.RoleShopOwner,
		&tenantID,
		&commissionRate,
		"admin-id",
		now,
	)
	require.NoError(t, err)
	repo.invitations[inv.Token] = inv

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	result, err := svc.AcceptInvitation(
		context.Background(),
		inv.Token,
		"StrongP4ss",
		"127.0.0.1",
		"test-agent",
	)

	require.NoError(t, err)
	require.NotNil(t, result)

	// user.TenantID must be set.
	require.Len(t, repo.createdUsers, 1)
	// UpdateUser must have been called to persist the tenant_id.
	require.Len(t, repo.updatedUsers, 1)
	assert.Equal(t, &tenantID, repo.updatedUsers[0].TenantID)

	// SetTenantOwner must be called with the right IDs.
	require.Len(t, shops.setTenantOwnerCalls, 1)
	assert.Equal(t, tenantID, shops.setTenantOwnerCalls[0].TenantID)
	assert.Equal(t, repo.createdUsers[0].ID, shops.setTenantOwnerCalls[0].UserID)

	// CreateResellerAccount must be called with the commission rate.
	require.Len(t, shops.createResellerCalls, 1)
	assert.Equal(t, tenantID, shops.createResellerCalls[0].TenantID)
	assert.Equal(t, commissionRate, shops.createResellerCalls[0].CommissionRate)
}

// ============================================================
// Tests: AssignRole (Task 10)
// ============================================================

// TestAssignRole_HappyPath verifies that a platform admin can assign a
// shop_staff role to an existing user. The RoleAssigned event must be published
// and the role binding recorded.
func TestAssignRole_HappyPath(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	targetID := "target-user-id"
	shopID := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	err := svc.AssignRole(context.Background(), actorID, targetID, rbac.RoleShopStaff, &shopID)

	require.NoError(t, err)

	// Role binding must be recorded.
	require.Len(t, rbacRepo.assignedRoles, 1)
	assert.Equal(t, targetID, rbacRepo.assignedRoles[0].UserID)
	assert.Equal(t, "role-staff-id", rbacRepo.assignedRoles[0].RoleID)
	assert.Equal(t, &shopID, rbacRepo.assignedRoles[0].TenantID)
	assert.Equal(t, actorID, rbacRepo.assignedRoles[0].GrantedBy)

	// RoleAssigned event must be published.
	hasEvent := false
	for _, e := range pub.events {
		if e.Type == aggregate.EventRoleAssigned {
			hasEvent = true
		}
	}
	assert.True(t, hasEvent, "RoleAssigned event must be published")
}

// TestAssignRole_GrantNotAllowed verifies that a shop_owner cannot assign a
// role in a shop they do not belong to.
func TestAssignRole_GrantNotAllowed(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	targetID := "target-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	err := svc.AssignRole(context.Background(), actorID, targetID, rbac.RoleShopStaff, &shopB)

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrGrantNotAllowed)
}

// ============================================================
// Tests: RevokeRole (Task 10)
// ============================================================

// TestRevokeRole_HappyPath verifies that a platform admin can revoke a
// shop_staff role from a user.
func TestRevokeRole_HappyPath(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	targetID := "target-user-id"
	shopID := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	err := svc.RevokeRole(context.Background(), actorID, targetID, rbac.RoleShopStaff, &shopID)

	require.NoError(t, err)

	// RevokeRole must be called on the repo.
	require.Len(t, rbacRepo.revokedRoles, 1)
	assert.Equal(t, targetID, rbacRepo.revokedRoles[0].UserID)
	assert.Equal(t, "role-staff-id", rbacRepo.revokedRoles[0].RoleID)
	assert.Equal(t, &shopID, rbacRepo.revokedRoles[0].TenantID)

	// RoleRevoked event must be published.
	hasEvent := false
	for _, e := range pub.events {
		if e.Type == aggregate.EventRoleRevoked {
			hasEvent = true
		}
	}
	assert.True(t, hasEvent, "RoleRevoked event must be published")
}

// TestRevokeRole_LastAdmin verifies that revoking the last global
// platform_admin binding returns ErrLastAdmin.
func TestRevokeRole_LastAdmin(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	targetID := "admin-user-id" // actor revokes their own binding (last admin)

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-admin-id",
		Key:       rbac.RolePlatformAdmin,
		ScopeKind: rbac.ScopeGlobal,
	})
	// Only one platform admin exists.
	rbacRepo.platformAdminCount = 1
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	err := svc.RevokeRole(context.Background(), actorID, targetID, rbac.RolePlatformAdmin, nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrLastAdmin)
	// No binding should be removed.
	assert.Empty(t, rbacRepo.revokedRoles)
}

// ============================================================
// Tests: CreateUserDirect (Task 10)
// ============================================================

// TestCreateUserDirect_HappyPath verifies that an admin can directly create a
// new user with MustChangePassword=true and a role binding assigned atomically.
func TestCreateUserDirect_HappyPath(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	shopID := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	user, err := svc.CreateUserDirect(context.Background(), actorID, service.CreateUserInput{
		Email:    "newstaff@example.com",
		Password: "StrongP4ss",
		RoleKey:  rbac.RoleShopStaff,
		TenantID: &shopID,
	})

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "newstaff@example.com", user.Email)
	assert.True(t, user.EmailVerified, "admin-created user must be email-verified")
	assert.True(t, user.MustChangePassword, "admin-created user must have MustChangePassword=true")

	// User must be persisted.
	require.Len(t, repo.createdUsers, 1)
	assert.True(t, repo.createdUsers[0].MustChangePassword)

	// Role binding must be assigned.
	require.Len(t, rbacRepo.assignedRoles, 1)
	assert.Equal(t, user.ID, rbacRepo.assignedRoles[0].UserID)
	assert.Equal(t, "role-staff-id", rbacRepo.assignedRoles[0].RoleID)
	assert.Equal(t, &shopID, rbacRepo.assignedRoles[0].TenantID)

	// RoleAssigned event must be published.
	hasEvent := false
	for _, e := range pub.events {
		if e.Type == aggregate.EventRoleAssigned {
			hasEvent = true
		}
	}
	assert.True(t, hasEvent, "RoleAssigned event must be published for direct-created user")
}

// TestCreateUserDirect_EmailTaken verifies that creating a user with a
// duplicate email returns ErrEmailTaken.
func TestCreateUserDirect_EmailTaken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	shopID := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	// Pre-load an existing user with the same email.
	repo.usersByEmail["dup@example.com"] = &aggregate.PlatformUser{ID: "existing-id", Email: "dup@example.com"}
	// Simulate a duplicate-email error from the repo by intercepting CreateUser.
	dupRepo := &dupEmailStubRepo{adminStubRepo: repo}

	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvcWith(t, dupRepo, rbacRepo, shops, pub, now)

	_, err := svc.CreateUserDirect(context.Background(), actorID, service.CreateUserInput{
		Email:    "dup@example.com",
		Password: "StrongP4ss",
		RoleKey:  rbac.RoleShopStaff,
		TenantID: &shopID,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrEmailTaken)
}

// dupEmailStubRepo wraps adminStubRepo and returns ErrEmailTaken from CreateUser.
type dupEmailStubRepo struct {
	*adminStubRepo
}

func (r *dupEmailStubRepo) CreateUser(_ context.Context, _ *aggregate.PlatformUser) error {
	return service.ErrEmailTaken
}

// ============================================================
// Tests: ListInvitations (Task 10)
// ============================================================

// TestListInvitations_AdminSeesAll verifies that a platform admin receives all
// pending invitations regardless of tenant scope.
func TestListInvitations_AdminSeesAll(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)

	// Two invitations in different shops.
	invA, err := aggregate.NewInvitation("a@example.com", rbac.RoleShopStaff, &shopA, nil, actorID, now)
	require.NoError(t, err)
	invB, err := aggregate.NewInvitation("b@example.com", rbac.RoleShopStaff, &shopB, nil, actorID, now)
	require.NoError(t, err)
	repo.invitations[invA.Token] = invA
	repo.invitations[invB.Token] = invB

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	invites, err := svc.ListInvitations(context.Background(), actorID)

	require.NoError(t, err)
	assert.Len(t, invites, 2, "platform admin must see all invitations")
}

// TestListInvitations_NonAdminScoped verifies that a shop_owner only sees
// invitations scoped to their own shop.
func TestListInvitations_NonAdminScoped(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)

	// One invitation in the actor's shop, one in another shop.
	invA, err := aggregate.NewInvitation("a@example.com", rbac.RoleShopStaff, &shopA, nil, actorID, now)
	require.NoError(t, err)
	invB, err := aggregate.NewInvitation("b@example.com", rbac.RoleShopStaff, &shopB, nil, "other-admin", now)
	require.NoError(t, err)
	repo.invitations[invA.Token] = invA
	repo.invitations[invB.Token] = invB

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	invites, err := svc.ListInvitations(context.Background(), actorID)

	require.NoError(t, err)
	require.Len(t, invites, 1, "shop_owner must see only their shop's invitations")
	assert.Equal(t, &shopA, invites[0].TenantID)
}

// ============================================================
// Tests: RevokeInvitation (Task 10)
// ============================================================

// TestRevokeInvitation_ScopeReject verifies that a shop_owner cannot revoke an
// invitation scoped to a different shop.
func TestRevokeInvitation_ScopeReject(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)

	// Invitation belongs to shopB — the actor does not belong to shopB.
	inv, err := aggregate.NewInvitation("b@example.com", rbac.RoleShopStaff, &shopB, nil, "other-admin", now)
	require.NoError(t, err)
	repo.invitations[inv.Token] = inv

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	err = svc.RevokeInvitation(context.Background(), actorID, inv.ID)

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrGrantNotAllowed)
	// Invitation must NOT be deleted.
	assert.Empty(t, repo.deletedInvitations)
}

// ============================================================
// Tests: ListUserRoles — cross-tenant scoping (security fix)
// ============================================================

// TestListUserRoles_NonAdminFiltered verifies that a shop_owner of shopA who
// requests a target user's bindings only sees the shopA binding; shopB and
// global bindings are filtered out to prevent cross-tenant disclosure.
func TestListUserRoles_NonAdminFiltered(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	targetID := "target-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	// Actor is only a shop_owner of shopA.
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)
	rbacRepo.addRole(rbac.Role{ID: "role-owner-id", Key: rbac.RoleShopOwner, ScopeKind: rbac.ScopeShop})
	rbacRepo.addRole(rbac.Role{ID: "role-staff-id", Key: rbac.RoleShopStaff, ScopeKind: rbac.ScopeShop})
	rbacRepo.addRole(rbac.Role{ID: "role-admin-id", Key: rbac.RolePlatformAdmin, ScopeKind: rbac.ScopeGlobal})

	// Target has three bindings: shopA, shopB, and a global (platform_admin).
	rbacRepo.bindings[targetID] = []rbac.Binding{
		{RoleID: "role-staff-id", RoleKey: rbac.RoleShopStaff, ScopeKind: rbac.ScopeShop, TenantID: &shopA},
		{RoleID: "role-staff-id", RoleKey: rbac.RoleShopStaff, ScopeKind: rbac.ScopeShop, TenantID: &shopB},
		{RoleID: "role-admin-id", RoleKey: rbac.RolePlatformAdmin, ScopeKind: rbac.ScopeGlobal, TenantID: nil},
	}

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	bindings, err := svc.ListUserRoles(context.Background(), actorID, targetID)

	require.NoError(t, err)
	require.Len(t, bindings, 1, "non-admin must see only bindings in their own tenant(s)")
	assert.Equal(t, &shopA, bindings[0].TenantID)
}

// TestListUserRoles_AdminSeesAll verifies that a platform admin sees all bindings
// for a target user — no filtering applied.
func TestListUserRoles_AdminSeesAll(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	targetID := "target-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{ID: "role-admin-id", Key: rbac.RolePlatformAdmin, ScopeKind: rbac.ScopeGlobal})
	rbacRepo.addRole(rbac.Role{ID: "role-staff-id", Key: rbac.RoleShopStaff, ScopeKind: rbac.ScopeShop})

	// Target has bindings in shopA, shopB, and a global binding.
	rbacRepo.bindings[targetID] = []rbac.Binding{
		{RoleID: "role-staff-id", RoleKey: rbac.RoleShopStaff, ScopeKind: rbac.ScopeShop, TenantID: &shopA},
		{RoleID: "role-staff-id", RoleKey: rbac.RoleShopStaff, ScopeKind: rbac.ScopeShop, TenantID: &shopB},
		{RoleID: "role-admin-id", RoleKey: rbac.RolePlatformAdmin, ScopeKind: rbac.ScopeGlobal, TenantID: nil},
	}

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	bindings, err := svc.ListUserRoles(context.Background(), actorID, targetID)

	require.NoError(t, err)
	assert.Len(t, bindings, 3, "platform admin must see all three bindings")
}

// ============================================================
// Tests: RevokeRole — no-op binding guard (security fix)
// ============================================================

// TestRevokeRole_NoBinding verifies that RevokeRole returns ErrBindingNotFound
// when the binding does not exist (RevokeRole stub returns 0 rows affected), and
// that no RoleRevoked event is published.
func TestRevokeRole_NoBinding(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	targetID := "target-user-id"
	shopID := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-staff-id",
		Key:       rbac.RoleShopStaff,
		ScopeKind: rbac.ScopeShop,
	})
	// Simulate missing binding: RevokeRole returns 0 rows affected.
	rbacRepo.revokeRowsAffected = 0

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	err := svc.RevokeRole(context.Background(), actorID, targetID, rbac.RoleShopStaff, &shopID)

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrBindingNotFound)

	// No RoleRevoked event must be published when nothing was removed.
	for _, e := range pub.events {
		assert.NotEqual(t, aggregate.EventRoleRevoked, e.Type,
			"RoleRevoked event must NOT be published when binding was not found")
	}
}

// ============================================================
// Tests: RevokeInvitation — symmetric grant-right check (security fix)
// ============================================================

// TestRevokeInvitation_RequiresGrantRight verifies that a shop_owner of shopA
// cannot revoke a shop_owner-role invitation (a role they cannot grant) even
// when it is scoped to shopA. The error must be ErrGrantNotAllowed.
func TestRevokeInvitation_RequiresGrantRight(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	// Actor is a shop_owner of shopA.
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)
	rbacRepo.addRole(rbac.Role{ID: "role-owner-id", Key: rbac.RoleShopOwner, ScopeKind: rbac.ScopeShop})

	// Invitation is for shop_owner in shopA — a role that cannot be granted by
	// a non-platform-admin per CanGrant.
	inv, err := aggregate.NewInvitation("newowner@example.com", rbac.RoleShopOwner, &shopA, nil, "admin-id", now)
	require.NoError(t, err)
	repo.invitations[inv.Token] = inv

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	err = svc.RevokeInvitation(context.Background(), actorID, inv.ID)

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrGrantNotAllowed)
	// Invitation must NOT be deleted.
	assert.Empty(t, repo.deletedInvitations)
}

// TestRevokeInvitation_AdminHappyPath verifies that a platform admin can revoke
// any invitation, including one for shop_owner.
func TestRevokeInvitation_AdminHappyPath(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{ID: "role-admin-id", Key: rbac.RolePlatformAdmin, ScopeKind: rbac.ScopeGlobal})
	rbacRepo.addRole(rbac.Role{ID: "role-owner-id", Key: rbac.RoleShopOwner, ScopeKind: rbac.ScopeShop})

	inv, err := aggregate.NewInvitation("newowner@example.com", rbac.RoleShopOwner, &shopA, nil, actorID, now)
	require.NoError(t, err)
	repo.invitations[inv.Token] = inv

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	err = svc.RevokeInvitation(context.Background(), actorID, inv.ID)

	require.NoError(t, err)
	require.Len(t, repo.deletedInvitations, 1)
	assert.Equal(t, inv.ID, repo.deletedInvitations[0])
}

// ============================================================
// Tests: ListInvitations — global invitation hidden from non-admin
// ============================================================

// TestListInvitations_GlobalHiddenFromNonAdmin verifies that a shop_owner does
// not see a NULL-tenant (global) invitation when listing. ListInvitations passes
// tenant scoping to the repo, and the repo stub filters out nil-tenant entries.
func TestListInvitations_GlobalHiddenFromNonAdmin(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)

	// One invitation scoped to shopA, one global (nil tenant).
	invA, err := aggregate.NewInvitation("a@example.com", rbac.RoleShopStaff, &shopA, nil, actorID, now)
	require.NoError(t, err)
	invGlobal, err := aggregate.NewInvitation("g@example.com", rbac.RolePlatformAdmin, nil, nil, "superadmin", now)
	require.NoError(t, err)
	repo.invitations[invA.Token] = invA
	repo.invitations[invGlobal.Token] = invGlobal

	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	invites, err := svc.ListInvitations(context.Background(), actorID)

	require.NoError(t, err)
	require.Len(t, invites, 1, "non-admin must not see global (nil-tenant) invitations")
	assert.Equal(t, &shopA, invites[0].TenantID)
}

// ============================================================
// Tests: CreateShop (Task 11)
// ============================================================

// TestCreateShop_NonAdminRejected verifies that a shop_owner (who holds
// shops.manage) is still rejected by the platform-admin guard inside
// CreateShop. The route gate alone (shops.manage) is insufficient.
func TestCreateShop_NonAdminRejected(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	// Actor is a shop_owner (holds shops.manage) but is NOT a platform admin.
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-owner-id",
		Key:       rbac.RoleShopOwner,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	_, err := svc.CreateShop(context.Background(), actorID, service.CreateShopInput{
		Name:   "My Shop",
		Domain: "myshop.example.com",
		Owner:  service.OwnerSpec{InviteEmail: "owner@example.com"},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrNotPlatformAdmin,
		"shop_owner must not be allowed to create shops even if they hold shops.manage")
	// No tenant must be provisioned.
	assert.Empty(t, shops.createTenantCalls)
}

// TestCreateShop_ExistingOwner verifies the existing-owner path:
//   - CreateTenant is called with the owner's user ID.
//   - The owner user's TenantID is set and UpdateUser is called.
//   - AssignRole(shop_owner, tenant) is called atomically.
//   - CreateResellerAccount is called with the commission rate.
//   - ShopResult.OwnerUserID is populated; InvitationToken is empty.
//   - ShopCreated event is published.
//   - Access cache is invalidated for the owner.
func TestCreateShop_ExistingOwner(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	ownerID := "existing-owner-id"
	commissionRate := 20

	repo := newAdminStubRepo()
	// Pre-load the owner so GetUserByIDForUpdate succeeds.
	repo.usersByID[ownerID] = &aggregate.PlatformUser{
		ID:    ownerID,
		Email: "existingowner@example.com",
	}

	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-owner-id",
		Key:       rbac.RoleShopOwner,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	result, err := svc.CreateShop(context.Background(), actorID, service.CreateShopInput{
		Name:           "Acme Shop",
		Domain:         "acme.example.com",
		Owner:          service.OwnerSpec{ExistingUserID: ownerID},
		CommissionRate: commissionRate,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// TenantID and APIKey from stub.
	assert.Equal(t, "tenant-1", result.TenantID)
	assert.Equal(t, "api-key-1", result.APIKey)

	// OwnerUserID must be set; InvitationToken must be empty.
	require.NotNil(t, result.OwnerUserID)
	assert.Equal(t, ownerID, *result.OwnerUserID)
	assert.Empty(t, result.InvitationToken)

	// CreateTenant must have been called with the owner's user ID.
	require.Len(t, shops.createTenantCalls, 1)
	assert.Equal(t, "Acme Shop", shops.createTenantCalls[0].Name)
	assert.Equal(t, "acme.example.com", shops.createTenantCalls[0].Domain)
	require.NotNil(t, shops.createTenantCalls[0].OwnerUserID)
	assert.Equal(t, ownerID, *shops.createTenantCalls[0].OwnerUserID)

	// UpdateUser must have been called with TenantID set.
	require.Len(t, repo.updatedUsers, 1)
	require.NotNil(t, repo.updatedUsers[0].TenantID)
	assert.Equal(t, "tenant-1", *repo.updatedUsers[0].TenantID)

	// AssignRole(shop_owner, tenantID) must have been called.
	require.Len(t, rbacRepo.assignedRoles, 1)
	assert.Equal(t, ownerID, rbacRepo.assignedRoles[0].UserID)
	assert.Equal(t, "role-owner-id", rbacRepo.assignedRoles[0].RoleID)
	require.NotNil(t, rbacRepo.assignedRoles[0].TenantID)
	assert.Equal(t, "tenant-1", *rbacRepo.assignedRoles[0].TenantID)
	assert.Equal(t, actorID, rbacRepo.assignedRoles[0].GrantedBy)

	// CreateResellerAccount must have been called with the commission rate.
	// The existing-owner path calls CreateResellerAccount but NOT SetTenantOwner
	// (SetTenantOwner is only called in AcceptInvitation for invite-owner flow).
	require.Len(t, shops.createResellerCalls, 1)
	assert.Equal(t, "tenant-1", shops.createResellerCalls[0].TenantID)
	assert.Equal(t, ownerID, shops.createResellerCalls[0].UserID)
	assert.Equal(t, commissionRate, shops.createResellerCalls[0].CommissionRate)

	// SetTenantOwner must NOT be called in the existing-owner path.
	assert.Empty(t, shops.setTenantOwnerCalls)

	// ShopCreated event must be published.
	hasShopCreated := false
	for _, e := range pub.events {
		if e.Type == aggregate.EventShopCreated {
			hasShopCreated = true
		}
	}
	assert.True(t, hasShopCreated, "ShopCreated event must be published")
}

// TestCreateShop_InviteOwner verifies the invite-owner path:
//   - CreateTenant is called with a nil owner (pending).
//   - CreateInvitation is called for shop_owner scoped to the new tenant.
//   - ShopResult.InvitationToken is set; OwnerUserID is nil.
//   - Both UserInvited and ShopCreated events are published.
func TestCreateShop_InviteOwner(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	commissionRate := 15

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-owner-id",
		Key:       rbac.RoleShopOwner,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	result, err := svc.CreateShop(context.Background(), actorID, service.CreateShopInput{
		Name:           "Pending Shop",
		Domain:         "pending.example.com",
		Owner:          service.OwnerSpec{InviteEmail: "futureowner@example.com"},
		CommissionRate: commissionRate,
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// TenantID and APIKey from stub.
	assert.Equal(t, "tenant-1", result.TenantID)
	assert.Equal(t, "api-key-1", result.APIKey)

	// InvitationToken must be set; OwnerUserID must be nil.
	assert.NotEmpty(t, result.InvitationToken)
	assert.Nil(t, result.OwnerUserID)

	// CreateTenant must have been called with nil owner (pending shop).
	require.Len(t, shops.createTenantCalls, 1)
	assert.Nil(t, shops.createTenantCalls[0].OwnerUserID,
		"invite-owner path must create tenant with nil owner")

	// CreateInvitation must be called with role=shop_owner, the new tenant, and
	// the commission rate.
	require.Len(t, repo.createdInvitations, 1)
	inv := repo.createdInvitations[0]
	assert.Equal(t, "futureowner@example.com", inv.Email)
	assert.Equal(t, rbac.RoleShopOwner, inv.RoleKey)
	require.NotNil(t, inv.TenantID)
	assert.Equal(t, "tenant-1", *inv.TenantID)
	require.NotNil(t, inv.CommissionRate)
	assert.Equal(t, commissionRate, *inv.CommissionRate)

	// The returned InvitationToken must match the created invitation's token.
	assert.Equal(t, inv.Token, result.InvitationToken)

	// Both UserInvited and ShopCreated events must be published.
	var hasUserInvited, hasShopCreated bool
	for _, e := range pub.events {
		switch e.Type {
		case aggregate.EventUserInvited:
			hasUserInvited = true
		case aggregate.EventShopCreated:
			hasShopCreated = true
		}
	}
	assert.True(t, hasUserInvited, "UserInvited event must be published")
	assert.True(t, hasShopCreated, "ShopCreated event must be published")
}

// TestCreateShop_NoOwner verifies that passing an empty OwnerSpec (neither
// ExistingUserID nor InviteEmail set) returns ErrOwnerNotSpecified and does
// not call CreateTenant.
func TestCreateShop_NoOwner(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-owner-id",
		Key:       rbac.RoleShopOwner,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	_, err := svc.CreateShop(context.Background(), actorID, service.CreateShopInput{
		Name: "X",
		// Owner is zero-value: both ExistingUserID and InviteEmail are empty.
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrOwnerNotSpecified,
		"empty OwnerSpec must return ErrOwnerNotSpecified")
	assert.Empty(t, shops.createTenantCalls, "CreateTenant must not be called when owner is missing")
}

// TestCreateShop_BothOwners verifies that passing an OwnerSpec with both
// ExistingUserID and InviteEmail set returns ErrOwnerAmbiguous and does not
// call CreateTenant.
func TestCreateShop_BothOwners(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	rbacRepo.addRole(rbac.Role{
		ID:        "role-owner-id",
		Key:       rbac.RoleShopOwner,
		ScopeKind: rbac.ScopeShop,
	})
	shops := &adminStubShops{}
	pub := &noopPublisher{}

	svc := buildAdminSvc(t, repo, rbacRepo, shops, pub, now)

	_, err := svc.CreateShop(context.Background(), actorID, service.CreateShopInput{
		Name: "Ambiguous Shop",
		Owner: service.OwnerSpec{
			ExistingUserID: "existing-user-id",
			InviteEmail:    "invite@example.com",
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrOwnerAmbiguous,
		"both ExistingUserID and InviteEmail set must return ErrOwnerAmbiguous")
	assert.Empty(t, shops.createTenantCalls, "CreateTenant must not be called when owner is over-specified")
}

// ============================================================
// Tests: Custom-role CRUD (Phase D)
// ============================================================

func TestCreateCustomRole_PlatformAdmin_OK(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	svc := buildAdminSvc(t, repo, rbacRepo, &adminStubShops{}, &noopPublisher{}, now)

	roleID, err := svc.CreateCustomRole(context.Background(), actorID, service.CreateCustomRoleInput{
		Name:        "Support Agent",
		ScopeKind:   rbac.ScopeGlobal,
		Permissions: []rbac.Permission{rbac.CustomersRead},
	})
	require.NoError(t, err)
	require.NotEmpty(t, roleID)
	require.Len(t, rbacRepo.customRoles, 1)
}

func TestCreateCustomRole_NonAdminGlobalScope_Denied(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)
	svc := buildAdminSvc(t, repo, rbacRepo, &adminStubShops{}, &noopPublisher{}, now)

	_, err := svc.CreateCustomRole(context.Background(), actorID, service.CreateCustomRoleInput{
		Name:        "Escalated",
		ScopeKind:   rbac.ScopeGlobal,
		Permissions: []rbac.Permission{rbac.CustomersRead},
	})
	require.ErrorIs(t, err, service.ErrGrantNotAllowed)
	require.Empty(t, rbacRepo.customRoles)
}

func TestAssignCustomRole_PlatformAdmin_OK(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"
	targetID := "target-user-id"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	svc := buildAdminSvc(t, repo, rbacRepo, &adminStubShops{}, &noopPublisher{}, now)

	roleID, err := svc.CreateCustomRole(context.Background(), actorID, service.CreateCustomRoleInput{
		Name:        "Analyst",
		ScopeKind:   rbac.ScopeGlobal,
		Permissions: []rbac.Permission{rbac.DashboardRead},
	})
	require.NoError(t, err)

	err = svc.AssignCustomRole(context.Background(), actorID, targetID, roleID, nil)
	require.NoError(t, err)

	require.Len(t, rbacRepo.assignedRoles, 1)
	assert.Equal(t, roleID, rbacRepo.assignedRoles[0].RoleID)
	assert.Equal(t, targetID, rbacRepo.assignedRoles[0].UserID)
}

func TestAssignCustomRole_NotFound(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	svc := buildAdminSvc(t, repo, rbacRepo, &adminStubShops{}, &noopPublisher{}, now)

	err := svc.AssignCustomRole(context.Background(), actorID, "target", "ghost-role", nil)
	require.ErrorIs(t, err, rbac.ErrRoleNotFound)
}

func TestListCustomRoles_NonAdminForeignTenant_Denied(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "owner-user-id"
	shopA := "11111111-1111-1111-1111-111111111111"
	shopB := "22222222-2222-2222-2222-222222222222"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = shopOwnerBindings(actorID, shopA)
	svc := buildAdminSvc(t, repo, rbacRepo, &adminStubShops{}, &noopPublisher{}, now)

	_, err := svc.ListCustomRoles(context.Background(), actorID, &shopB)
	require.ErrorIs(t, err, service.ErrGrantNotAllowed)
}

func TestDeleteCustomRole_PlatformAdmin_OK(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	svc := buildAdminSvc(t, repo, rbacRepo, &adminStubShops{}, &noopPublisher{}, now)

	roleID, err := svc.CreateCustomRole(context.Background(), actorID, service.CreateCustomRoleInput{
		Name:        "Temp",
		ScopeKind:   rbac.ScopeGlobal,
		Permissions: []rbac.Permission{rbac.DashboardRead},
	})
	require.NoError(t, err)
	require.Len(t, rbacRepo.customRoles, 1)

	require.NoError(t, svc.DeleteCustomRole(context.Background(), actorID, roleID))
	require.Empty(t, rbacRepo.customRoles)
}

func TestDeleteCustomRole_NotFound(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	actorID := "admin-user-id"

	repo := newAdminStubRepo()
	rbacRepo := newAdminStubRBACRepo()
	rbacRepo.bindings = platformAdminBindings(actorID)
	svc := buildAdminSvc(t, repo, rbacRepo, &adminStubShops{}, &noopPublisher{}, now)

	err := svc.DeleteCustomRole(context.Background(), actorID, "ghost")
	require.ErrorIs(t, err, rbac.ErrRoleNotFound)
}
