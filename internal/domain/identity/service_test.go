package identity_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/identitytest"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers ---

func newTestService(t *testing.T) (*identity.Service, *identitytest.MockRepository, *identitytest.MockPublisher) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)
	repo := new(identitytest.MockRepository)
	pub := new(identitytest.MockPublisher)

	svc := identity.NewService(repo, pub, txmanagertest.NoopTxRunner{}, jwtIssuer, clock.NewReal(), 15*time.Minute, 7*24*time.Hour)
	return svc, repo, pub
}

func hashedPassword(t *testing.T) string {
	t.Helper()
	h, err := authutil.HashPassword("StrongP4ss")
	require.NoError(t, err)
	return h
}

// --- Service Tests ---

func TestService_Register_Success(t *testing.T) {
	svc, repo, pub := newTestService(t)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "alice@example.com").Return(nil, identity.ErrNotFound)
	repo.On("CreateUser", ctx, mock.AnythingOfType("*aggregate.PlatformUser")).Return(nil)
	repo.On("CreateEmailVerification", ctx, mock.AnythingOfType("*aggregate.EmailVerification")).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	result, err := svc.Register(ctx, identity.RegisterInput{
		Email:    "alice@example.com",
		Password: "StrongP4ss",
	})

	require.NoError(t, err)
	assert.NotNil(t, result.User)
	assert.Equal(t, "alice@example.com", result.User.Email)
	assert.NotEmpty(t, result.VerificationToken)
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestService_Register_DuplicateEmail(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	existing := &identity.PlatformUser{ID: "existing-id", Email: "alice@example.com"}
	repo.On("GetUserByEmail", ctx, "alice@example.com").Return(existing, nil)

	_, err := svc.Register(ctx, identity.RegisterInput{
		Email:    "alice@example.com",
		Password: "StrongP4ss",
	})

	assert.ErrorIs(t, err, identity.ErrEmailTaken)
	repo.AssertExpectations(t)
}

