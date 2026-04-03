//go:build integration

package integration_test

import (
	"testing"
)

// TestProvisioningFlow validates that billing lifecycle events trigger the
// correct provisioning and deprovisioning of Remnawave bindings.
//
// This test requires a full running stack (PostgreSQL + Valkey + NATS + Remnawave)
// and is gated behind the "integration" build tag. Run with:
//
//	go test -tags=integration ./tests/integration/ -v
//
// Individual subtests are currently skipped until testcontainers setup is added.
func TestProvisioningFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("pay invoice triggers provisioning", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// Setup: create user, create subscription, pay invoice.
		// After payment the subscription transitions to active, which should
		// trigger the provisioning saga to create Remnawave bindings.
		//
		// GET /api/bindings
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		// Response: array containing at least one active binding with a
		//           remnawave_short_uuid set.
	})

	t.Run("list bindings by subscription", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// GET /api/subscriptions/{subID}/bindings
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		// Response: array of bindings for that subscription.
	})

	t.Run("cancel triggers deprovisioning", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// POST /api/subscriptions/{subID}/cancel
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		//
		// After cancellation, the deprovisioning saga should run. Bindings
		// should transition to the "deprovisioned" status.
		//
		// GET /api/bindings
		// Verify all bindings for the subscription are deprovisioned.
	})

	t.Run("pay invoice for plan with addons provisions multiple bindings", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// Setup: create user, create subscription with addon_ids (e.g. gaming
		// and streaming addons), pay invoice.
		//
		// GET /api/subscriptions/{subID}/bindings
		// Expect: multiple bindings with different purposes (base, gaming, streaming).
	})
}
