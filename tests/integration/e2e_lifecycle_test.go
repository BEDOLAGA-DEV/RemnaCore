//go:build integration

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
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/identitytest"
	multisubaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/multisubtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/handler"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
	"log/slog"
)

const (
	lifecycleTestEmail    = "lifecycle@example.com"
	lifecycleTestPassword = "Lifecycle!2026"

	lifecycleTestPlanID   = "plan-lifecycle-1"
	lifecycleTestPlanName = "Lifecycle Basic"

	lifecycleTestPlanPriceCents int64 = 999

	lifecycleAccessTTL  = 15 * time.Minute
	lifecycleRefreshTTL = 7 * 24 * time.Hour
)

// lifecycleHarness bundles all services, mocks, and the unified chi router
// needed for the full E2E lifecycle test.
type lifecycleHarness struct {
	router http.Handler

	// Identity mocks.
	identityRepo *identitytest.MockRepository
	identityPub  *identitytest.MockPublisher

	// Billing mocks.
	plans    *billingtest.MockPlanRepo
	subs     *billingtest.MockSubscriptionRepo
	invoices *billingtest.MockInvoiceRepo
	families *billingtest.MockFamilyRepo
	billPub  *billingtest.MockEventPublisher

	// Multisub mocks.
	bindings *multisubtest.MockBindingRepo

	jwt *authutil.JWTIssuer
}

// newLifecycleHarness wires a single chi router with identity, billing, and
// multisub routes backed by mock repositories and real domain services.
func newLifecycleHarness(t *testing.T) *lifecycleHarness {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)

	// Identity wiring.
	identityRepo := new(identitytest.MockRepository)
	identityPub := new(identitytest.MockPublisher)
	identitySvc := identity.NewService(
		identityRepo, identityPub, txmanagertest.NoopTxRunner{},
		jwtIssuer, clock.NewReal(), lifecycleAccessTTL, lifecycleRefreshTTL,
	)
	identityHandler := handler.NewIdentityHandler(identitySvc)

	// Billing wiring.
	plans := new(billingtest.MockPlanRepo)
	subs := new(billingtest.MockSubscriptionRepo)
	invoices := new(billingtest.MockInvoiceRepo)
	families := new(billingtest.MockFamilyRepo)
	billPub := new(billingtest.MockEventPublisher)
	prorate := billingservice.NewProrateCalculator()
	trial := billingservice.NewTrialManager(billingservice.DefaultTrialDays)
	billingSvc := billingservice.NewBillingService(billingservice.BillingDeps{
		Plans:     plans,
		Subs:      subs,
		Invoices:  invoices,
		Families:  families,
		Publisher: billPub,
		Prorate:   prorate,
		Trial:     trial,
		TxRunner:  billingtest.NoopTxRunner{},
		Clock:     clock.NewReal(),
		Logger:    slog.Default(),
	})
	addonSvc := billingservice.NewAddonService(subs, plans, billPub, billingtest.NoopTxRunner{}, clock.NewReal(), slog.Default())
	billingHandler := handler.NewBillingHandler(billingSvc, addonSvc, plans, subs, invoices)

	// Multisub wiring.
	bindings := new(multisubtest.MockBindingRepo)
	multisubHandler := handler.NewMultiSubHandler(bindings)

	// Unified router with all routes.
	r := chi.NewRouter()

	// Public identity routes.
	r.Post("/api/auth/register", identityHandler.Register)
	r.Post("/api/auth/login", identityHandler.Login)
	r.Post("/api/auth/verify-email", identityHandler.VerifyEmail)

	// Public billing routes.
	r.Get("/api/plans", billingHandler.GetPlans)
	r.Get("/api/plans/{planID}", billingHandler.GetPlan)

	// Protected routes.
	r.Group(func(protected chi.Router) {
		protected.Use(middleware.Auth(jwtIssuer))
		// Identity.
		protected.Get("/api/me", identityHandler.Me)
		// Billing.
		protected.Post("/api/subscriptions", billingHandler.CreateSubscription)
		protected.Get("/api/subscriptions", billingHandler.GetMySubscriptions)
		protected.Get("/api/subscriptions/{subID}", billingHandler.GetSubscription)
		protected.Post("/api/subscriptions/{subID}/cancel", billingHandler.CancelSubscription)
		protected.Get("/api/invoices", billingHandler.GetInvoices)
		protected.Post("/api/invoices/{invoiceID}/pay", billingHandler.PayInvoice)
		// Multisub.
		protected.Get("/api/bindings", multisubHandler.GetMyBindings)
		protected.Get("/api/subscriptions/{subID}/bindings", multisubHandler.GetBindingsBySubscription)
	})

	return &lifecycleHarness{
		router:       r,
		identityRepo: identityRepo,
		identityPub:  identityPub,
		plans:        plans,
		subs:         subs,
		invoices:     invoices,
		families:     families,
		billPub:      billPub,
		bindings:     bindings,
		jwt:          jwtIssuer,
	}
}

