package service

import (
	"context"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// RBACCatalogSync idempotently projects the Go permission catalog and system
// roles into the database and backfills legacy role assignments. It runs once at
// startup (before serving traffic) and is safe to run repeatedly.
type RBACCatalogSync struct {
	repo     rbac.Repository
	txRunner txmanager.Runner
}

// NewRBACCatalogSync constructs the sync.
func NewRBACCatalogSync(repo rbac.Repository, txRunner txmanager.Runner) *RBACCatalogSync {
	return &RBACCatalogSync{repo: repo, txRunner: txRunner}
}

// Run performs the catalog sync inside a single transaction.
func (s *RBACCatalogSync) Run(ctx context.Context) error {
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		return s.repo.SyncCatalog(txCtx, rbac.Catalog(), rbac.SystemRoles())
	}); err != nil {
		return fmt.Errorf("rbac catalog sync: %w", err)
	}
	return nil
}
