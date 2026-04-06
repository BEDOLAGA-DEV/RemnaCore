package postgres

import "context"

// TryAdvisoryLock exports tryAdvisoryLock for integration tests in the
// postgres_test package.
func (pm *PartitionManager) TryAdvisoryLock(ctx context.Context) bool {
	return pm.tryAdvisoryLock(ctx)
}

// ReleaseAdvisoryLock exports releaseAdvisoryLock for integration tests in
// the postgres_test package.
func (pm *PartitionManager) ReleaseAdvisoryLock(ctx context.Context) {
	pm.releaseAdvisoryLock(ctx)
}
