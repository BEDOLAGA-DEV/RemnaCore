// Package txmanager defines an interface for running functions within database
// transactions. Domain services use this interface to guarantee that business
// writes and outbox event inserts happen atomically, without importing any
// adapter or infrastructure package.
package txmanager

import "context"

// Runner executes a function within a database transaction. The transaction is
// committed if fn returns nil, rolled back otherwise. Implementations store the
// active transaction in the returned context so that repositories and the
// outbox publisher can participate transparently.
type Runner interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
