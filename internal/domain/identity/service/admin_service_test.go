package service_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
	invitations    map[string]*aggregate.Invitation // keyed by token
	usersByEmail   map[string]*aggregate.PlatformUser
	sessions       []*aggregate.Session

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
	}
}

func (r *adminStubRepo) CreateUser(_ context.Context, u *aggregate.PlatformUser) error {
	r.createdUsers = append(r.createdUsers, u)
	return nil
}
func (r *adminStubRepo) GetUserByID(_ context.Context, _ string) (*aggregate.PlatformUser, error) {
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) GetUserByIDForUpdate(_ context.Context, _ string) (*aggregate.PlatformUser, error) {
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
func (r *adminStubRepo) CountAdmins(_ context.Context) (int64, error)   { return 0, nil }
func (r *adminStubRepo) AcquireBootstrapLock(_ context.Context) error    { return nil }
func (r *adminStubRepo) CreateSession(_ context.Context, s *aggregate.Session) error {
	r.sessions = append(r.sessions, s)
	return nil
}
func (r *adminStubRepo) GetSessionByRefreshToken(_ context.Context, _ string) (*aggregate.Session, error) {
	return nil, service.ErrNotFound
}
func (r *adminStubRepo) DeleteSession(_ context.Context, _ string) error          { return nil }
func (r *adminStubRepo) DeleteUserSessions(_ context.Context, _ string) error     { return nil }
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
func (r *adminStubRepo) DeletePasswordReset(_ context.Context, _ string) error         { return nil }
func (r *adminStubRepo) DeleteUserPasswordResets(_ context.Context, _ string) error    { return nil }
func (r *adminStubRepo) DeleteExpiredSessions(_ context.Context) (int64, error)        { return 0, nil }
func (r *adminStubRepo) DeleteExpiredVerifications(_ context.Context) (int64, error)   { return 0, nil }
func (r *adminStubRepo) DeleteExpiredPasswordResets(_ context.Context) (int64, error)  { return 0, nil }
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
func (r *adminStubRepo) ListInvitations(_ context.Context, _ []string, _ bool) ([]*aggregate.Invitation, error) {
	return nil, nil
}
func (r *adminStubRepo) DeleteExpiredInvitations(_ context.Context) (int64, error) { return 0, nil }

// ---- rbac stub ----

// adminStubRBACRepo is a hand-written stub for rbac.Repository.
// It stores roles in a map keyed by key string and records AssignRole calls.
type adminStubRBACRepo struct {
	roles         map[string]rbac.Role
	bindings      map[string][]rbac.Binding // keyed by userID
	assignedRoles []assignedRoleRecord
}

type assignedRoleRecord struct {
	UserID    string
	RoleID    string
	TenantID  *string
	GrantedBy string
}

func newAdminStubRBACRepo() *adminStubRBACRepo {
	return &adminStubRBACRepo{
		roles:    map[string]rbac.Role{},
		bindings: map[string][]rbac.Binding{},
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

func (r *adminStubRBACRepo) RevokeRole(_ context.Context, _, _ string, _ *string) (int64, error) {
	return 0, nil
}

func (r *adminStubRBACRepo) GetRole(_ context.Context, key string) (rbac.Role, error) {
	if role, ok := r.roles[key]; ok {
		return role, nil
	}
	return rbac.Role{}, rbac.ErrRoleNotFound
}

func (r *adminStubRBACRepo) CountPlatformAdmins(_ context.Context) (int, error) { return 1, nil }

// ---- shop provisioner stub ----

type adminStubShops struct {
	setTenantOwnerCalls      []setTenantOwnerCall
	createResellerCalls      []createResellerCall
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

// buildAdminSvc creates a ready-to-use IdentityAdminService with the given
// stubs wired together.
func buildAdminSvc(
	t *testing.T,
	repo *adminStubRepo,
	rbacRepo *adminStubRBACRepo,
	shops *adminStubShops,
	pub domainevent.Publisher,
	fixedNow time.Time,
) *service.IdentityAdminService {
	t.Helper()

	jwtIssuer := newTestJWT(t)
	issuer := service.NewSessionIssuer(repo, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
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

	// User must be created.
	require.Len(t, repo.createdUsers, 1)
	assert.Equal(t, "staff@example.com", repo.createdUsers[0].Email)

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
