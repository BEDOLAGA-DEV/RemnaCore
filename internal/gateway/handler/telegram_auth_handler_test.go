package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/identitytest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	resellerservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway/middleware"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
	"github.com/stretchr/testify/mock"
)

// buildTelegramInitData constructs a valid Telegram WebApp initData payload
// signed with the given bot token. The auth_date is set to authDate (Unix seconds).
func buildTelegramInitData(t *testing.T, botToken string, authDate int64, userJSON string) string {
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
	v.Set("hash", hex.EncodeToString(dataMAC.Sum(nil)))
	return v.Encode()
}

// newTestTelegramAuthHandler builds a TelegramAuthHandler wired with:
//   - a ResellerService backed by botRepo (other repo deps nil; bot ops don't touch them)
//   - an identity.Service backed by the given MockRepository, mock clock, and noop publisher
func newTestTelegramAuthHandler(
	t *testing.T,
	botRepo resellerservice.ShopBotRepository,
	identityRepo *identitytest.MockRepository,
	clk clock.Clock,
) (*TelegramAuthHandler, *identitytest.MockPublisher) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	resellerSvc := reseller.NewResellerService(
		nil, nil, nil,
		noopPublisherForHandler{},
		logger,
		nil,
		txmanagertest.NoopTxRunner{},
		botRepo,
		resellerservice.AllowAllBotPlugins{},
	)

	pub := new(identitytest.MockPublisher)
	key := generateTestECDSAKey(t)
	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)
	sessions := identity.NewSessionIssuer(identityRepo, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
	identitySvc := identity.NewService(
		identityRepo, pub, txmanagertest.NoopTxRunner{}, jwtIssuer, clk,
		15*time.Minute, 7*24*time.Hour, sessions,
	)

	return NewTelegramAuthHandler(identitySvc, resellerSvc), pub
}

// --- Tests ---

