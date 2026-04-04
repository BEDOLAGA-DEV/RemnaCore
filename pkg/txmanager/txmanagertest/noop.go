// Package txmanagertest provides shared test doubles for txmanager interfaces.
package txmanagertest

import "context"

// NoopTxRunner implements txmanager.Runner by executing fn directly without a
// real database transaction. Suitable for unit tests where repositories are mocked.
type NoopTxRunner struct{}

// RunInTx executes fn with the original context.
func (NoopTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
