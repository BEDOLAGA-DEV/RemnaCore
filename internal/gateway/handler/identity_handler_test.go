package handler

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/identitytest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Test Helpers ---

func newTestIdentityHandler(t *testing.T) (*IdentityHandler, *identitytest.MockRepository, *identitytest.MockPublisher) {
	t.Helper()

	repo := new(identitytest.MockRepository)
	pub := new(identitytest.MockPublisher)

	key := generateTestECDSAKey(t)
	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)

	svc := identity.NewService(repo, pub, jwtIssuer, clock.NewReal(), 15*time.Minute, 7*24*time.Hour)
	h := NewIdentityHandler(svc)
	return h, repo, pub
}

func generateTestECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

// --- Tests ---

func TestRegister_Success(t *testing.T) {
	h, repo, pub := newTestIdentityHandler(t)

	repo.On("GetUserByEmail", mock.Anything, "alice@example.com").Return(nil, identity.ErrNotFound)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*identity.PlatformUser")).Return(nil)
	repo.On("CreateEmailVerification", mock.Anything, mock.AnythingOfType("*identity.EmailVerification")).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	body := `{"email":"alice@example.com","password":"StrongP4ss"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp["user_id"])
	assert.Equal(t, "alice@example.com", resp["email"])
	assert.NotEmpty(t, resp["verification_token"])
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	h, repo, _ := newTestIdentityHandler(t)

	existing := &identity.PlatformUser{ID: "existing-id", Email: "alice@example.com"}
	repo.On("GetUserByEmail", mock.Anything, "alice@example.com").Return(existing, nil)

	body := `{"email":"alice@example.com","password":"StrongP4ss"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "already taken")
	repo.AssertExpectations(t)
}

