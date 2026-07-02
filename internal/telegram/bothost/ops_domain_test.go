package bothost

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/identitytest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// ─── Stub readers ────────────────────────────────────────────────────────────

// stubTariffReader is a test double for TariffReader that records the context
// each method received and returns configurable data.
type stubTariffReader struct {
	lastCtx     context.Context
	lastChannel string
	offers      []TariffOffer
	listErr     error
	getOffer    *TariffOffer
	getErr      error
}

func (s *stubTariffReader) ListVisible(ctx context.Context, channel string) ([]TariffOffer, error) {
	s.lastCtx = ctx
	s.lastChannel = channel
	return s.offers, s.listErr
}

func (s *stubTariffReader) Get(ctx context.Context, _ string) (*TariffOffer, error) {
	s.lastCtx = ctx
	return s.getOffer, s.getErr
}

// stubSubscriptionReader is a test double for SubscriptionReader.
type stubSubscriptionReader struct {
	lastCtx    context.Context
	lastUserID string
	activeSubs []Subscription
	activeErr  error
	getSub     *Subscription
	getOwnerID string
	getErr     error
}

func (s *stubSubscriptionReader) ActiveByUser(ctx context.Context, userID string) ([]Subscription, error) {
	s.lastCtx = ctx
	s.lastUserID = userID
	return s.activeSubs, s.activeErr
}

func (s *stubSubscriptionReader) Get(ctx context.Context, _ string) (*Subscription, string, error) {
	s.lastCtx = ctx
	return s.getSub, s.getOwnerID, s.getErr
}

// stubInvoiceReader is a test double for InvoiceReader.
type stubInvoiceReader struct {
	lastCtx  context.Context
	invoices []Invoice
	err      error
}

func (s *stubInvoiceReader) PendingByUser(ctx context.Context, _ string) ([]Invoice, error) {
	s.lastCtx = ctx
	return s.invoices, s.err
}

// stubBalanceReader is a test double for BalanceReader.
type stubBalanceReader struct {
	lastCtx context.Context
	wallets []Wallet
	err     error
}

func (s *stubBalanceReader) WalletsByUser(ctx context.Context, _ string) ([]Wallet, error) {
	s.lastCtx = ctx
	return s.wallets, s.err
}

// stubCheckoutStarter is a test double for CheckoutStarter that records the
// context and input it was called with.
type stubCheckoutStarter struct {
	lastCtx   context.Context
	lastInput CheckoutInput
	result    CheckoutResult
	err       error
}

