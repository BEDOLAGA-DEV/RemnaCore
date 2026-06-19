//go:build integration

// Package postgres_test — TxManager re-entrancy integration test.
//
// Verifies that a nested RunInTx call (inner) joins the outer transaction
// rather than beginning an independent one. The test:
//
//  1. Begins an outer RunInTx.
//  2. Inside the outer closure, calls RunInTx again (inner) and writes a row.
//  3. Returns an error from the OUTER closure (after the inner RunInTx succeeds).
//  4. Asserts that the row written by the inner call was rolled back together
//     with the outer transaction — i.e. the inner write was NOT committed.
//
// Run with:
//
//	go test -tags integration -v ./internal/adapter/postgres/ -run TestTxManager_Reentrant
package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
)

// TestTxManager_Reentrant proves that a nested RunInTx joins the outer
// transaction. When the outer closure returns an error the inner write is
// rolled back together — atomicity is preserved end-to-end.
func TestTxManager_Reentrant(t *testing.T) {
	// Apply only the identity migration: we only need a real table to write to.
	pool, _ := setupTestDBWith(t, "001_identity.sql")
	tm := postgres.NewTxManager(pool)
	ctx := context.Background()

	// A sentinel error to trigger rollback of the outer transaction.
	outerErr := errors.New("intentional outer rollback")

	// The user ID we will try to insert inside the inner RunInTx.
	userID := uuid.Must(uuid.NewV7()).String()

	err := tm.RunInTx(ctx, func(outerCtx context.Context) error {
		// Inner RunInTx: must join the outer tx (re-entrant path).
		innerErr := tm.RunInTx(outerCtx, func(innerCtx context.Context) error {
			_, execErr := pool.Exec(innerCtx,
				`INSERT INTO identity.platform_users (id, email, password_hash, role, email_verified)
				 VALUES ($1, $2, 'hash', 'customer', false)`,
				userID, "reentrant@test.com",
			)
			return execErr
		})
		if innerErr != nil {
			return innerErr
		}
		// Intentionally fail the outer closure AFTER the inner RunInTx succeeded.
		// If the inner call had begun a separate tx and committed, the row would
		// persist; with correct re-entrancy the row must roll back here.
		return outerErr
	})

	require.ErrorIs(t, err, outerErr, "outer RunInTx must return the outer error")

	// Assert: the row written inside the inner RunInTx must NOT be present
	// because the inner write participated in the outer transaction and was
	// rolled back with it.
	var count int
	rowErr := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM identity.platform_users WHERE id = $1`, userID,
	).Scan(&count)
	require.NoError(t, rowErr)
	assert.Equal(t, 0, count,
		"inner write must be rolled back together with the outer transaction (re-entrancy)")
}