func TestLogin_Success(t *testing.T) {
	h, repo, pub := newTestIdentityHandler(t)

	hash, err := authutil.HashPassword("StrongP4ss")
	require.NoError(t, err)

	user := &identity.PlatformUser{
		ID:            "user-1",
		Email:         "alice@example.com",
		PasswordHash:  hash,
		EmailVerified: true,
		Role:          identity.RoleCustomer,
	}

	repo.On("GetUserByEmail", mock.Anything, "alice@example.com").Return(user, nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*identity.Session")).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	body := `{"email":"alice@example.com","password":"StrongP4ss"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp["access_token"])
	assert.NotEmpty(t, resp["refresh_token"])
	assert.NotNil(t, resp["user"])
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestLogin_WrongPassword(t *testing.T) {
	h, repo, _ := newTestIdentityHandler(t)

	hash, err := authutil.HashPassword("StrongP4ss")
	require.NoError(t, err)

	user := &identity.PlatformUser{
		ID:            "user-1",
		Email:         "alice@example.com",
		PasswordHash:  hash,
		EmailVerified: true,
		Role:          identity.RoleCustomer,
	}

	repo.On("GetUserByEmail", mock.Anything, "alice@example.com").Return(user, nil)

	body := `{"email":"alice@example.com","password":"WrongPassword1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var resp map[string]any
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "invalid credentials")
	repo.AssertExpectations(t)
}

func TestMe_Success(t *testing.T) {
	h, repo, _ := newTestIdentityHandler(t)

	user := &identity.PlatformUser{
		ID:            "user-1",
		Email:         "alice@example.com",
		EmailVerified: true,
		Role:          identity.RoleCustomer,
	}

	repo.On("GetUserByID", mock.Anything, "user-1").Return(user, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	// Inject claims into context via middleware helper.
	ctx := context.WithValue(req.Context(), middleware.ClaimsContextKey, &authutil.UserClaims{
		UserID: "user-1",
		Email:  "alice@example.com",
		Role:   "customer",
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.Me(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "user-1", resp["id"])
	assert.Equal(t, "alice@example.com", resp["email"])
	repo.AssertExpectations(t)
}

// --- Password Reset Tests ---

func TestForgotPassword_Success(t *testing.T) {
	tests := []struct {
		name  string
		email string
		setup func(repo *identitytest.MockRepository, pub *identitytest.MockPublisher)
	}{
		{
			name:  "existing user receives reset",
			email: "alice@example.com",
			setup: func(repo *identitytest.MockRepository, pub *identitytest.MockPublisher) {
				user := &identity.PlatformUser{ID: "user-1", Email: "alice@example.com"}
				repo.On("GetUserByEmail", mock.Anything, "alice@example.com").Return(user, nil)
				repo.On("DeleteUserPasswordResets", mock.Anything, "user-1").Return(nil)
				repo.On("CreatePasswordReset", mock.Anything, mock.AnythingOfType("*identity.PasswordReset")).Return(nil)
				pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)
			},
		},
		{
			name:  "non-existent email still returns 200 (anti-enumeration)",
			email: "nobody@example.com",
			setup: func(repo *identitytest.MockRepository, pub *identitytest.MockPublisher) {
				repo.On("GetUserByEmail", mock.Anything, "nobody@example.com").Return(nil, identity.ErrNotFound)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, pub := newTestIdentityHandler(t)
			tt.setup(repo, pub)

			body := `{"email":"` + tt.email + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewBufferString(body))
			req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
			rec := httptest.NewRecorder()

			h.ForgotPassword(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)

			var resp map[string]any
			err := json.NewDecoder(rec.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Contains(t, resp["status"], "reset link")
			repo.AssertExpectations(t)
			pub.AssertExpectations(t)
		})
	}
}

func TestResetPassword_Success(t *testing.T) {
	h, repo, pub := newTestIdentityHandler(t)

	hash, err := authutil.HashPassword("OldPass123")
	require.NoError(t, err)

	reset := &identity.PasswordReset{
		ID:        "reset-1",
		UserID:    "user-1",
		Email:     "alice@example.com",
		Token:     "valid-token",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	user := &identity.PlatformUser{
		ID:           "user-1",
		Email:        "alice@example.com",
		PasswordHash: hash,
		Role:         identity.RoleCustomer,
	}

	repo.On("GetPasswordResetByToken", mock.Anything, "valid-token").Return(reset, nil)
	repo.On("GetUserByID", mock.Anything, "user-1").Return(user, nil)
	repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*identity.PlatformUser")).Return(nil)
	repo.On("DeleteUserSessions", mock.Anything, "user-1").Return(nil)
	repo.On("DeletePasswordReset", mock.Anything, "reset-1").Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	body := `{"token":"valid-token","new_password":"NewStr0ng!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.ResetPassword(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	err = json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "password reset successful", resp["status"])
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestResetPassword_WeakPassword(t *testing.T) {
	tests := []struct {
		name        string
		newPassword string
		wantStatus  int
		wantError   string
	}{
		{
			name:        "too short",
			newPassword: "Ab1",
			wantStatus:  http.StatusBadRequest,
			wantError:   "at least 8 characters",
		},
		{
			name:        "no uppercase",
			newPassword: "alllowercase1",
			wantStatus:  http.StatusBadRequest,
			wantError:   "uppercase",
		},
		{
			name:        "no digit",
			newPassword: "NoDigitHere",
			wantStatus:  http.StatusBadRequest,
			wantError:   "uppercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, _ := newTestIdentityHandler(t)

			reset := &identity.PasswordReset{
				ID:        "reset-1",
				UserID:    "user-1",
				Email:     "alice@example.com",
				Token:     "valid-token",
				ExpiresAt: time.Now().Add(time.Hour),
				CreatedAt: time.Now(),
			}

			repo.On("GetPasswordResetByToken", mock.Anything, "valid-token").Return(reset, nil)

			body := `{"token":"valid-token","new_password":"` + tt.newPassword + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBufferString(body))
			req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
			rec := httptest.NewRecorder()

			h.ResetPassword(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code, "weak password should return 400, not 500")

			var resp map[string]any
			err := json.NewDecoder(rec.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Contains(t, resp["error"], tt.wantError)
			repo.AssertExpectations(t)
		})
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	h, repo, _ := newTestIdentityHandler(t)

	repo.On("GetPasswordResetByToken", mock.Anything, "bad-token").Return(nil, identity.ErrNotFound)

	body := `{"token":"bad-token","new_password":"NewStr0ng!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.ResetPassword(rec, req)

	// ErrPasswordResetNotFound maps to 404 in mapServiceError.
	assert.Equal(t, http.StatusNotFound, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "password reset token not found")
	repo.AssertExpectations(t)
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	h, repo, _ := newTestIdentityHandler(t)

	reset := &identity.PasswordReset{
		ID:        "reset-1",
		UserID:    "user-1",
		Email:     "alice@example.com",
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-time.Hour), // expired
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}

	repo.On("GetPasswordResetByToken", mock.Anything, "expired-token").Return(reset, nil)

	body := `{"token":"expired-token","new_password":"NewStr0ng!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.ResetPassword(rec, req)

	assert.Equal(t, http.StatusGone, rec.Code)

	var resp map[string]any
	err := json.NewDecoder(rec.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "expired")
	repo.AssertExpectations(t)
}
