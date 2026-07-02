package telegram

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/identitytest"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// scopeNoopPublisher discards domain events for identity-service wiring.
type scopeNoopPublisher struct{}

func (scopeNoopPublisher) Publish(_ context.Context, _ domainevent.Event) error        { return nil }
func (scopeNoopPublisher) PublishBatch(_ context.Context, _ []domainevent.Event) error { return nil }

// newScopeTestIdentity wires a real identity.Service over the given mock repo
// (mirrors the bothost ops_test pattern).
func newScopeTestIdentity(t *testing.T, repo *identitytest.MockRepository) *identity.Service {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)
	pub := scopeNoopPublisher{}
	sessions := identity.NewSessionIssuer(repo, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
	return identity.NewService(
		repo, pub, txmanagertest.NoopTxRunner{}, jwtIssuer,
		clock.NewReal(), 15*time.Minute, 7*24*time.Hour, sessions,
	)
}

// scopeStubSubs records the ctx GetByUserID received and returns a configurable
// subscription from GetByID.
type scopeStubSubs struct {
	billing.SubscriptionReader
	lastCtx  context.Context
	getByID  *billingaggregate.Subscription
	getByIDN int
}

func (s *scopeStubSubs) GetByUserID(ctx context.Context, _ string) ([]*billingaggregate.Subscription, error) {
	s.lastCtx = ctx
	return nil, nil
}

func (s *scopeStubSubs) GetByID(_ context.Context, _ string) (*billingaggregate.Subscription, error) {
	s.getByIDN++
	return s.getByID, nil
}

// newPlatformScopeBot builds an offline platform Bot (b.bot == nil — sends are
// nil-guarded) for handler scope tests.
func newPlatformScopeBot(t *testing.T, repo *identitytest.MockRepository) *Bot {
	t.Helper()
	return &Bot{
		tenantID: tenantctx.PlatformScopeSentinel,
		txRunner: txmanagertest.NoopTxRunner{},
		identity: newScopeTestIdentity(t, repo),
		logger:   testLogger(),
	}
}

func messageUpdate(tgID int64, text string) *models.Update {
	return &models.Update{
		Message: &models.Message{
			ID:   1,
			Chat: models.Chat{ID: 500},
			From: &models.User{ID: tgID, FirstName: "Neo"},
			Text: text,
		},
	}
}

// TestHandleStart_IdentityReadCarriesPlatformScope verifies the legacy /start
// handler runs its RLS-protected identity read inside a platform-scoped ctx.
func TestHandleStart_IdentityReadCarriesPlatformScope(t *testing.T) {
	var gotCtx context.Context
	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, int64(42)).
		Run(func(args mock.Arguments) { gotCtx = args.Get(0).(context.Context) }).
		Return(nil, identity.ErrNotFound)

	b := newPlatformScopeBot(t, repo)

	b.handleStart(context.Background(), nil, messageUpdate(42, "/start"))

	require.NotNil(t, gotCtx, "identity read must have happened")
	require.Equal(t, tenantctx.PlatformScopeSentinel, tenantctx.TenantIDFromContext(gotCtx))
}

// TestHandleMy_ReadsCarryPlatformScope verifies /my runs both the identity and
// the subscription reads inside a platform-scoped ctx.
func TestHandleMy_ReadsCarryPlatformScope(t *testing.T) {
	var gotIdentityCtx context.Context
	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, int64(42)).
		Run(func(args mock.Arguments) { gotIdentityCtx = args.Get(0).(context.Context) }).
		Return(&identity.PlatformUser{ID: "u-1"}, nil)

	subs := &scopeStubSubs{}
	b := newPlatformScopeBot(t, repo)
	b.subs = subs

	b.handleMy(context.Background(), nil, messageUpdate(42, "/my"))

	require.NotNil(t, gotIdentityCtx)
	require.Equal(t, tenantctx.PlatformScopeSentinel, tenantctx.TenantIDFromContext(gotIdentityCtx))
	require.NotNil(t, subs.lastCtx, "subscription read must have happened")
	require.Equal(t, tenantctx.PlatformScopeSentinel, tenantctx.TenantIDFromContext(subs.lastCtx))
}

func callbackUpdate(tgID int64, data string) *models.Update {
	return &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "cb-1",
			From: models.User{ID: tgID},
			Data: data,
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{ID: 7, Chat: models.Chat{ID: 500}},
			},
		},
	}
}

// TestHandleCancelCallback_ForeignSubscription_Denied verifies the ownership
// check: a caller cannot cancel a subscription owned by another user. The
// concrete billing service is nil, so reaching CancelSubscription would panic —
// the test passing proves the guard denied before the cancel call.
func TestHandleCancelCallback_ForeignSubscription_Denied(t *testing.T) {
	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, int64(42)).
		Return(&identity.PlatformUser{ID: "u-caller"}, nil)

	subs := &scopeStubSubs{getByID: &billingaggregate.Subscription{ID: "s-1", UserID: "u-OTHER"}}
	b := newPlatformScopeBot(t, repo)
	b.subs = subs

	// Must not panic (would if CancelSubscription on nil b.billing were reached).
	b.handleCancelCallback(context.Background(), nil, callbackUpdate(42, CallbackPrefixCancel+"s-1"))

	require.Equal(t, 1, subs.getByIDN, "ownership read must have run")
}