// lifecycleAccessToken creates a signed JWT for the given user.
func lifecycleAccessToken(t *testing.T, jwtIssuer *authutil.JWTIssuer, userID string) string {
	t.Helper()
	token, err := jwtIssuer.Sign(authutil.UserClaims{
		UserID: userID,
		Email:  lifecycleTestEmail,
	}, lifecycleAccessTTL)
	require.NoError(t, err)
	return token
}

// TestFullLifecycle validates the complete business cycle through HTTP handlers
// backed by real domain services and mock repositories:
//
//	Register -> Verify Email -> Login -> Create Subscription -> Pay Invoice
//	-> Verify Activation -> Check Bindings -> Cancel Subscription
//
// Each phase is a subtest that shares state with subsequent phases through
// captured response data. Mocks are configured per-phase to simulate the
// repository state at that point in the lifecycle.
func TestFullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E lifecycle test in short mode")
	}

	h := newLifecycleHarness(t)

	// Mutable state captured across subtests.
	var (
		userID            string
		verificationToken string
		accessToken       string
		subscriptionID    string
		invoiceID         string
	)

	// Phase 1: Registration
	t.Run("phase1_register", func(t *testing.T) {
		h.identityRepo.On("GetUserByEmail", mock.Anything, lifecycleTestEmail).
			Return(nil, identity.ErrNotFound).Once()
		h.identityRepo.On("CreateUser", mock.Anything, mock.AnythingOfType("*aggregate.PlatformUser")).
			Return(nil).Once()
		h.identityRepo.On("CreateEmailVerification", mock.Anything, mock.AnythingOfType("*aggregate.EmailVerification")).
			Return(nil).Once()
		h.identityPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).
			Return(nil).Once()

		body := `{"email":"` + lifecycleTestEmail + `","password":"` + lifecycleTestPassword + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code, "register should return 201")

		var resp map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		require.NotEmpty(t, resp["user_id"], "response must contain user_id")
		require.NotEmpty(t, resp["verification_token"], "response must contain verification_token")
		assert.Equal(t, lifecycleTestEmail, resp["email"])

		userID = resp["user_id"].(string)
		verificationToken = resp["verification_token"].(string)

		h.identityRepo.AssertExpectations(t)
		h.identityPub.AssertExpectations(t)
	})

	// Phase 2: Email Verification
	t.Run("phase2_verify_email", func(t *testing.T) {
		require.NotEmpty(t, userID, "phase1 must populate userID")
		require.NotEmpty(t, verificationToken, "phase1 must populate verificationToken")

		verification := &identity.EmailVerification{
			ID:        "ev-lifecycle-1",
			UserID:    userID,
			Email:     lifecycleTestEmail,
			Token:     verificationToken,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		unverifiedUser := &identity.PlatformUser{
			ID:            userID,
			Email:         lifecycleTestEmail,
			EmailVerified: false,
			Role:          identity.RoleCustomer,
		}

		h.identityRepo.On("GetEmailVerification", mock.Anything, verificationToken).
			Return(verification, nil).Once()
		h.identityRepo.On("GetUserByIDForUpdate", mock.Anything, userID).
			Return(unverifiedUser, nil).Once()
		h.identityRepo.On("UpdateUser", mock.Anything, mock.AnythingOfType("*aggregate.PlatformUser")).
			Return(nil).Once()
		h.identityRepo.On("DeleteEmailVerification", mock.Anything, "ev-lifecycle-1").
			Return(nil).Once()
		h.identityPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).
			Return(nil).Once()

		body := `{"token":"` + verificationToken + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/verify-email", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "verify email should return 200")

		var resp map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "verified", resp["status"])

		h.identityRepo.AssertExpectations(t)
		h.identityPub.AssertExpectations(t)
	})

	// Phase 3: Login
	t.Run("phase3_login", func(t *testing.T) {
		require.NotEmpty(t, userID, "phase1 must populate userID")

		passwordHash, err := authutil.HashPassword(lifecycleTestPassword)
		require.NoError(t, err)

		verifiedUser := &identity.PlatformUser{
			ID:            userID,
			Email:         lifecycleTestEmail,
			PasswordHash:  passwordHash,
			EmailVerified: true,
			Role:          identity.RoleCustomer,
		}

		h.identityRepo.On("GetUserByEmail", mock.Anything, lifecycleTestEmail).
			Return(verifiedUser, nil).Once()
		h.identityRepo.On("CreateSession", mock.Anything, mock.AnythingOfType("*aggregate.Session")).
			Return(nil).Once()
		h.identityPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).
			Return(nil).Once()

		body := `{"email":"` + lifecycleTestEmail + `","password":"` + lifecycleTestPassword + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "login should return 200")

		var resp map[string]any
		err = json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		require.NotEmpty(t, resp["access_token"], "response must contain access_token")
		require.NotEmpty(t, resp["refresh_token"], "response must contain refresh_token")
		assert.NotNil(t, resp["user"])

		// The access token from login was signed by the identity service's JWT issuer,
		// which is the same one the middleware uses. Use it directly for protected routes.
		accessToken = resp["access_token"].(string)

		h.identityRepo.AssertExpectations(t)
		h.identityPub.AssertExpectations(t)
	})

	// Phase 4: Create Subscription
	t.Run("phase4_create_subscription", func(t *testing.T) {
		require.NotEmpty(t, accessToken, "phase3 must populate accessToken")

		plan := &aggregate.Plan{
			ID:        lifecycleTestPlanID,
			Name:      lifecycleTestPlanName,
			BasePrice: vo.Money{Amount: lifecycleTestPlanPriceCents, Currency: vo.CurrencyUSD},
			Interval:  vo.IntervalMonth,
			IsActive:  true,
		}

		h.plans.On("GetByID", mock.Anything, lifecycleTestPlanID).
			Return(plan, nil).Once()
		h.subs.On("GetActiveByUserID", mock.Anything, mock.Anything).
			Return([]*aggregate.Subscription{}, nil).Once()
		h.subs.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).
			Return(nil).Once()
		h.invoices.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).
			Return(nil).Once()
		// BillingService publishes subscription.created event from the aggregate.
		h.billPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).
			Return(nil)

		body := `{"plan_id":"` + lifecycleTestPlanID + `","addon_ids":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/subscriptions", bytes.NewBufferString(body))
		req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+accessToken)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code, "create subscription should return 201")

		var resp map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		require.NotNil(t, resp["subscription"], "response must contain subscription")
		require.NotNil(t, resp["invoice"], "response must contain invoice")

		subMap := resp["subscription"].(map[string]any)
		invMap := resp["invoice"].(map[string]any)

		subscriptionID = subMap["ID"].(string)
		invoiceID = invMap["ID"].(string)

		assert.NotEmpty(t, subscriptionID)
		assert.NotEmpty(t, invoiceID)
		assert.Equal(t, string(aggregate.StatusTrial), subMap["Status"])
		assert.Equal(t, string(aggregate.InvoiceDraft), invMap["Status"])

		h.plans.AssertExpectations(t)
		h.subs.AssertExpectations(t)
		h.invoices.AssertExpectations(t)
	})

	// Phase 5: Pay Invoice (activates subscription)
	t.Run("phase5_pay_invoice", func(t *testing.T) {
		require.NotEmpty(t, accessToken, "phase3 must populate accessToken")
		require.NotEmpty(t, invoiceID, "phase4 must populate invoiceID")
		require.NotEmpty(t, subscriptionID, "phase4 must populate subscriptionID")

		now := time.Now()

		// Build the invoice that the handler will load for ownership check and
		// the service will load to transition.
		draftInvoice := &aggregate.Invoice{
			ID:             invoiceID,
			SubscriptionID: subscriptionID,
			UserID:         userID,
			LineItems:      []vo.LineItem{vo.NewLineItem(lifecycleTestPlanName, vo.LineItemPlan, vo.Money{Amount: lifecycleTestPlanPriceCents, Currency: vo.CurrencyUSD}, 1)},
			Subtotal:       vo.Money{Amount: lifecycleTestPlanPriceCents, Currency: vo.CurrencyUSD},
			TotalDiscount:  vo.Money{Amount: 0, Currency: vo.CurrencyUSD},
			Total:          vo.Money{Amount: lifecycleTestPlanPriceCents, Currency: vo.CurrencyUSD},
			Status:         aggregate.InvoiceDraft,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		// The subscription to be activated after payment.
		trialSub := &aggregate.Subscription{
			ID:        subscriptionID,
			UserID:    userID,
			PlanID:    lifecycleTestPlanID,
			Status:    aggregate.StatusTrial,
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Handler calls GetByID for ownership check.
		h.invoices.On("GetByID", mock.Anything, invoiceID).
			Return(draftInvoice, nil).Once()
		// Service calls GetByIDForUpdate inside RunInTx for processing.
		h.invoices.On("GetByIDForUpdate", mock.Anything, invoiceID).
			Return(draftInvoice, nil).Once()
		h.invoices.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.Invoice")).
			Return(nil).Once()
		// Service loads subscription for activation inside RunInTx.
		h.subs.On("GetByIDForUpdate", mock.Anything, subscriptionID).
			Return(trialSub, nil).Once()
		h.subs.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).
			Return(nil).Once()
		// Multiple events: invoice.paid, subscription.activated.
		h.billPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).
			Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/invoices/"+invoiceID+"/pay", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+accessToken)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "pay invoice should return 200")

		var resp map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "paid", resp["status"])

		// Verify the subscription was transitioned to active in-memory.
		assert.Equal(t, aggregate.StatusActive, trialSub.Status,
			"subscription should be activated after invoice payment")

		h.invoices.AssertExpectations(t)
		h.subs.AssertExpectations(t)
	})

	// Phase 6: Verify subscription is active (read through handler)
	t.Run("phase6_verify_subscription_active", func(t *testing.T) {
		require.NotEmpty(t, accessToken, "phase3 must populate accessToken")
		require.NotEmpty(t, subscriptionID, "phase4 must populate subscriptionID")

		now := time.Now()
		activeSub := &aggregate.Subscription{
			ID:        subscriptionID,
			UserID:    userID,
			PlanID:    lifecycleTestPlanID,
			Status:    aggregate.StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}

		h.subs.On("GetByID", mock.Anything, subscriptionID).
			Return(activeSub, nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/api/subscriptions/"+subscriptionID, nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+accessToken)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "get subscription should return 200")

		var resp map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, string(aggregate.StatusActive), resp["Status"],
			"subscription should report active status")

		h.subs.AssertExpectations(t)
	})

	// Phase 7: Check Remnawave bindings (simulated post-provisioning read)
	t.Run("phase7_check_bindings", func(t *testing.T) {
		require.NotEmpty(t, accessToken, "phase3 must populate accessToken")

		now := time.Now()
		provisionedBindings := []*multisubaggregate.RemnawaveBinding{
			{
				ID:                 "bind-lc-1",
				SubscriptionID:     subscriptionID,
				PlatformUserID:     userID,
				RemnawaveUUID:      "rw-uuid-lc-1",
				RemnawaveShortUUID: "rw-short-lc-1",
				RemnawaveUsername:  "p_" + userID[:8] + "_base_0",
				Purpose:            multisubaggregate.PurposeBase,
				Status:             multisubaggregate.BindingActive,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
		}

		h.bindings.On("GetByPlatformUserID", mock.Anything, userID).
			Return(provisionedBindings, nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/api/bindings", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+accessToken)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "get bindings should return 200")

		var resp []map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		require.Len(t, resp, 1, "should have one provisioned binding")
		assert.Equal(t, "bind-lc-1", resp[0]["ID"])
		assert.Equal(t, "active", resp[0]["Status"])
		assert.Equal(t, "base", resp[0]["Purpose"])

		h.bindings.AssertExpectations(t)
	})

	// Phase 8: Cancel Subscription
	t.Run("phase8_cancel_subscription", func(t *testing.T) {
		require.NotEmpty(t, accessToken, "phase3 must populate accessToken")
		require.NotEmpty(t, subscriptionID, "phase4 must populate subscriptionID")

		now := time.Now()
		activeSub := &aggregate.Subscription{
			ID:        subscriptionID,
			UserID:    userID,
			PlanID:    lifecycleTestPlanID,
			Status:    aggregate.StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Handler calls GetByID for ownership check.
		h.subs.On("GetByID", mock.Anything, subscriptionID).
			Return(activeSub, nil).Once()
		// Service calls GetByIDForUpdate inside RunInTx for cancellation.
		h.subs.On("GetByIDForUpdate", mock.Anything, subscriptionID).
			Return(activeSub, nil).Once()
		h.subs.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.Subscription")).
			Return(nil).Once()
		// subscription.cancelled event.
		h.billPub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).
			Return(nil)

		req := httptest.NewRequest(http.MethodPost, "/api/subscriptions/"+subscriptionID+"/cancel", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+accessToken)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "cancel subscription should return 200")

		var resp map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", resp["status"])

		// Verify the aggregate transitioned to cancelled in-memory.
		assert.Equal(t, aggregate.StatusCancelled, activeSub.Status,
			"subscription should be cancelled after cancel request")
		assert.NotNil(t, activeSub.CancelledAt,
			"CancelledAt should be set after cancellation")

		h.subs.AssertExpectations(t)
	})

	// Phase 9: Verify deprovisioned bindings (simulated post-deprovisioning read)
	t.Run("phase9_verify_deprovisioned_bindings", func(t *testing.T) {
		require.NotEmpty(t, accessToken, "phase3 must populate accessToken")

		now := time.Now()
		deprovisionedBindings := []*multisubaggregate.RemnawaveBinding{
			{
				ID:                 "bind-lc-1",
				SubscriptionID:     subscriptionID,
				PlatformUserID:     userID,
				RemnawaveUUID:      "rw-uuid-lc-1",
				RemnawaveShortUUID: "rw-short-lc-1",
				RemnawaveUsername:  "p_" + userID[:8] + "_base_0",
				Purpose:            multisubaggregate.PurposeBase,
				Status:             multisubaggregate.BindingDeprovisioned,
				CreatedAt:          now,
				UpdatedAt:          now,
			},
		}

		h.bindings.On("GetByPlatformUserID", mock.Anything, userID).
			Return(deprovisionedBindings, nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/api/bindings", nil)
		req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+accessToken)
		rec := httptest.NewRecorder()

		h.router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "get bindings should return 200")

		var resp []map[string]any
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		require.Len(t, resp, 1, "should still have one binding")
		assert.Equal(t, "deprovisioned", resp[0]["Status"],
			"binding should be deprovisioned after subscription cancellation")

		h.bindings.AssertExpectations(t)
	})
}