func (s *stubCheckoutStarter) Start(ctx context.Context, in CheckoutInput) (CheckoutResult, error) {
	s.lastCtx = ctx
	s.lastInput = in
	return s.result, s.err
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// callDomainOp registers all domain ops and invokes op with the given
// permission and JSON-serialised args. It returns the raw result and error.
func callDomainOp(t *testing.T, oc *OpContext, perm plugin.PermissionScope, op string, args any) (json.RawMessage, error) {
	t.Helper()
	r := NewRegistry()
	RegisterDomainOps(r)
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	return r.Call(context.Background(), oc, NewPermSet(perm), op, raw)
}

// newOCWithIdentity builds an OpContext wired with a real identity.Service backed
// by the given repo mock, and a NoopTxRunner.
func newOCWithIdentity(t *testing.T, tenant string, repo *identitytest.MockRepository) *OpContext {
	t.Helper()
	svc := newTestIdentityService(t, repo)
	return &OpContext{
		TenantID: tenant,
		Identity: svc,
		TxRunner: txmanagertest.NoopTxRunner{},
	}
}

// ─── plans.list ──────────────────────────────────────────────────────────────

func TestDomainOp_PlansList_ReturnsOffers(t *testing.T) {
	const tenant = "shop-abc"
	stub := &stubTariffReader{
		offers: []TariffOffer{
			{PlanID: "plan-1", Name: "Basic"},
		},
	}
	oc := &OpContext{
		TenantID: tenant,
		TxRunner: txmanagertest.NoopTxRunner{},
		Tariffs:  stub,
	}

	out, err := callDomainOp(t, oc, plugin.PermBillingRead, OpPlansList, plansListArgs{})
	require.NoError(t, err)

	var got []TariffOffer
	require.NoError(t, json.Unmarshal(out, &got))
	require.Len(t, got, 1)
	require.Equal(t, "plan-1", got[0].PlanID)
}

func TestDomainOp_PlansList_DefaultChannel(t *testing.T) {
	const tenant = "shop-abc"
	stub := &stubTariffReader{offers: []TariffOffer{}}
	oc := &OpContext{
		TenantID: tenant,
		TxRunner: txmanagertest.NoopTxRunner{},
		Tariffs:  stub,
	}

	_, err := callDomainOp(t, oc, plugin.PermBillingRead, OpPlansList, plansListArgs{})
	require.NoError(t, err)
	require.Equal(t, ChannelTelegram, stub.lastChannel)
}

func TestDomainOp_PlansList_TenantCtx(t *testing.T) {
	const tenant = "shop-abc"
	stub := &stubTariffReader{offers: []TariffOffer{}}
	oc := &OpContext{
		TenantID: tenant,
		TxRunner: txmanagertest.NoopTxRunner{},
		Tariffs:  stub,
	}

	_, err := callDomainOp(t, oc, plugin.PermBillingRead, OpPlansList, plansListArgs{})
	require.NoError(t, err)
	require.Equal(t, tenant, tenantctx.TenantIDFromContext(stub.lastCtx))
}

// ─── plans.get ───────────────────────────────────────────────────────────────

func TestDomainOp_PlansGet_ReturnsOffer(t *testing.T) {
	const tenant = "shop-xyz"
	offer := &TariffOffer{PlanID: "plan-2", Name: "Pro"}
	stub := &stubTariffReader{getOffer: offer}
	oc := &OpContext{
		TenantID: tenant,
		TxRunner: txmanagertest.NoopTxRunner{},
		Tariffs:  stub,
	}

	out, err := callDomainOp(t, oc, plugin.PermBillingRead, OpPlansGet, plansGetArgs{PlanID: "plan-2"})
	require.NoError(t, err)

	var got TariffOffer
	require.NoError(t, json.Unmarshal(out, &got))
	require.Equal(t, "plan-2", got.PlanID)

	require.Equal(t, tenant, tenantctx.TenantIDFromContext(stub.lastCtx))
}

// ─── subscriptions.mine ──────────────────────────────────────────────────────

func TestDomainOp_SubscriptionsMine_ReturnsSubs(t *testing.T) {
	const (
		tenant = "shop-subs"
		tgID   = int64(111)
		userID = "u-mine"
	)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, tgID).
		Return(&identity.PlatformUser{ID: userID}, nil)

	subStub := &stubSubscriptionReader{
		activeSubs: []Subscription{{ID: "sub-1", PlanID: "plan-1", Status: "active"}},
	}
	oc := newOCWithIdentity(t, tenant, repo)
	oc.Subs = subStub

	out, err := callDomainOp(t, oc, plugin.PermBillingRead, OpSubscriptionsMine, subscriptionsMineArgs{TelegramID: tgID})
	require.NoError(t, err)

	var got []Subscription
	require.NoError(t, json.Unmarshal(out, &got))
	require.Len(t, got, 1)
	require.Equal(t, "sub-1", got[0].ID)

	// resolveUser passed userID to ActiveByUser
	require.Equal(t, userID, subStub.lastUserID)
	// reader received tenant-scoped ctx
	require.Equal(t, tenant, tenantctx.TenantIDFromContext(subStub.lastCtx))

	repo.AssertExpectations(t)
}

// ─── subscriptions.get ───────────────────────────────────────────────────────

func TestDomainOp_SubscriptionsGet_OwnerMatch(t *testing.T) {
	const (
		tenant = "shop-subs"
		tgID   = int64(222)
		userID = "u-owner"
		subID  = "sub-99"
	)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, tgID).
		Return(&identity.PlatformUser{ID: userID}, nil)

	subStub := &stubSubscriptionReader{
		getSub:     &Subscription{ID: subID, PlanID: "plan-1", Status: "active"},
		getOwnerID: userID, // owner matches resolved user
	}
	oc := newOCWithIdentity(t, tenant, repo)
	oc.Subs = subStub

	out, err := callDomainOp(t, oc, plugin.PermBillingRead, OpSubscriptionsGet, subscriptionsGetArgs{
		TelegramID: tgID,
		ID:         subID,
	})
	require.NoError(t, err)

	var got Subscription
	require.NoError(t, json.Unmarshal(out, &got))
	require.Equal(t, subID, got.ID)

	require.Equal(t, tenant, tenantctx.TenantIDFromContext(subStub.lastCtx))
	repo.AssertExpectations(t)
}

func TestDomainOp_SubscriptionsGet_OwnerMismatch(t *testing.T) {
	const (
		tenant = "shop-subs"
		tgID   = int64(222)
		userID = "u-requester"
		subID  = "sub-77"
	)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, tgID).
		Return(&identity.PlatformUser{ID: userID}, nil)

	subStub := &stubSubscriptionReader{
		getSub:     &Subscription{ID: subID, PlanID: "plan-1", Status: "active"},
		getOwnerID: "u-other", // different owner
	}
	oc := newOCWithIdentity(t, tenant, repo)
	oc.Subs = subStub

	out, err := callDomainOp(t, oc, plugin.PermBillingRead, OpSubscriptionsGet, subscriptionsGetArgs{
		TelegramID: tgID,
		ID:         subID,
	})
	require.ErrorIs(t, err, ErrForbidden)
	require.Nil(t, out) // no record leaks on forbidden

	repo.AssertExpectations(t)
}

