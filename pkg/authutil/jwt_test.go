package authutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"
)

func generateTestKeys(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}
	return priv, &priv.PublicKey
}

func TestJWT_SignAndVerify(t *testing.T) {
	priv, pub := generateTestKeys(t)
	issuer := NewJWTIssuer(priv, pub)

	claims := UserClaims{
		UserID: "user-123",
		Email:  "user@example.com",
		Role:   "admin",
	}

	token, err := issuer.Sign(claims, 5*time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if token == "" {
		t.Fatal("Sign returned empty token")
	}

	got, err := issuer.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if got.UserID != claims.UserID {
		t.Errorf("UserID mismatch: got %q, want %q", got.UserID, claims.UserID)
	}
	if got.Email != claims.Email {
		t.Errorf("Email mismatch: got %q, want %q", got.Email, claims.Email)
	}
	if got.Role != claims.Role {
		t.Errorf("Role mismatch: got %q, want %q", got.Role, claims.Role)
	}
}

func TestJWT_Expired(t *testing.T) {
	priv, pub := generateTestKeys(t)
	issuer := NewJWTIssuer(priv, pub)

	claims := UserClaims{
		UserID: "user-456",
		Email:  "expired@example.com",
		Role:   "viewer",
	}

	token, err := issuer.Sign(claims, -1*time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	_, err = issuer.Verify(token)
	if err == nil {
		t.Fatal("Verify should return error for expired token")
	}
}

func TestJWT_InvalidSignature(t *testing.T) {
	priv1, pub1 := generateTestKeys(t)
	priv2, pub2 := generateTestKeys(t)
	_ = pub1

	issuer1 := NewJWTIssuer(priv1, nil)
	issuer2 := NewJWTIssuer(priv2, pub2)

	claims := UserClaims{
		UserID: "user-789",
		Email:  "wrong@example.com",
		Role:   "editor",
	}

	token, err := issuer1.Sign(claims, 5*time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	_, err = issuer2.Verify(token)
	if err == nil {
		t.Fatal("Verify should return error when signed with a different key")
	}
}

func TestJWT_ClaimsFields(t *testing.T) {
	priv, pub := generateTestKeys(t)
	issuer := NewJWTIssuer(priv, pub)

	claims := UserClaims{
		UserID: "uid-abc-def",
		Email:  "claims@test.io",
		Role:   "superadmin",
	}

	token, err := issuer.Sign(claims, 10*time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	got, err := issuer.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	if got.UserID != "uid-abc-def" {
		t.Errorf("UserID: got %q, want %q", got.UserID, "uid-abc-def")
	}
	if got.Email != "claims@test.io" {
		t.Errorf("Email: got %q, want %q", got.Email, "claims@test.io")
	}
	if got.Role != "superadmin" {
		t.Errorf("Role: got %q, want %q", got.Role, "superadmin")
	}
}
