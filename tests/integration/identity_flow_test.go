//go:build integration

package integration_test

import (
	"testing"
)

// TestIdentityFlow validates the full identity lifecycle end-to-end:
// register -> login -> verify email -> access protected endpoint -> refresh token.
//
// This test requires a full running stack (PostgreSQL + Valkey + NATS) and is
// gated behind the "integration" build tag. Run with:
//
//	go test -tags=integration ./tests/integration/ -v
//
// Individual subtests are currently skipped until testcontainers setup is added.
func TestIdentityFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Shared state across subtests (populated by earlier steps, consumed by later ones).
	var (
		accessToken       string
		refreshToken      string
		verificationToken string
		userID            string
	)

	// Suppress unused-variable warnings; these will be used once subtests are implemented.
	_ = accessToken
	_ = refreshToken
	_ = verificationToken
	_ = userID

	t.Run("register new user", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// POST /api/auth/register
		// Body: {"email": "integration@example.com", "password": "IntTest!2026"}
		// Expect: 201 Created
		// Response: { "user_id": "<uuid>", "email": "integration@example.com", "verification_token": "<token>" }
		//
		// Capture user_id and verification_token for subsequent subtests.
	})

	t.Run("login with unverified email", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// POST /api/auth/login
		// Body: {"email": "integration@example.com", "password": "IntTest!2026"}
		// Expect: 200 OK (login works even without email verification for now)
		// Response: { "access_token": "<jwt>", "refresh_token": "<token>", "user": {...} }
		//
		// Capture access_token and refresh_token.
	})

	t.Run("verify email", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// POST /api/auth/verify-email
		// Body: {"token": "<verification_token from register step>"}
		// Expect: 200 OK
		// Response: { "status": "verified" }
	})

	t.Run("access protected endpoint", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// GET /api/me
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		// Response: { "id": "<user_id>", "email": "integration@example.com", "email_verified": true, ... }
		//
		// Verify that the returned profile matches the registered user.
	})

	t.Run("refresh token", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// POST /api/auth/refresh
		// Body: {"refresh_token": "<refresh_token from login step>"}
		// Expect: 200 OK
		// Response: { "access_token": "<new_jwt>", "refresh_token": "<new_refresh_token>" }
		//
		// Verify new tokens are different from original ones.
	})
}
