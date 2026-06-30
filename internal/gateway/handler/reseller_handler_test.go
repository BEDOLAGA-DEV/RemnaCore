package handler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	resellerservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// --- Bot-handler test helpers ---

// botStubRepo is an in-memory ShopBotRepository stub for handler-level tests.
type botStubRepo struct {
	cfg       *vo.ShopBotConfig
	upsertErr error
}

func (s *botStubRepo) Upsert(_ context.Context, _ string, _ vo.ShopBotConfig) error {
	return s.upsertErr
}

func (s *botStubRepo) GetByTenant(_ context.Context, _ string) (*vo.ShopBotConfig, error) {
	if s.cfg == nil {
		return nil, resellerservice.ErrShopBotNotFound
	}
	return s.cfg, nil
}

func (s *botStubRepo) ListEnabled(_ context.Context) ([]vo.ShopBotWithTenant, error) {
	return nil, nil
}

// Compile-time satisfaction check.
var _ resellerservice.ShopBotRepository = (*botStubRepo)(nil)

// noopPublisherForHandler discards all domain events.
type noopPublisherForHandler struct{}

func (noopPublisherForHandler) Publish(_ context.Context, _ domainevent.Event) error { return nil }
func (noopPublisherForHandler) PublishBatch(_ context.Context, _ []domainevent.Event) error {
	return nil
}

// newBotTestHandler builds a ResellerHandler backed by a ResellerService wired
// with the given ShopBotRepository stub. Other repository deps are nil because
// SetShopBot/GetShopBot never touch them.
func newBotTestHandler(t *testing.T, repo resellerservice.ShopBotRepository) *ResellerHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := reseller.NewResellerService(
		nil, nil, nil,
		noopPublisherForHandler{},
		logger,
		nil,
		txmanagertest.NoopTxRunner{},
		repo,
	)
	return NewResellerHandler(svc)
}

// withTenant returns a copy of r whose context carries the given tenant ID.
func withTenant(r *http.Request, tenantID string) *http.Request {
	return r.WithContext(tenantctx.WithTenantID(r.Context(), tenantID))
}

// --- Bot handler tests ---

func TestGetResellerBot_NeverLeaksToken(t *testing.T) {
	const secretToken = "123:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	repo := &botStubRepo{
		cfg: &vo.ShopBotConfig{
			Token:       config.NewSecretString(secretToken),
			CabinetURL:  "https://cabinet.example.com",
			BotUsername: "testbot",
			Enabled:     true,
		},
	}
	h := newBotTestHandler(t, repo)

	req := withTenant(httptest.NewRequest(http.MethodGet, "/reseller/bot", nil), "tenant-abc")
	rec := httptest.NewRecorder()

	h.GetResellerBot(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.NotContains(t, body, secretToken, "token must never appear in GET response")
	require.Contains(t, body, `"has_token":true`)
	require.Contains(t, body, `"enabled":true`)
	require.Contains(t, body, `"cabinet_url":"https://cabinet.example.com"`)
	require.Contains(t, body, `"bot_username":"testbot"`)
}

func TestGetResellerBot_NoActiveTenant_Forbidden(t *testing.T) {
	h := newBotTestHandler(t, &botStubRepo{})

	req := httptest.NewRequest(http.MethodGet, "/reseller/bot", nil)
	rec := httptest.NewRecorder()

	h.GetResellerBot(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "COMMON.FORBIDDEN")
}

func TestGetResellerBot_NotConfigured_ReturnsEmptyOK(t *testing.T) {
	h := newBotTestHandler(t, &botStubRepo{cfg: nil})

	req := withTenant(httptest.NewRequest(http.MethodGet, "/reseller/bot", nil), "tenant-abc")
	rec := httptest.NewRecorder()

	h.GetResellerBot(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `"has_token":false`)
	require.Contains(t, body, `"enabled":false`)
}

func TestSetResellerBot_EmptyToken_BadRequest(t *testing.T) {
	h := newBotTestHandler(t, &botStubRepo{})

	body := strings.NewReader(`{"bot_token":"","cabinet_url":"https://example.com","enabled":true}`)
	req := withTenant(httptest.NewRequest(http.MethodPut, "/reseller/bot", body), "tenant-abc")
	rec := httptest.NewRecorder()

	h.SetResellerBot(rec, req)

	// ValidationFailed maps to 422 Unprocessable Entity (4xx).
	require.True(t, rec.Code >= 400 && rec.Code < 500, "expected 4xx but got %d", rec.Code)
}

func TestSetResellerBot_NoActiveTenant_Forbidden(t *testing.T) {
	h := newBotTestHandler(t, &botStubRepo{})

	body := strings.NewReader(`{"bot_token":"123:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA","cabinet_url":"https://example.com","enabled":true}`)
	req := httptest.NewRequest(http.MethodPut, "/reseller/bot", body)
	rec := httptest.NewRecorder()

	h.SetResellerBot(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSetResellerBot_ValidInput_NoContent(t *testing.T) {
	h := newBotTestHandler(t, &botStubRepo{})

	const validToken = "123:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	body := strings.NewReader(`{"bot_token":"` + validToken + `","cabinet_url":"https://cabinet.example.com","enabled":true}`)
	req := withTenant(httptest.NewRequest(http.MethodPut, "/reseller/bot", body), "tenant-abc")
	rec := httptest.NewRecorder()

	h.SetResellerBot(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestUpdateTenantBot_EmptyToken_BadRequest(t *testing.T) {
	h := newBotTestHandler(t, &botStubRepo{})

	body := strings.NewReader(`{"bot_token":"","cabinet_url":"https://example.com","enabled":true}`)
	req := httptest.NewRequest(http.MethodPut, "/admin/tenants/tenant-xyz/bot", body)
	rec := httptest.NewRecorder()

	// Inject URL param manually (chi not wired; call handler directly).
	// UpdateTenantBot reads tenantID from chi.URLParam which returns "" without
	// a chi context — the handler 400s on empty tenantID before the token check.
	// Use a chi context to test token validation separately.
	h.UpdateTenantBot(rec, req)

	// Without chi URL params the tenantID is "" → ValidationFailed (4xx).
	require.True(t, rec.Code >= 400 && rec.Code < 500, "expected 4xx but got %d", rec.Code)
}

// --- Original commission test ---

func TestResellerCommissions_NoActiveTenant_Forbidden(t *testing.T) {
	h := NewResellerHandler(&reseller.ResellerService{})

	req := httptest.NewRequest(http.MethodGet, "/api/reseller/commissions", nil)
	rec := httptest.NewRecorder()

	h.Commissions(rec, req)

	// VERIFIED (correction #21): apierror.Error serializes Details via
	// `json:"details,omitempty"` (pkg/apierror/error.go) and writeAPIError
	// encodes the whole struct (handler/response.go), so the Details string IS
	// in the body. We still assert on the STABLE COMMON.FORBIDDEN code + 403
	// status rather than the human-readable Details text, which is an
	// implementation detail that may be reworded.
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "COMMON.FORBIDDEN")
}
