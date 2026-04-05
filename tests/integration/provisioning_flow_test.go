package integration_test

import (
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
	multisubaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/multisubtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	provisionTestUserID = "prov-user-1"
	provisionTestEmail  = "provision@example.com"
)

// provisioningTestHarness bundles the router, mocks, and JWT issuer used
// across provisioning integration subtests.
type provisioningTestHarness struct {
	router   http.Handler
	bindings *multisubtest.MockBindingRepo
	jwt      *authutil.JWTIssuer
}

// newProvisioningTestHarness creates a chi router with multisub binding routes
// wired to a real MultiSubHandler backed by mock repositories.
func newProvisioningTestHarness(t *testing.T) *provisioningTestHarness {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)
	bindings := new(multisubtest.MockBindingRepo)

	mh := handler.NewMultiSubHandler(bindings)

	r := chi.NewRouter()
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.Auth(jwtIssuer))
		protected.Get("/api/bindings", mh.GetMyBindings)
		protected.Get("/api/subscriptions/{subID}/bindings", mh.GetBindingsBySubscription)
	})

	return &provisioningTestHarness{
		router:   r,
		bindings: bindings,
		jwt:      jwtIssuer,
	}
}

// provisionAccessToken creates a signed JWT for the provisioning test user.
func provisionAccessToken(t *testing.T, jwtIssuer *authutil.JWTIssuer) string {
	t.Helper()
	token, err := jwtIssuer.Sign(authutil.UserClaims{
		UserID: provisionTestUserID,
		Email:  provisionTestEmail,
		Role:   string(identity.RoleCustomer),
	}, testAccessTTL)
	require.NoError(t, err)
	return token
}

// TestProvisioningFlow validates the provisioning-related HTTP handlers:
// listing bindings (both user-level and per-subscription) and auth enforcement.
func TestProvisioningFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("bindings list requires auth", func(t *testing.T) {
		h := newProvisioningTestHarness(t)

		req := httptest.NewRequest(http.MethodGet, "/api/bindings", nil)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		var resp map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp["error"])
	})

	t.Run("get my bindings returns empty array", func(t *testing.T) {
		h := newProvisioningTestHarness(t)
		token := provisionAccessToken(t, h.jwt)

		h.bindings.On("GetByPlatformUserID", mock.Anything, provisionTestUserID).
			Return([]*multisubaggregate.RemnawaveBinding{}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/bindings", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []*multisubaggregate.RemnawaveBinding
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Empty(t, resp)

		h.bindings.AssertExpectations(t)
	})

	t.Run("get my bindings returns active bindings", func(t *testing.T) {
		h := newProvisioningTestHarness(t)
		token := provisionAccessToken(t, h.jwt)

		now := time.Now()
		activeBindings := []*multisubaggregate.RemnawaveBinding{
			{
				ID:                 "bind-1",
				SubscriptionID:     "sub-1",
				PlatformUserID:     provisionTestUserID,
				RemnawaveUUID:      "rw-uuid-1",
				RemnawaveShortUUID: "rw-short-1",
				RemnawaveUsername:  "p_provuser_base_0",
				Purpose:            multisubaggregate.PurposeBase,
				Status:             multisubaggregate.BindingActive,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
			{
				ID:                 "bind-2",
				SubscriptionID:     "sub-1",
				PlatformUserID:     provisionTestUserID,
				RemnawaveUUID:      "rw-uuid-2",
				RemnawaveShortUUID: "rw-short-2",
				RemnawaveUsername:  "p_provuser_gaming_0",
				Purpose:            multisubaggregate.PurposeGaming,
				Status:             multisubaggregate.BindingActive,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
		}

		h.bindings.On("GetByPlatformUserID", mock.Anything, provisionTestUserID).
			Return(activeBindings, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/bindings", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Len(t, resp, 2)

		// Verify the first binding has expected fields.
		assert.Equal(t, "bind-1", resp[0]["ID"])
		assert.Equal(t, "rw-short-1", resp[0]["RemnawaveShortUUID"])
		assert.Equal(t, "base", resp[0]["Purpose"])
		assert.Equal(t, "active", resp[0]["Status"])

		h.bindings.AssertExpectations(t)
	})

	t.Run("subscription bindings requires auth", func(t *testing.T) {
		h := newProvisioningTestHarness(t)

		req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/sub-1/bindings", nil)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("get bindings by subscription returns empty array", func(t *testing.T) {
		h := newProvisioningTestHarness(t)
		token := provisionAccessToken(t, h.jwt)

		h.bindings.On("GetBySubscriptionID", mock.Anything, "sub-1").
			Return([]*multisubaggregate.RemnawaveBinding{}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/sub-1/bindings", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []*multisubaggregate.RemnawaveBinding
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Empty(t, resp)

		h.bindings.AssertExpectations(t)
	})

	t.Run("get bindings by subscription owned by user", func(t *testing.T) {
		h := newProvisioningTestHarness(t)
		token := provisionAccessToken(t, h.jwt)

		now := time.Now()
		bindings := []*multisubaggregate.RemnawaveBinding{
			{
				ID:                 "bind-10",
				SubscriptionID:     "sub-10",
				PlatformUserID:     provisionTestUserID,
				RemnawaveUUID:      "rw-uuid-10",
				RemnawaveShortUUID: "rw-short-10",
				Purpose:            multisubaggregate.PurposeBase,
				Status:             multisubaggregate.BindingActive,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
		}

		h.bindings.On("GetBySubscriptionID", mock.Anything, "sub-10").Return(bindings, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/sub-10/bindings", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Len(t, resp, 1)
		assert.Equal(t, "bind-10", resp[0]["ID"])

		h.bindings.AssertExpectations(t)
	})

	t.Run("get bindings by subscription owned by other user returns 403", func(t *testing.T) {
		h := newProvisioningTestHarness(t)
		token := provisionAccessToken(t, h.jwt)

		now := time.Now()
		otherUserBindings := []*multisubaggregate.RemnawaveBinding{
			{
				ID:             "bind-99",
				SubscriptionID: "sub-99",
				PlatformUserID: "other-user-id",
				Purpose:        multisubaggregate.PurposeBase,
				Status:         multisubaggregate.BindingActive,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		}

		h.bindings.On("GetBySubscriptionID", mock.Anything, "sub-99").Return(otherUserBindings, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/sub-99/bindings", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)

		var resp map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Contains(t, resp["error"], "does not belong to you")

		h.bindings.AssertExpectations(t)
	})
}
