package middleware

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

func generateTestKeys(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return priv, &priv.PublicKey
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	priv, pub := generateTestKeys(t)
	issuer := authutil.NewJWTIssuer(priv, pub)

	claims := authutil.UserClaims{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "customer",
	}
	token, err := issuer.Sign(claims, 5*time.Minute)
	require.NoError(t, err)

	var gotClaims *authutil.UserClaims
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims = GetClaims(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := Auth(issuer)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, gotClaims)
	assert.Equal(t, "user-123", gotClaims.UserID)
	assert.Equal(t, "test@example.com", gotClaims.Email)
	assert.Equal(t, "customer", gotClaims.Role)
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	priv, pub := generateTestKeys(t)
	issuer := authutil.NewJWTIssuer(priv, pub)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	handler := Auth(issuer)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "missing")
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	priv, pub := generateTestKeys(t)
	issuer := authutil.NewJWTIssuer(priv, pub)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	handler := Auth(issuer)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+"totally.invalid.token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid")
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	priv, pub := generateTestKeys(t)
	issuer := authutil.NewJWTIssuer(priv, pub)

	claims := authutil.UserClaims{
		UserID: "user-456",
		Email:  "expired@example.com",
		Role:   "customer",
	}
	// Sign with negative TTL to produce an already-expired token.
	token, err := issuer.Sign(claims, -1*time.Minute)
	require.NoError(t, err)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	handler := Auth(issuer)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(httpconst.HeaderAuthorization, httpconst.BearerPrefix+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid")
}