// ─── invoices.pending ────────────────────────────────────────────────────────

func TestDomainOp_InvoicesPending_ReturnsInvoices(t *testing.T) {
	const (
		tenant = "shop-inv"
		tgID   = int64(333)
		userID = "u-inv"
	)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, tgID).
		Return(&identity.PlatformUser{ID: userID}, nil)

	invStub := &stubInvoiceReader{
		invoices: []Invoice{{ID: "inv-1", Status: "pending", Amount: 1000, Currency: "USD"}},
	}
	oc := newOCWithIdentity(t, tenant, repo)
	oc.Invoices = invStub

	out, err := callDomainOp(t, oc, plugin.PermBillingRead, OpInvoicesPending, invoicesPendingArgs{TelegramID: tgID})
	require.NoError(t, err)

	var got []Invoice
	require.NoError(t, json.Unmarshal(out, &got))
	require.Len(t, got, 1)
	require.Equal(t, "inv-1", got[0].ID)

	require.Equal(t, tenant, tenantctx.TenantIDFromContext(invStub.lastCtx))
	repo.AssertExpectations(t)
}

// ─── balance.get ─────────────────────────────────────────────────────────────

func TestDomainOp_BalanceGet_ReturnsWallets(t *testing.T) {
	const (
		tenant = "shop-bal"
		tgID   = int64(444)
		userID = "u-bal"
	)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, tgID).
		Return(&identity.PlatformUser{ID: userID}, nil)

	balStub := &stubBalanceReader{
		wallets: []Wallet{{Kind: "main", Currency: "USD", Balance: 500, Available: 400}},
	}
	oc := newOCWithIdentity(t, tenant, repo)
	oc.Balance = balStub

	out, err := callDomainOp(t, oc, plugin.PermBillingRead, OpBalanceGet, balanceGetArgs{TelegramID: tgID})
	require.NoError(t, err)

	var got []Wallet
	require.NoError(t, json.Unmarshal(out, &got))
	require.Len(t, got, 1)
	require.Equal(t, int64(500), got[0].Balance)

	require.Equal(t, tenant, tenantctx.TenantIDFromContext(balStub.lastCtx))
	repo.AssertExpectations(t)
}

// ─── Nil-capability guards ───────────────────────────────────────────────────

// TestDomainOps_NilCapability verifies every read op fails closed with
// ErrCapabilityUnavailable when its reader is not wired into the OpContext.
// checkout.create's nil guard sits behind resolveUser and is covered
// separately by TestDomainOp_CheckoutCreate_NilCheckout.
func TestDomainOps_NilCapability(t *testing.T) {
	cases := []struct {
		op   string
		args any
	}{
		{OpPlansList, plansListArgs{}},
		{OpPlansGet, plansGetArgs{PlanID: "p1"}},
		{OpSubscriptionsMine, subscriptionsMineArgs{TelegramID: 1}},
		{OpSubscriptionsGet, subscriptionsGetArgs{TelegramID: 1, ID: "s1"}},
		{OpInvoicesPending, invoicesPendingArgs{TelegramID: 1}},
		{OpBalanceGet, balanceGetArgs{TelegramID: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.op, func(t *testing.T) {
			oc := &OpContext{TenantID: "shop-nil", TxRunner: txmanagertest.NoopTxRunner{}}

			_, err := callDomainOp(t, oc, plugin.PermBillingRead, tc.op, tc.args)
			require.ErrorIs(t, err, ErrCapabilityUnavailable)
		})
	}
}

// ─── checkout.create ─────────────────────────────────────────────────────────

func TestDomainOp_CheckoutCreate_ReturnsResult(t *testing.T) {
	const (
		tenant  = "shop-co"
		tgID    = int64(555)
		userID  = "u-co"
		planID  = "plan-3"
		email   = "user@example.com"
		country = "US"
	)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, tgID).
		Return(&identity.PlatformUser{ID: userID}, nil)

	coStub := &stubCheckoutStarter{
		result: CheckoutResult{
			CheckoutURL:    "https://pay.example.com/session-abc",
			SubscriptionID: "sub-new",
			InvoiceID:      "inv-new",
			Provider:       "stripe",
		},
	}
	oc := newOCWithIdentity(t, tenant, repo)
	oc.Checkout = coStub

	out, err := callDomainOp(t, oc, plugin.PermPaymentWrite, OpCheckoutCreate, checkoutCreateArgs{
		TelegramID: tgID,
		PlanID:     planID,
		Email:      email,
		Country:    country,
	})
	require.NoError(t, err)

	var got CheckoutResult
	require.NoError(t, json.Unmarshal(out, &got))
	require.Equal(t, "https://pay.example.com/session-abc", got.CheckoutURL)
	require.Equal(t, "sub-new", got.SubscriptionID)

	// Input fields must be threaded through correctly.
	require.Equal(t, userID, coStub.lastInput.UserID)
	require.Equal(t, planID, coStub.lastInput.PlanID)
	require.Equal(t, email, coStub.lastInput.UserEmail)
	require.Equal(t, country, coStub.lastInput.UserCountry)

	// Checkout.Start must receive the shop's tenant in ctx (so its self-wrapped
	// RunInTx stamps the subscription/invoice with tenant_id) — but NOT be
	// wrapped in RunInTx here (CheckoutService self-wraps; double-wrap would
	// break re-entrancy). WithTenantID only annotates the ctx; it does not open
	// a transaction.
	require.Equal(t, tenant, tenantctx.TenantIDFromContext(coStub.lastCtx),
		"checkout.create must scope Start's ctx to the shop tenant")

	repo.AssertExpectations(t)
}

