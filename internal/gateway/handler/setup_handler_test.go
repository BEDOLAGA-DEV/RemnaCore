package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/identitytest"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newTestSetupHandler(t *testing.T) (*SetupHandler, *identitytest.MockRepository, *identitytest.MockPublisher) {
	t.Helper()

	repo := new(identitytest.MockRepository)
	pub := new(identitytest.MockPublisher)

	key := generateTestECDSAKey(t)
	jwtIssuer := authutil.NewJWTIssuer(key, &key.PublicKey)

	sessions := identity.NewSessionIssuer(repo, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
	svc := identity.NewService(repo, pub, txmanagertest.NoopTxRunner{}, jwtIssuer, clock.NewReal(), 15*time.Minute, 7*24*time.Hour, sessions)
	return NewSetupHandler(svc), repo, pub
}

func TestSetupStatus(t *testing.T) {
	tests := []struct {
		name      string
		admins    int64
		wantSetup bool
	}{
		{name: "no admins -> setup needed", admins: 0, wantSetup: true},
		{name: "admin exists -> setup not needed", admins: 1, wantSetup: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, _ := newTestSetupHandler(t)
			repo.On("CountAdmins", mock.Anything).Return(tt.admins, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
			rec := httptest.NewRecorder()

			h.Status(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			var resp map[string]bool
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
			assert.Equal(t, tt.wantSetup, resp["needs_setup"])
			repo.AssertExpectations(t)
		})
	}
}

func TestSetupCreateAdmin_Success(t *testing.T) {
	h, repo, pub := newTestSetupHandler(t)

	repo.On("AcquireBootstrapLock", mock.Anything).Return(nil)
	repo.On("CountAdmins", mock.Anything).Return(int64(0), nil)
	repo.On("CreateUser", mock.Anything, mock.AnythingOfType("*aggregate.PlatformUser")).Return(nil)
	repo.On("CreateSession", mock.Anything, mock.AnythingOfType("*aggregate.Session")).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	body := `{"email":"admin@example.com","password":"StrongP4ss"}`
	req := httptest.NewRequest(http.MethodPost, "/api/setup/admin", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.CreateAdmin(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.NotEmpty(t, resp["access_token"])
	assert.NotEmpty(t, resp["refresh_token"])
	assert.NotNil(t, resp["user"])
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestSetupCreateAdmin_AlreadyCompleted(t *testing.T) {
	h, repo, _ := newTestSetupHandler(t)

	repo.On("AcquireBootstrapLock", mock.Anything).Return(nil)
	repo.On("CountAdmins", mock.Anything).Return(int64(1), nil)

	body := `{"email":"admin2@example.com","password":"StrongP4ss"}`
	req := httptest.NewRequest(http.MethodPost, "/api/setup/admin", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.CreateAdmin(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "IDENTITY.SETUP_ALREADY_COMPLETED", resp["code"])
}

func TestSetupCreateAdmin_EmptyPassword(t *testing.T) {
	h, _, _ := newTestSetupHandler(t)

	body := `{"email":"admin@example.com","password":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/setup/admin", bytes.NewBufferString(body))
	req.Header.Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
	rec := httptest.NewRecorder()

	h.CreateAdmin(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}
