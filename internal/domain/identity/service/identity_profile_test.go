package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// profileStubRepo is a minimal stub for service.Repository used by profile
// (ResetPassword) tests. It embeds adminStubRepo for all the no-op methods and
// overrides only the methods exercised by ResetPassword.
type profileStubRepo struct {
	adminStubRepo

	resetsByToken map[string]*aggregate.PasswordReset
	updated       []*aggregate.PlatformUser
}

func newProfileStubRepo() *profileStubRepo {
	return &profileStubRepo{
		adminStubRepo: adminStubRepo{
			invitations:  map[string]*aggregate.Invitation{},
			usersByEmail: map[string]*aggregate.PlatformUser{},
			usersByID:    map[string]*aggregate.PlatformUser{},
		},
		resetsByToken: map[string]*aggregate.PasswordReset{},
	}
}

func (r *profileStubRepo) GetPasswordResetByToken(_ context.Context, token string) (*aggregate.PasswordReset, error) {
	if pr, ok := r.resetsByToken[token]; ok {
		return pr, nil
	}
	return nil, service.ErrNotFound
}

func (r *profileStubRepo) UpdateUser(_ context.Context, u *aggregate.PlatformUser) error {
	r.updated = append(r.updated, u)
	return nil
}

// Override from embedded adminStubRepo to be a no-op (needed by ResetPassword).
func (r *profileStubRepo) DeleteUserSessions(_ context.Context, _ string) error { return nil }
func (r *profileStubRepo) DeletePasswordReset(_ context.Context, _ string) error { return nil }

// buildProfileSvc constructs a service.Service suitable for testing profile
// operations using the given repo stub.
func buildProfileSvc(t *testing.T, repo service.Repository, fixedNow time.Time) *service.Service {
	t.Helper()
	jwtIssuer := newTestJWT(t)
	pub := &noopPublisher{}
	clk := clock.NewMock(fixedNow)
	sessions := service.NewSessionIssuer(repo, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
	return service.NewService(
		repo,
		pub,
		txmanagertest.NoopTxRunner{},
		jwtIssuer,
		clk,
		15*time.Minute,
		7*24*time.Hour,
		sessions,
	)
}

// TestResetPassword_ClearsMustChangePassword verifies that a successful
// ResetPassword call clears the must_change_password flag on the persisted user.
// Admin-created accounts have MustChangePassword=true; after the user completes
// the password-reset flow the flag must be cleared so they are not prompted again.
func TestResetPassword_ClearsMustChangePassword(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	repo := newProfileStubRepo()

	// Build a user who has MustChangePassword=true (simulating an admin-created account).
	hash, err := authutil.HashPassword("OldP4ssw0rd")
	require.NoError(t, err)
	user := &aggregate.PlatformUser{
		ID:                 "user-id-1",
		Email:              "user@example.com",
		PasswordHash:       hash,
		EmailVerified:      true,
		MustChangePassword: true,
	}
	repo.usersByID[user.ID] = user

	// Create a valid (not-yet-expired) password reset token.
	reset, err := aggregate.NewPasswordReset(user.ID, user.Email, now)
	require.NoError(t, err)
	repo.resetsByToken[reset.Token] = reset

	svc := buildProfileSvc(t, repo, now)

	err = svc.ResetPassword(context.Background(), reset.Token, "NewStr0ngP4ss")
	require.NoError(t, err)

	// UpdateUser must have been called with MustChangePassword=false.
	require.Len(t, repo.updated, 1, "UpdateUser must be called exactly once")
	assert.False(t, repo.updated[0].MustChangePassword,
		"must_change_password must be cleared after a successful password reset")
}

// TestResetPassword_ExpiredToken verifies that an expired reset token is rejected
// and no user update is performed.
func TestResetPassword_ExpiredToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	pastTime := now.Add(-aggregate.PasswordResetTTL - time.Second)

	repo := newProfileStubRepo()

	hash, err := authutil.HashPassword("OldP4ssw0rd")
	require.NoError(t, err)
	user := &aggregate.PlatformUser{
		ID:           "user-id-2",
		Email:        "user2@example.com",
		PasswordHash: hash,
	}
	repo.usersByID[user.ID] = user

	// Create an expired reset token (created far in the past).
	reset, err := aggregate.NewPasswordReset(user.ID, user.Email, pastTime)
	require.NoError(t, err)
	repo.resetsByToken[reset.Token] = reset

	svc := buildProfileSvc(t, repo, now)

	err = svc.ResetPassword(context.Background(), reset.Token, "NewStr0ngP4ss")
	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrPasswordResetExpired)
	assert.Empty(t, repo.updated, "no user update must occur for an expired token")
}
