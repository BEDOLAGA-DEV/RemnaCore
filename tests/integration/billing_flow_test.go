//go:build integration

package integration_test

import (
	"testing"
)

// TestBillingFlow validates the full billing lifecycle end-to-end:
// list plans -> create subscription -> pay invoice -> cancel subscription.
//
// This test requires a full running stack (PostgreSQL + Valkey + NATS) and is
// gated behind the "integration" build tag. Run with:
//
//	go test -tags=integration ./tests/integration/ -v
//
// Individual subtests are currently skipped until testcontainers setup is added.
func TestBillingFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("list active plans", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// GET /api/plans
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		// Response: array of active plans
	})

	t.Run("get single plan", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// GET /api/plans/{planID}
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		// Response: plan object with addons
	})

	t.Run("create subscription", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// POST /api/subscriptions
		// Header: Authorization: Bearer <access_token>
		// Body: {"plan_id": "<uuid>", "addon_ids": []}
		// Expect: 201 Created
		// Response: { "subscription": {...}, "invoice": {...} }
		//
		// Capture subscription ID and invoice ID for subsequent subtests.
	})

	t.Run("list user subscriptions", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// GET /api/subscriptions
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		// Response: array containing the created subscription
	})

	t.Run("pay invoice", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// POST /api/invoices/{invoiceID}/pay
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		// Response: { "status": "paid" }
		//
		// Verify the subscription transitions from trial to active.
	})

	t.Run("list invoices", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// GET /api/invoices
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		// Response: array of pending invoices (should be empty after payment)
	})

	t.Run("cancel subscription", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// POST /api/subscriptions/{subID}/cancel
		// Header: Authorization: Bearer <access_token>
		// Expect: 200 OK
		// Response: { "status": "cancelled" }
		//
		// Verify subscription status transitions to cancelled.
	})

	t.Run("cancel already cancelled subscription returns conflict", func(t *testing.T) {
		t.Skip("requires full stack -- will be enabled with testcontainers setup")

		// POST /api/subscriptions/{subID}/cancel
		// Header: Authorization: Bearer <access_token>
		// Expect: 409 Conflict
		// Response: { "error": "invalid subscription state transition" }
	})
}
