package integration_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/identitytest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	testAccessTTL  = 15 * time.Minute
	testRefreshTTL = 7 * 24 * time.Hour

	testEmail    = "integration@example.com"
	testPassword = "IntTest!2026"
)

// identityTestHarness bundles the router, mocks, and JWT issuer used across
// identity integration subtests.
type identityTestHarness struct {
	router http.Handler
	repo   *identitytest.MockRepository
	pub    *identitytest.MockPublisher
	jwt    *authutil.JWTIssuer
}

// newIdentityTestHarness creates a chi router with only the identity-related
// routes wired to a real identity.Service backed by mock repositories.
func newIdentityTestHarness(t *testing.T) *identityTestHarness {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)
	repo := new(identitytest.MockRepository)
	pub := new(identitytest.MockPublisher)

	svc := identity.NewService(repo, pub, jwtIssuer, clock.NewReal(), testAccessTTL, testRefreshTTL)
	h := handler.NewIdentityHandler(svc)

	r := chi.NewRouter()
	r.Post("/api/auth/register", h.Register)
	r.Post("/api/auth/login", h.Login)
	r.Post("/api/auth/verify-email", h.VerifyEmail)
	r.Post("/api/auth/refresh", h.RefreshToken)
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.Auth(jwtIssuer))
		protected.Get("/api/me", h.Me)
	})

	return &identityTestHarness{
		router: r,
		repo:   repo,
		pub:    pub,
		jwt:    jwtIssuer,
	}
}

