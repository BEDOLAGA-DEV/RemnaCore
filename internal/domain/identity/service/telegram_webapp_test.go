package service_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// buildTgInitData constructs a valid Telegram WebApp initData string signed
// with the given botToken and auth_date. Mirrors the helper in
// pkg/telegramauth/initdata_test.go so no cross-package test dependency is needed.
func buildTgInitData(t *testing.T, botToken string, authDate int64, userJSON string) string {
	t.Helper()
	v := url.Values{}
	v.Set("auth_date", strconv.FormatInt(authDate, 10))
	v.Set("user", userJSON)

	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k + "=" + v.Get(k))
	}

	secretMAC := hmac.New(sha256.New, []byte("WebAppData"))
	secretMAC.Write([]byte(botToken))
	secret := secretMAC.Sum(nil)

	dataMAC := hmac.New(sha256.New, secret)
	dataMAC.Write([]byte(b.String()))
	h := hex.EncodeToString(dataMAC.Sum(nil))

	v.Set("hash", h)
	return v.Encode()
}

// newTestServiceForWebApp returns a Service wired with a telegramStubRepo and
// NoopTxRunner, identical to newTestServiceWithStubs (used for RegisterViaTelegram
// tests) but returned directly so this test file can reuse the same stubs.
func newTestServiceForWebApp(t *testing.T) (*service.Service, *telegramStubRepo) {
	t.Helper()
	repo := newTelegramStubRepo()
	pub := &noopPublisher{}
	fixedNow := time.Unix(1_700_000_000, 0)
	clk := clock.NewMock(fixedNow)
	jwtIssuer := newTestJWT(t)
	sessions := service.NewSessionIssuer(repo, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
	svc := service.NewService(repo, pub, txmanagertest.NoopTxRunner{}, jwtIssuer, clk, 15*time.Minute, 7*24*time.Hour, sessions)
	return svc, repo
}

// TestLoginViaTelegramWebApp_EmptyToken verifies that an empty bot token
// returns ErrTelegramAuthUnavailable immediately without touching the repo.
func TestLoginViaTelegramWebApp_EmptyToken(t *testing.T) {
	svc, repo := newTestServiceForWebApp(t)

	result, err := svc.LoginViaTelegramWebApp(
		context.Background(),
		"some-init-data",
		"",         // empty bot token
		"tenant-1",
		"127.0.0.1",
		"test-agent",
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, service.ErrTelegramAuthUnavailable)
	assert.Nil(t, result)
	assert.Equal(t, 0, repo.createCalls, "no repo write must occur on empty token")
}

// TestLoginViaTelegramWebApp_HappyPath verifies the full flow: valid initData
// registers the Telegram user and issues a session. The returned LoginResult
// must have non-empty tokens, TelegramID set, and TenantID == tenantID.
func TestLoginViaTelegramWebApp_HappyPath(t *testing.T) {
	const (
		botToken = "123456:test-bot-token"
		tenantID = "tenant-abc"
	)

	svc, repo := newTestServiceForWebApp(t)

	// clock is fixed at 1_700_000_000; auth_date 10 s ago is within 24 h.
	fixedNow := time.Unix(1_700_000_000, 0)
	userJSON := `{"id":42,"first_name":"Alice","last_name":"Smith","username":"alice"}`
	initData := buildTgInitData(t, botToken, fixedNow.Unix()-10, userJSON)

	result, err := svc.LoginViaTelegramWebApp(
		context.Background(),
		initData,
		botToken,
		tenantID,
		"127.0.0.1",
		"Mozilla/5.0",
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	require.NotNil(t, result.User)
	require.NotNil(t, result.User.TelegramID, "user must have TelegramID set")
	assert.Equal(t, int64(42), *result.User.TelegramID)
	require.NotNil(t, result.User.TenantID, "user must have TenantID set")
	assert.Equal(t, tenantID, *result.User.TenantID)

	// Exactly one user must be created (RegisterViaTelegram persisted it).
	assert.Equal(t, 1, repo.createCalls)
	// Exactly one session must be persisted (issueSession).
	assert.Len(t, repo.adminStubRepo.sessions, 1)
}

// TestLoginViaTelegramWebApp_TamperedInitData verifies that a corrupted
// initData payload is rejected with an error and no session is created.
func TestLoginViaTelegramWebApp_TamperedInitData(t *testing.T) {
	const (
		botToken = "123456:test-bot-token"
		tenantID = "tenant-abc"
	)

	svc, repo := newTestServiceForWebApp(t)

	fixedNow := time.Unix(1_700_000_000, 0)
	userJSON := `{"id":42,"first_name":"Eve","username":"eve"}`
	good := buildTgInitData(t, botToken, fixedNow.Unix()-10, userJSON)
	// Tamper: append a character to corrupt the hash.
	tampered := good + "x"

	result, err := svc.LoginViaTelegramWebApp(
		context.Background(),
		tampered,
		botToken,
		tenantID,
		"127.0.0.1",
		"Mozilla/5.0",
	)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, repo.createCalls, "no repo write must occur on tampered initData")
	assert.Empty(t, repo.adminStubRepo.sessions, "no session must be created on tampered initData")
}
