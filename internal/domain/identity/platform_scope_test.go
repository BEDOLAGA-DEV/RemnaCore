package identity_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// These tests guard against the regression class that broke the deploy: the
// pre-auth identity paths read/write identity.platform_users, which is FORCE
// RLS with NO tenant_id-IS-NULL read branch. Under the non-superuser runtime
// role they see ZERO rows unless the platform sentinel GUC is set. The bug was
// forgetting RunInTx(WithPlatformScope(ctx)); a plain ctx silently ran unscoped.
//
// Each test asserts the CONTEXT passed to the repository carries the platform
// sentinel. If someone removes the WithPlatformScope wrap, the matcher stops
// matching and the test fails at merge — without needing a real database.

// platformScoped matches a context whose app.tenant_id resolves to the platform
// sentinel ("*"). NoopTxRunner passes the RunInTx context straight through, so
// the repository sees exactly what the service scoped it with.
var platformScoped = mock.MatchedBy(func(ctx context.Context) bool {
	return tenantctx.TenantIDFromContext(ctx) == tenantctx.PlatformScopeSentinel
})

func TestPlatformScope_SetupNeeded(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.On("CountAdmins", platformScoped).Return(int64(1), nil)

	needed, err := svc.SetupNeeded(context.Background())
	require.NoError(t, err)
	require.False(t, needed)
	repo.AssertExpectations(t) // fails if CountAdmins wasn't called platform-scoped
}

func TestPlatformScope_Login(t *testing.T) {
	svc, repo, pub := newTestService(t)
	hash := hashedPassword(t)
	user := &identity.PlatformUser{ID: "u1", Email: "a@b.com", PasswordHash: hash, Role: identity.RoleCustomer}

	repo.On("GetUserByEmail", platformScoped, "a@b.com").Return(user, nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*aggregate.Session")).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	_, err := svc.Login(context.Background(), identity.LoginInput{Email: "a@b.com", Password: "StrongP4ss"})
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPlatformScope_RefreshToken(t *testing.T) {
	svc, repo, pub := newTestService(t)
	user := &identity.PlatformUser{ID: "u1", Email: "a@b.com", Role: identity.RoleCustomer}
	session := &identity.Session{ID: "s1", UserID: "u1", RefreshToken: "rt", ExpiresAt: time.Now().Add(time.Hour)}

	repo.On("GetSessionByRefreshToken", mock.Anything, "rt").Return(session, nil)
	repo.On("GetUserByID", platformScoped, "u1").Return(user, nil)
	repo.On("DeleteSession", mock.Anything, "s1").Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*aggregate.Session")).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	_, err := svc.RefreshToken(context.Background(), "rt")
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPlatformScope_Register(t *testing.T) {
	svc, repo, pub := newTestService(t)
	repo.On("GetUserByEmail", platformScoped, "new@b.com").Return(nil, identity.ErrNotFound)
	repo.On("CreateUser", platformScoped, mock.AnythingOfType("*aggregate.PlatformUser")).Return(nil)
	repo.On("CreateEmailVerification", platformScoped, mock.AnythingOfType("*aggregate.EmailVerification")).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	_, err := svc.Register(context.Background(), identity.RegisterInput{Email: "new@b.com", Password: "StrongP4ss"})
	require.NoError(t, err)
	repo.AssertExpectations(t)
}