func TestDomainOp_CheckoutCreate_ResolveUserPropagatesNotFound(t *testing.T) {
	const (
		tenant = "shop-co"
		tgID   = int64(666)
	)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, tgID).
		Return(nil, identity.ErrNotFound)

	coStub := &stubCheckoutStarter{}
	oc := newOCWithIdentity(t, tenant, repo)
	oc.Checkout = coStub

	_, err := callDomainOp(t, oc, plugin.PermPaymentWrite, OpCheckoutCreate, checkoutCreateArgs{
		TelegramID: tgID,
		PlanID:     "plan-x",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, identity.ErrNotFound)

	repo.AssertExpectations(t)
}

func TestDomainOp_CheckoutCreate_NilCheckout(t *testing.T) {
	const (
		tenant = "shop-co"
		tgID   = int64(777)
		userID = "u-co2"
	)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, tgID).
		Return(&identity.PlatformUser{ID: userID}, nil)

	oc := newOCWithIdentity(t, tenant, repo)
	// Checkout is nil

	_, err := callDomainOp(t, oc, plugin.PermPaymentWrite, OpCheckoutCreate, checkoutCreateArgs{
		TelegramID: tgID,
		PlanID:     "plan-x",
	})
	require.ErrorIs(t, err, ErrCapabilityUnavailable)
}

// ─── Permission gating ───────────────────────────────────────────────────────

func TestDomainOp_PermissionGating_BillingReadRequired(t *testing.T) {
	oc := &OpContext{
		TenantID: "shop-perm",
		TxRunner: txmanagertest.NoopTxRunner{},
		Tariffs:  &stubTariffReader{offers: []TariffOffer{}},
	}

	r := NewRegistry()
	RegisterDomainOps(r)
	raw, err := json.Marshal(plansListArgs{})
	require.NoError(t, err)

	// No billing:read permission → denied
	_, err = r.Call(context.Background(), oc, NewPermSet(), OpPlansList, raw)
	require.ErrorIs(t, err, ErrPermissionDenied)

	// With billing:read permission → ok
	_, err = r.Call(context.Background(), oc, NewPermSet(plugin.PermBillingRead), OpPlansList, raw)
	require.NoError(t, err)
}

func TestDomainOp_PermissionGating_PaymentWriteRequired(t *testing.T) {
	const (
		tenant = "shop-perm"
		tgID   = int64(888)
		userID = "u-perm"
	)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, tgID).
		Return(&identity.PlatformUser{ID: userID}, nil).Maybe()

	coStub := &stubCheckoutStarter{result: CheckoutResult{CheckoutURL: "https://pay.test/"}}
	oc := newOCWithIdentity(t, tenant, repo)
	oc.Checkout = coStub

	r := NewRegistry()
	RegisterDomainOps(r)
	raw, err := json.Marshal(checkoutCreateArgs{TelegramID: tgID, PlanID: "p1"})
	require.NoError(t, err)

	// billing:read alone is not enough for checkout
	_, err = r.Call(context.Background(), oc, NewPermSet(plugin.PermBillingRead), OpCheckoutCreate, raw)
	require.ErrorIs(t, err, ErrPermissionDenied)

	// payment:write is required
	_, err = r.Call(context.Background(), oc, NewPermSet(plugin.PermPaymentWrite), OpCheckoutCreate, raw)
	require.NoError(t, err)
}
