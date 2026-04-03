//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
)

const (
	testDBName = "vpn_test"
	testDBUser = "test"
	testDBPass = "test"
)

// setupTestDB starts a PostgreSQL 18 container, applies the migration, and
// returns a connected pool. The container is terminated when the test finishes.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	migrationPath, err := filepath.Abs("migrations")
	require.NoError(t, err)

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		tcpostgres.WithInitScripts(filepath.Join(migrationPath, "001_identity.sql")),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(testContainerStartupTimeout),
		),
	)
	if err != nil {
		t.Skipf("skipping integration test: could not start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

func newTestUser(t *testing.T) *identity.PlatformUser {
	t.Helper()
	now := time.Now().Truncate(time.Microsecond)
	return &identity.PlatformUser{
		ID:            uuid.New().String(),
		Email:         fmt.Sprintf("user-%s@test.com", uuid.New().String()[:8]),
		PasswordHash:  "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012",
		DisplayName:   "",
		EmailVerified: false,
		Role:          identity.RoleCustomer,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func TestIdentityRepo_CreateAndGetUser(t *testing.T) {
	pool := setupTestDB(t)
	repo := postgres.NewIdentityRepository(pool)
	ctx := context.Background()

	user := newTestUser(t)

	// Create
	err := repo.CreateUser(ctx, user)
	require.NoError(t, err)

	// Get by email
	got, err := repo.GetUserByEmail(ctx, user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, user.Email, got.Email)
	assert.Equal(t, user.PasswordHash, got.PasswordHash)
	assert.Equal(t, user.Role, got.Role)
	assert.False(t, got.EmailVerified)

	// Get by ID
	got2, err := repo.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got2.ID)
	assert.Equal(t, user.Email, got2.Email)

	// Not found
	_, err = repo.GetUserByID(ctx, uuid.New().String())
	assert.ErrorIs(t, err, identity.ErrNotFound)

	_, err = repo.GetUserByEmail(ctx, "nonexistent@test.com")
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestIdentityRepo_DuplicateEmail(t *testing.T) {
	pool := setupTestDB(t)
	repo := postgres.NewIdentityRepository(pool)
	ctx := context.Background()

	user := newTestUser(t)
	err := repo.CreateUser(ctx, user)
	require.NoError(t, err)

	// Duplicate with same email (different casing)
	dup := newTestUser(t)
	dup.Email = user.Email
	err = repo.CreateUser(ctx, dup)
	assert.Error(t, err, "expected unique constraint violation for duplicate email")
}

func TestIdentityRepo_SessionLifecycle(t *testing.T) {
	pool := setupTestDB(t)
	repo := postgres.NewIdentityRepository(pool)
	ctx := context.Background()

	user := newTestUser(t)
	require.NoError(t, repo.CreateUser(ctx, user))

	now := time.Now().Truncate(time.Microsecond)
	session := &identity.Session{
		ID:           uuid.New().String(),
		UserID:       user.ID,
		RefreshToken: uuid.New().String(),
		ExpiresAt:    now.Add(24 * time.Hour),
		CreatedAt:    now,
	}

	// Create session
	err := repo.CreateSession(ctx, session)
	require.NoError(t, err)

	// Get by refresh token
	got, err := repo.GetSessionByRefreshToken(ctx, session.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, session.ID, got.ID)
	assert.Equal(t, session.UserID, got.UserID)
	assert.Equal(t, session.RefreshToken, got.RefreshToken)

	// Not found
	_, err = repo.GetSessionByRefreshToken(ctx, "nonexistent-token")
	assert.ErrorIs(t, err, identity.ErrNotFound)

	// Delete session
	err = repo.DeleteSession(ctx, session.ID)
	require.NoError(t, err)

	_, err = repo.GetSessionByRefreshToken(ctx, session.RefreshToken)
	assert.ErrorIs(t, err, identity.ErrNotFound)

	// Delete user sessions (create two, delete all)
	s1 := &identity.Session{
		ID: uuid.New().String(), UserID: user.ID,
		RefreshToken: uuid.New().String(),
		ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}
	s2 := &identity.Session{
		ID: uuid.New().String(), UserID: user.ID,
		RefreshToken: uuid.New().String(),
		ExpiresAt: now.Add(time.Hour), CreatedAt: now,
	}
	require.NoError(t, repo.CreateSession(ctx, s1))
	require.NoError(t, repo.CreateSession(ctx, s2))

	err = repo.DeleteUserSessions(ctx, user.ID)
	require.NoError(t, err)

	_, err = repo.GetSessionByRefreshToken(ctx, s1.RefreshToken)
	assert.ErrorIs(t, err, identity.ErrNotFound)
	_, err = repo.GetSessionByRefreshToken(ctx, s2.RefreshToken)
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestIdentityRepo_EmailVerification(t *testing.T) {
	pool := setupTestDB(t)
	repo := postgres.NewIdentityRepository(pool)
	ctx := context.Background()

	user := newTestUser(t)
	require.NoError(t, repo.CreateUser(ctx, user))

	now := time.Now().Truncate(time.Microsecond)
	verification := &identity.EmailVerification{
		ID:        uuid.New().String(),
		UserID:    user.ID,
		Email:     user.Email,
		Token:     uuid.New().String(),
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}

	// Create
	err := repo.CreateEmailVerification(ctx, verification)
	require.NoError(t, err)

	// Get by token
	got, err := repo.GetEmailVerification(ctx, verification.Token)
	require.NoError(t, err)
	assert.Equal(t, verification.ID, got.ID)
	assert.Equal(t, verification.UserID, got.UserID)
	assert.Equal(t, verification.Email, got.Email)
	assert.Equal(t, verification.Token, got.Token)

	// Not found
	_, err = repo.GetEmailVerification(ctx, "nonexistent-token")
	assert.ErrorIs(t, err, identity.ErrNotFound)

	// Delete
	err = repo.DeleteEmailVerification(ctx, verification.ID)
	require.NoError(t, err)

	_, err = repo.GetEmailVerification(ctx, verification.Token)
	assert.ErrorIs(t, err, identity.ErrNotFound)
}