// TestWebAppLogin_HappyPath verifies that a valid shop_id + correctly signed
// initData yields 200 with access_token and refresh_token, and that the bot
// token itself is absent from the response body.
func TestWebAppLogin_HappyPath(t *testing.T) {
	const (
		botToken = "987654321:AABotToken_HandlerTest"
		shopID   = "shop-abc"
	)

	fixedNow := time.Unix(1_700_000_000, 0)
	clk := clock.NewMock(fixedNow)

	userJSON := `{"id":99,"first_name":"Bob","last_name":"Test","username":"bobtest"}`
	initData := buildTelegramInitData(t, botToken, fixedNow.Unix()-10, userJSON)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, int64(99)).Return(nil, identity.ErrNotFound)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*aggregate.PlatformUser")).Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*aggregate.Session")).Return(nil)

	botRepo := &botStubRepo{
		cfg: &vo.ShopBotConfig{
			Token:   config.NewSecretString(botToken),
			Enabled: true,
		},
	}

	h, pub := newTestTelegramAuthHandler(t, botRepo, repo, clk)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	body, _ := json.Marshal(map[string]string{
		"shop_id":   shopID,
		"init_data": initData,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/webapp", bytes.NewReader(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.WebAppLogin(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.NotEmpty(t, resp["access_token"], "access_token must be present")
	assert.NotEmpty(t, resp["refresh_token"], "refresh_token must be present")
	assert.NotNil(t, resp["user"], "user must be present")

	// The bot token must never appear in the response body.
	assert.NotContains(t, rec.Body.String(), botToken, "bot token must not leak into the response")

	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

// TestWebAppLogin_UnknownShop verifies that a shop_id whose bot config cannot
// be found results in a 401 (no detail about the missing config).
func TestWebAppLogin_UnknownShop(t *testing.T) {
	// botStubRepo with nil cfg returns ErrShopBotNotFound.
	botRepo := &botStubRepo{cfg: nil}
	repo := new(identitytest.MockRepository)

	h, _ := newTestTelegramAuthHandler(t, botRepo, repo, clock.NewReal())

	body, _ := json.Marshal(map[string]string{
		"shop_id":   "nonexistent-shop",
		"init_data": "auth_date=1700000000&hash=deadbeef&user=%7B%22id%22%3A1%7D",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/webapp", bytes.NewReader(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.WebAppLogin(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	// No identity repo interaction must occur.
	repo.AssertNotCalled(t, "GetUserByTelegramID", mock.Anything, mock.Anything)
}

// TestWebAppLogin_DisabledBot verifies that a shop whose bot config exists but
// is disabled (Enabled=false) also results in a 401.
func TestWebAppLogin_DisabledBot(t *testing.T) {
	botRepo := &botStubRepo{
		cfg: &vo.ShopBotConfig{
			Token:   config.NewSecretString("123456:disabledtoken"),
			Enabled: false,
		},
	}
	repo := new(identitytest.MockRepository)

	h, _ := newTestTelegramAuthHandler(t, botRepo, repo, clock.NewReal())

	body, _ := json.Marshal(map[string]string{
		"shop_id":   "shop-disabled",
		"init_data": "auth_date=1700000000&hash=deadbeef&user=%7B%22id%22%3A1%7D",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/webapp", bytes.NewReader(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.WebAppLogin(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	repo.AssertNotCalled(t, "GetUserByTelegramID", mock.Anything, mock.Anything)
}

// TestWebAppLogin_TokenNeverInResponse is an explicit contract test asserting
// that the bot token (the shop's secret) cannot appear in the response body
// regardless of outcome.  We use a known token string and verify it is absent
// even when the initData is invalid (which causes a 401).
func TestWebAppLogin_TokenNeverInResponse(t *testing.T) {
	const knownToken = "111222333:AASuperSecretBotTokenThatMustNeverLeak"

	botRepo := &botStubRepo{
		cfg: &vo.ShopBotConfig{
			Token:   config.NewSecretString(knownToken),
			Enabled: true,
		},
	}
	repo := new(identitytest.MockRepository)

	h, _ := newTestTelegramAuthHandler(t, botRepo, repo, clock.NewReal())

	// Deliberately malformed initData so identity returns an error.
	body, _ := json.Marshal(map[string]string{
		"shop_id":   "shop-leak-check",
		"init_data": "auth_date=1700000000&hash=badhash&user=%7B%22id%22%3A1%7D",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/webapp", bytes.NewReader(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.WebAppLogin(rec, req)

	// Verify the token is not in the response body (error or success path).
	assert.NotContains(t, rec.Body.String(), knownToken)
}

// TestWebAppLogin_MissingFields verifies the field semantics after shop_id was
// made optional: init_data is always required (422 when absent); init_data
// present but no shop resolvable (no shop_id and no domain tenant) → 401.
func TestWebAppLogin_MissingFields(t *testing.T) {
	botRepo := &botStubRepo{cfg: nil}
	repo := new(identitytest.MockRepository)
	h, _ := newTestTelegramAuthHandler(t, botRepo, repo, clock.NewReal())

	tests := []struct {
		name string
		body string
		code int
	}{
		{name: "missing init_data", body: `{"shop_id":"shop-1"}`, code: http.StatusUnprocessableEntity},
		{name: "both empty", body: `{}`, code: http.StatusUnprocessableEntity},
		{name: "init_data present but no shop resolvable", body: `{"init_data":"somedata"}`, code: http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/webapp", strings.NewReader(tc.body))
			req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
			rec := httptest.NewRecorder()
			h.WebAppLogin(rec, req)
			assert.Equal(t, tc.code, rec.Code)
		})
	}
}

// TestWebAppLogin_DomainResolvedShop verifies that when shop_id is omitted, the
// handler falls back to the tenant resolved from the request domain (set in the
// context by TenantResolver) and authenticates successfully.
func TestWebAppLogin_DomainResolvedShop(t *testing.T) {
	const (
		botToken = "555000111:AADomainResolvedToken"
		shopID   = "shop-from-domain"
	)

	fixedNow := time.Unix(1_700_000_000, 0)
	clk := clock.NewMock(fixedNow)

	userJSON := `{"id":77,"first_name":"Dom","username":"domuser"}`
	initData := buildTelegramInitData(t, botToken, fixedNow.Unix()-10, userJSON)

	repo := new(identitytest.MockRepository)
	repo.On("GetUserByTelegramID", mock.Anything, int64(77)).Return(nil, identity.ErrNotFound)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*aggregate.PlatformUser")).Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*aggregate.Session")).Return(nil)

	botRepo := &botStubRepo{cfg: &vo.ShopBotConfig{Token: config.NewSecretString(botToken), Enabled: true}}
	h, pub := newTestTelegramAuthHandler(t, botRepo, repo, clk)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	// No shop_id in the body — only init_data.
	body, _ := json.Marshal(map[string]string{"init_data": initData})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/telegram/webapp", bytes.NewReader(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	// Simulate TenantResolver having resolved the shop from the request domain.
	ctx := context.WithValue(req.Context(), middleware.TenantContextKey, &reseller.Tenant{ID: shopID, IsActive: true})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.WebAppLogin(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.NotEmpty(t, resp["access_token"])
	assert.NotEmpty(t, resp["refresh_token"])
	assert.NotContains(t, rec.Body.String(), botToken, "bot token must not leak into the response")

	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}