func TestService_Login_Success(t *testing.T) {
	svc, repo, pub := newTestService(t)
	ctx := context.Background()

	hash := hashedPassword(t)
	user := &identity.PlatformUser{
		ID:            "user-1",
		Email:         "alice@example.com",
		PasswordHash:  hash,
		EmailVerified: true,
		Role:          identity.RoleCustomer,
	}

	repo.On("GetUserByEmail", ctx, "alice@example.com").Return(user, nil)
	repo.On("CreateSession", ctx, mock.AnythingOfType("*aggregate.Session")).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	result, err := svc.Login(ctx, identity.LoginInput{
		Email:    "alice@example.com",
		Password: "StrongP4ss",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Equal(t, user, result.User)
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestService_Login_WrongPassword(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	hash := hashedPassword(t)
	user := &identity.PlatformUser{
		ID:            "user-1",
		Email:         "alice@example.com",
		PasswordHash:  hash,
		EmailVerified: true,
		Role:          identity.RoleCustomer,
	}

	repo.On("GetUserByEmail", ctx, "alice@example.com").Return(user, nil)

	_, err := svc.Login(ctx, identity.LoginInput{
		Email:    "alice@example.com",
		Password: "WrongPassword1",
	})

	assert.ErrorIs(t, err, identity.ErrInvalidCredentials)
}

func TestService_Login_UserNotFound(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	repo.On("GetUserByEmail", ctx, "unknown@example.com").Return(nil, identity.ErrNotFound)

	_, err := svc.Login(ctx, identity.LoginInput{
		Email:    "unknown@example.com",
		Password: "StrongP4ss",
	})

	// Must return ErrInvalidCredentials, NOT ErrNotFound (security best practice)
	assert.ErrorIs(t, err, identity.ErrInvalidCredentials)
}

func TestService_SetupNeeded(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	repo.On("CountAdmins", ctx).Return(int64(0), nil).Once()
	need, err := svc.SetupNeeded(ctx)
	require.NoError(t, err)
	assert.True(t, need)

	repo.On("CountAdmins", ctx).Return(int64(1), nil).Once()
	need, err = svc.SetupNeeded(ctx)
	require.NoError(t, err)
	assert.False(t, need)
	repo.AssertExpectations(t)
}

func TestService_CreateFirstAdmin_Success(t *testing.T) {
	svc, repo, pub := newTestService(t)
	ctx := context.Background()

	repo.On("AcquireBootstrapLock", mock.Anything).Return(nil)
	repo.On("CountAdmins", mock.Anything).Return(int64(0), nil)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*aggregate.PlatformUser")).Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*aggregate.Session")).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	result, err := svc.CreateFirstAdmin(ctx, identity.CreateFirstAdminInput{
		Email:    "admin@example.com",
		Password: "StrongP4ss",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Equal(t, identity.RoleAdmin, result.User.Role)
	assert.Equal(t, "admin@example.com", result.User.Email)
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestService_CreateFirstAdmin_AlreadyCompleted(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	// An admin already exists: the count guard rejects the second bootstrap.
	repo.On("AcquireBootstrapLock", mock.Anything).Return(nil)
	repo.On("CountAdmins", mock.Anything).Return(int64(1), nil)

	_, err := svc.CreateFirstAdmin(ctx, identity.CreateFirstAdminInput{
		Email:    "admin2@example.com",
		Password: "StrongP4ss",
	})

	assert.ErrorIs(t, err, identity.ErrSetupAlreadyCompleted)
	// CreateUser must never be reached when setup is already completed.
	repo.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)
}

func TestService_CreateFirstAdmin_WeakPassword(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.CreateFirstAdmin(ctx, identity.CreateFirstAdminInput{
		Email:    "admin@example.com",
		Password: "weak",
	})

	require.Error(t, err)
}

func TestService_VerifyEmail_Success(t *testing.T) {
	svc, repo, pub := newTestService(t)
	ctx := context.Background()

	verification := &identity.EmailVerification{
		ID:        "v-1",
		UserID:    "user-1",
		Email:     "alice@example.com",
		Token:     "abc123",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	user := &identity.PlatformUser{
		ID:            "user-1",
		Email:         "alice@example.com",
		EmailVerified: false,
	}

	repo.On("GetEmailVerification", ctx, "abc123").Return(verification, nil)
	repo.On("GetUserByIDForUpdate", ctx, "user-1").Return(user, nil)
	repo.On("UpdateUser", ctx, mock.AnythingOfType("*aggregate.PlatformUser")).Return(nil)
	repo.On("DeleteEmailVerification", ctx, "v-1").Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.VerifyEmail(ctx, "abc123")

	require.NoError(t, err)
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestService_VerifyEmail_Expired(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	verification := &identity.EmailVerification{
		ID:        "v-1",
		UserID:    "user-1",
		Email:     "alice@example.com",
		Token:     "abc123",
		ExpiresAt: time.Now().Add(-time.Hour), // expired
	}

	repo.On("GetEmailVerification", ctx, "abc123").Return(verification, nil)

	err := svc.VerifyEmail(ctx, "abc123")

	assert.ErrorIs(t, err, identity.ErrTokenExpired)
}

func TestService_RefreshToken_Success(t *testing.T) {
	svc, repo, pub := newTestService(t)
	ctx := context.Background()

	user := &identity.PlatformUser{
		ID:    "user-1",
		Email: "alice@example.com",
		Role:  identity.RoleCustomer,
	}
	session := &identity.Session{
		ID:           "s-1",
		UserID:       "user-1",
		RefreshToken: "old-refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	repo.On("GetSessionByRefreshToken", mock.Anything, "old-refresh-token").Return(session, nil)
	repo.On("GetUserByID", mock.Anything, "user-1").Return(user, nil)
	repo.On("DeleteSession", mock.Anything, "s-1").Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*aggregate.Session")).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	result, err := svc.RefreshToken(ctx, "old-refresh-token")

	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.Equal(t, user, result.User)
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestService_RefreshToken_CreateSessionFails_RollsBack(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	user := &identity.PlatformUser{
		ID:    "user-1",
		Email: "alice@example.com",
		Role:  identity.RoleCustomer,
	}
	session := &identity.Session{
		ID:           "s-1",
		UserID:       "user-1",
		RefreshToken: "old-refresh-token",
		ExpiresAt:    time.Now().Add(time.Hour),
	}

	createSessionErr := errors.New("db connection lost")

	repo.On("GetSessionByRefreshToken", mock.Anything, "old-refresh-token").Return(session, nil)
	repo.On("GetUserByID", mock.Anything, "user-1").Return(user, nil)
	repo.On("DeleteSession", mock.Anything, "s-1").Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*aggregate.Session")).Return(createSessionErr)

	result, err := svc.RefreshToken(ctx, "old-refresh-token")

	// CreateSession failed inside the transaction, so the whole tx (including
	// DeleteSession) is rolled back by txRunner. The caller sees the error.
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.ErrorIs(t, err, createSessionErr)

	// DeleteSession was called (it executes before CreateSession inside the tx),
	// but because the tx rolls back, the deletion is NOT persisted.
	// With NoopTxRunner the rollback is semantic — the important thing is that
	// both operations are inside the same RunInTx call, guaranteeing atomicity
	// with a real transaction manager.
	repo.AssertExpectations(t)
}

func TestService_RefreshToken_Expired(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ctx := context.Background()

	session := &identity.Session{
		ID:           "s-1",
		UserID:       "user-1",
		RefreshToken: "old-refresh-token",
		ExpiresAt:    time.Now().Add(-time.Hour), // expired
	}

	repo.On("GetSessionByRefreshToken", ctx, "old-refresh-token").Return(session, nil)

	_, err := svc.RefreshToken(ctx, "old-refresh-token")

	assert.ErrorIs(t, err, identity.ErrSessionExpired)
}
