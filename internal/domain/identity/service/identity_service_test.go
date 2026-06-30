package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// telegramStubRepo wraps adminStubRepo and adds telegram-specific state
// tracking so tests can assert on create call counts and the stored users.
type telegramStubRepo struct {
	adminStubRepo

	byTelegramID map[int64]*aggregate.PlatformUser
	createCalls  int
}

func newTelegramStubRepo() *telegramStubRepo {
	return &telegramStubRepo{
		adminStubRepo: adminStubRepo{
			invitations:  map[string]*aggregate.Invitation{},
			usersByEmail: map[string]*aggregate.PlatformUser{},
			usersByID:    map[string]*aggregate.PlatformUser{},
		},
		byTelegramID: map[int64]*aggregate.PlatformUser{},
	}
}

// GetUserByTelegramID overrides the embedded stub to return stored telegram users.
func (r *telegramStubRepo) GetUserByTelegramID(_ context.Context, id int64) (*aggregate.PlatformUser, error) {
	if u, ok := r.byTelegramID[id]; ok {
		return u, nil
	}
	return nil, service.ErrNotFound
}

// CreateUser overrides the embedded stub to track calls and index by telegram ID.
func (r *telegramStubRepo) CreateUser(_ context.Context, u *aggregate.PlatformUser) error {
	r.createCalls++
	r.adminStubRepo.createdUsers = append(r.adminStubRepo.createdUsers, u)
	if u.TelegramID != nil {
		r.byTelegramID[*u.TelegramID] = u
	}
	return nil
}

// newTestServiceWithStubs builds a Service with a telegramStubRepo and
// NoopTxRunner for unit testing RegisterViaTelegram.
func newTestServiceWithStubs(t *testing.T) (*service.Service, *telegramStubRepo) {
	t.Helper()
	repo := newTelegramStubRepo()
	pub := &noopPublisher{}
	clk := clock.NewMock(time.Unix(1_700_000_000, 0))
	jwtIssuer := newTestJWT(t)
	sessions := service.NewSessionIssuer(repo, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
	svc := service.NewService(repo, pub, txmanagertest.NoopTxRunner{}, jwtIssuer, clk, 15*time.Minute, 7*24*time.Hour, sessions)
	return svc, repo
}

// TestRegisterViaTelegram_Idempotent verifies that the first call creates the
// user (createCalls==1) and the second call with the same (telegramID, tenantID)
// returns the identical user without a second database write (createCalls still 1).
func TestRegisterViaTelegram_Idempotent(t *testing.T) {
	svc, repo := newTestServiceWithStubs(t)
	u1, err := svc.RegisterViaTelegram(context.Background(), 999, "t-1", "Bob")
	require.NoError(t, err)
	require.Equal(t, int64(999), *u1.TelegramID)
	require.Equal(t, "t-1", *u1.TenantID)
	require.Equal(t, 1, repo.createCalls)
	u2, err := svc.RegisterViaTelegram(context.Background(), 999, "t-1", "Bob")
	require.NoError(t, err)
	require.Equal(t, u1.ID, u2.ID)
	require.Equal(t, 1, repo.createCalls) // no second create
}
