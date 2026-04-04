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

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/billingtest"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

const (
	billingTestUserID = "billing-user-1"
	billingTestEmail  = "billing@example.com"
)

// billingTestHarness bundles the router, mocks, and JWT issuer used across
// billing integration subtests.
type billingTestHarness struct {
	router   http.Handler
	plans    *billingtest.MockPlanRepo
	subs     *billingtest.MockSubscriptionRepo
	invoices *billingtest.MockInvoiceRepo
	families *billingtest.MockFamilyRepo
	pub      *billingtest.MockEventPublisher
	jwt      *authutil.JWTIssuer
}

// newBillingTestHarness creates a chi router with billing-related routes wired
// to a real BillingService backed by mock repositories.
func newBillingTestHarness(t *testing.T) *billingTestHarness {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)

	plans := new(billingtest.MockPlanRepo)
	subs := new(billingtest.MockSubscriptionRepo)
	invoices := new(billingtest.MockInvoiceRepo)
	families := new(billingtest.MockFamilyRepo)
	pub := new(billingtest.MockEventPublisher)

	prorate := billingservice.NewProrateCalculator()
	trial := billingservice.NewTrialManager(billingservice.DefaultTrialDays)
	txRunner := billingtest.NoopTxRunner{}
	svc := billingservice.NewBillingService(plans, subs, invoices, families, pub, prorate, trial, txRunner, clock.NewReal(), nil)

	bh := handler.NewBillingHandler(svc, plans, subs, invoices)

	r := chi.NewRouter()
	// Public routes.
	r.Get("/api/plans", bh.GetPlans)
	r.Get("/api/plans/{planID}", bh.GetPlan)

	// Protected routes.
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.Auth(jwtIssuer))
		protected.Post("/api/subscriptions", bh.CreateSubscription)
		protected.Get("/api/subscriptions", bh.GetMySubscriptions)
		protected.Get("/api/subscriptions/{subID}", bh.GetSubscription)
		protected.Post("/api/subscriptions/{subID}/cancel", bh.CancelSubscription)
		protected.Get("/api/invoices", bh.GetInvoices)
		protected.Post("/api/invoices/{invoiceID}/pay", bh.PayInvoice)
	})

	return &billingTestHarness{
		router:   r,
		plans:    plans,
		subs:     subs,
		invoices: invoices,
		families: families,
		pub:      pub,
		jwt:      jwtIssuer,
	}
}

// billingAccessToken creates a signed JWT for the billing test user.
func billingAccessToken(t *testing.T, jwtIssuer *authutil.JWTIssuer) string {
	t.Helper()
	token, err := jwtIssuer.Sign(authutil.UserClaims{
		UserID: billingTestUserID,
		Email:  billingTestEmail,
		Role:   string(identity.RoleCustomer),
	}, testAccessTTL)
	require.NoError(t, err)
	return token
}

// TestBillingFlow validates the billing HTTP handlers end-to-end: list plans,
// create subscription, get subscriptions, pay invoice, cancel subscription.
func TestBillingFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("list plans returns empty array", func(t *testing.T) {
		h := newBillingTestHarness(t)

		h.plans.On("GetActive", mock.Anything).Return([]*aggregate.Plan{}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/plans", nil)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []*aggregate.Plan
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Empty(t, resp)

		h.plans.AssertExpectations(t)
	})

	t.Run("list plans returns active plans", func(t *testing.T) {
		h := newBillingTestHarness(t)

		activePlans := []*aggregate.Plan{
			{
				ID:        "plan-1",
				Name:      "Basic",
				BasePrice: vo.Money{Amount: 999, Currency: "USD"},
			},
			{
				ID:        "plan-2",
				Name:      "Premium",
				BasePrice: vo.Money{Amount: 1999, Currency: "USD"},
			},
		}
		h.plans.On("GetActive", mock.Anything).Return(activePlans, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/plans", nil)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Len(t, resp, 2)

		h.plans.AssertExpectations(t)
	})

	t.Run("get single plan", func(t *testing.T) {
		h := newBillingTestHarness(t)

		plan := &aggregate.Plan{
			ID:        "plan-1",
			Name:      "Basic",
			BasePrice: vo.Money{Amount: 999, Currency: "USD"},
		}
		h.plans.On("GetByID", mock.Anything, "plan-1").Return(plan, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/plans/plan-1", nil)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "plan-1", resp["ID"])
		assert.Equal(t, "Basic", resp["Name"])

		h.plans.AssertExpectations(t)
	})

	t.Run("create subscription requires auth", func(t *testing.T) {
		h := newBillingTestHarness(t)

		body := `{"plan_id":"plan-1","addon_ids":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/subscriptions", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("create subscription with valid token", func(t *testing.T) {
		h := newBillingTestHarness(t)
		token := billingAccessToken(t, h.jwt)

		plan := &aggregate.Plan{
			ID:        "plan-1",
			Name:      "Basic",
			BasePrice: vo.Money{Amount: 999, Currency: "USD"},
			Interval:  vo.IntervalMonth,
		}
		h.plans.On("GetByID", mock.Anything, "plan-1").Return(plan, nil)
		h.subs.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).Return(nil)
		h.invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).Return(nil)
		h.pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

		body := `{"plan_id":"plan-1","addon_ids":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/subscriptions", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotNil(t, resp["subscription"])
		assert.NotNil(t, resp["invoice"])

		h.plans.AssertExpectations(t)
		h.subs.AssertExpectations(t)
		h.invoices.AssertExpectations(t)
		h.pub.AssertExpectations(t)
	})

	t.Run("get my subscriptions requires auth", func(t *testing.T) {
		h := newBillingTestHarness(t)

		req := httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("get my subscriptions returns empty array", func(t *testing.T) {
		h := newBillingTestHarness(t)
		token := billingAccessToken(t, h.jwt)

		h.subs.On("GetByUserID", mock.Anything, billingTestUserID).Return([]*aggregate.Subscription{}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/subscriptions", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []*aggregate.Subscription
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Empty(t, resp)

		h.subs.AssertExpectations(t)
	})

	t.Run("cancel subscription", func(t *testing.T) {
		h := newBillingTestHarness(t)
		token := billingAccessToken(t, h.jwt)

		now := time.Now()
		sub := &aggregate.Subscription{
			ID:        "sub-1",
			UserID:    billingTestUserID,
			PlanID:    "plan-1",
			Status:    aggregate.StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}

		// The handler calls subs.GetByID to verify ownership, then
		// the service calls subs.GetByID again to load the aggregate.
		h.subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil)
		h.subs.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).Return(nil)
		h.pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/sub-1/cancel", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", resp["status"])

		h.subs.AssertExpectations(t)
		h.pub.AssertExpectations(t)
	})

	t.Run("get invoices requires auth", func(t *testing.T) {
		h := newBillingTestHarness(t)

		req := httptest.NewRequest(http.MethodGet, "/api/invoices", nil)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("get pending invoices returns empty array", func(t *testing.T) {
		h := newBillingTestHarness(t)
		token := billingAccessToken(t, h.jwt)

		h.invoices.On("GetPendingByUserID", mock.Anything, billingTestUserID).Return([]*aggregate.Invoice{}, nil)

		req := httptest.NewRequest(http.MethodGet, "/api/invoices", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []*aggregate.Invoice
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Empty(t, resp)

		h.invoices.AssertExpectations(t)
	})
}