// TestIdentityFlow validates the full identity lifecycle through the HTTP layer:
// register -> login -> access protected endpoint -> verify email -> refresh token.
//
// Each subtest operates against a real chi router with real handlers and a real
// identity service, backed by mock repositories. No Docker or external infra
// required.
func TestIdentityFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Shared state populated by earlier subtests and consumed by later ones.
	var (
		userID            string
		verificationToken string
		accessToken       string
		refreshToken      string
	)

	t.Run("register new user", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		h.repo.On("GetUserByEmail", mock.Anything, testEmail).Return(nil, identity.ErrNotFound)
		h.repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*identity.PlatformUser")).Return(nil)
		h.repo.On("CreateEmailVerification", mock.Anything, mock.AnythingOfType("*identity.EmailVerification")).Return(nil)
		h.pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

		body := `{"email":"` + testEmail + `","password":"` + testPassword + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["user_id"])
		assert.Equal(t, testEmail, resp["email"])
		assert.NotEmpty(t, resp["verification_token"])

		userID = resp["user_id"].(string)
		verificationToken = resp["verification_token"].(string)

		h.repo.AssertExpectations(t)
		h.pub.AssertExpectations(t)
	})

	t.Run("register duplicate email returns 409", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		existing := &identity.PlatformUser{ID: "existing-id", Email: testEmail}
		h.repo.On("GetUserByEmail", mock.Anything, testEmail).Return(existing, nil)

		body := `{"email":"` + testEmail + `","password":"` + testPassword + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusConflict, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Contains(t, resp["error"], "already taken")

		h.repo.AssertExpectations(t)
	})

	t.Run("login", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		hash, err := authutil.HashPassword(testPassword)
		require.NoError(t, err)

		user := &identity.PlatformUser{
			ID:            userID,
			Email:         testEmail,
			PasswordHash:  hash,
			EmailVerified: true,
			Role:          identity.RoleCustomer,
		}

		h.repo.On("GetUserByEmail", mock.Anything, testEmail).Return(user, nil)
		h.repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*identity.Session")).Return(nil)
		h.pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

		body := `{"email":"` + testEmail + `","password":"` + testPassword + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err = json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["access_token"])
		assert.NotEmpty(t, resp["refresh_token"])
		assert.NotNil(t, resp["user"])

		accessToken = resp["access_token"].(string)
		refreshToken = resp["refresh_token"].(string)

		h.repo.AssertExpectations(t)
		h.pub.AssertExpectations(t)
	})

	t.Run("login with wrong password returns 401", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		hash, err := authutil.HashPassword(testPassword)
		require.NoError(t, err)

		user := &identity.PlatformUser{
			ID:            userID,
			Email:         testEmail,
			PasswordHash:  hash,
			EmailVerified: true,
			Role:          identity.RoleCustomer,
		}

		h.repo.On("GetUserByEmail", mock.Anything, testEmail).Return(user, nil)

		body := `{"email":"` + testEmail + `","password":"WrongPassword1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var resp map[string]interface{}
		err = json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Contains(t, resp["error"], "invalid credentials")

		h.repo.AssertExpectations(t)
	})

	t.Run("access protected endpoint with valid token", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		// The access token was issued by a different harness, so we need to
		// issue one from this harness's JWT issuer for the middleware to accept it.
		token, err := h.jwt.Sign(authutil.UserClaims{
			UserID: userID,
			Email:  testEmail,
			Role:   string(identity.RoleCustomer),
		}, testAccessTTL)
		require.NoError(t, err)

		user := &identity.PlatformUser{
			ID:            userID,
			Email:         testEmail,
			EmailVerified: true,
			Role:          identity.RoleCustomer,
		}

		h.repo.On("GetUserByID", mock.Anything, userID).Return(user, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err = json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, userID, resp["id"])
		assert.Equal(t, testEmail, resp["email"])

		h.repo.AssertExpectations(t)
	})

	t.Run("access protected endpoint without token returns 401", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["error"])
	})

	t.Run("access protected endpoint with malformed token returns 401", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+"not-a-real-jwt")
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("verify email", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		// Use a deterministic token for the mock setup.
		vToken := verificationToken
		if vToken == "" {
			vToken = "test-verification-token"
		}

		verification := &identity.EmailVerification{
			ID:        "v-1",
			UserID:    userID,
			Email:     testEmail,
			Token:     vToken,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		user := &identity.PlatformUser{
			ID:            userID,
			Email:         testEmail,
			EmailVerified: false,
			Role:          identity.RoleCustomer,
		}

		h.repo.On("GetEmailVerification", mock.Anything, vToken).Return(verification, nil)
		h.repo.On("GetUserByID", mock.Anything, userID).Return(user, nil)
		h.repo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*identity.PlatformUser")).Return(nil)
		h.repo.On("DeleteEmailVerification", mock.Anything, "v-1").Return(nil)
		h.pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

		body := `{"token":"` + vToken + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/verify-email", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "verified", resp["status"])

		h.repo.AssertExpectations(t)
		h.pub.AssertExpectations(t)
	})

	t.Run("verify email with expired token returns 410", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		verification := &identity.EmailVerification{
			ID:        "v-2",
			UserID:    userID,
			Email:     testEmail,
			Token:     "expired-token",
			ExpiresAt: time.Now().Add(-time.Hour),
		}

		h.repo.On("GetEmailVerification", mock.Anything, "expired-token").Return(verification, nil)

		body := `{"token":"expired-token"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/verify-email", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusGone, rec.Code)

		h.repo.AssertExpectations(t)
	})

	t.Run("refresh token", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		// Use a deterministic refresh token for mock setup.
		rToken := refreshToken
		if rToken == "" {
			rToken = "test-refresh-token"
		}

		user := &identity.PlatformUser{
			ID:    userID,
			Email: testEmail,
			Role:  identity.RoleCustomer,
		}
		session := &identity.Session{
			ID:           "s-1",
			UserID:       userID,
			RefreshToken: rToken,
			ExpiresAt:    time.Now().Add(time.Hour),
		}

		h.repo.On("GetSessionByRefreshToken", mock.Anything, rToken).Return(session, nil)
		h.repo.On("GetUserByID", mock.Anything, userID).Return(user, nil)
		h.repo.On("DeleteSession", mock.Anything, "s-1").Return(nil)
		h.repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*identity.Session")).Return(nil)

		body := `{"refresh_token":"` + rToken + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["access_token"])
		assert.NotEmpty(t, resp["refresh_token"])

		// New tokens should be issued.
		newAccess := resp["access_token"].(string)
		newRefresh := resp["refresh_token"].(string)
		assert.NotEqual(t, accessToken, newAccess)
		assert.NotEqual(t, rToken, newRefresh)

		h.repo.AssertExpectations(t)
	})

	t.Run("refresh with expired session returns 401", func(t *testing.T) {
		h := newIdentityTestHarness(t)

		session := &identity.Session{
			ID:           "s-2",
			UserID:       userID,
			RefreshToken: "expired-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
		}

		h.repo.On("GetSessionByRefreshToken", mock.Anything, "expired-refresh").Return(session, nil)

		body := `{"refresh_token":"expired-refresh"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		h.repo.AssertExpectations(t)
	})
}
