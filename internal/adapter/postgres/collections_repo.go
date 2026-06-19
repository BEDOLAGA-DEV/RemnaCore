package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// Collection document SQL statements.
const (
	listDocumentsSQL = `
		SELECT id, plugin_slug, collection, document, tenant_id, created_at, updated_at
		FROM plugins.collections
		WHERE plugin_slug = $1 AND collection = $2
		ORDER BY created_at DESC`

	getDocumentSQL = `
		SELECT id, plugin_slug, collection, document, tenant_id, created_at, updated_at
		FROM plugins.collections
		WHERE plugin_slug = $1 AND collection = $2 AND id = $3`

	insertDocumentSQL = `
		INSERT INTO plugins.collections (plugin_slug, collection, document, tenant_id)
		VALUES ($1, $2, $3, NULLIF(current_setting('app.tenant_id', true), '*')::uuid)
		RETURNING id, plugin_slug, collection, document, created_at, updated_at`

	updateDocumentSQL = `
		UPDATE plugins.collections
		SET document = $1, updated_at = now()
		WHERE id = $2 AND plugin_slug = $3 AND collection = $4`

	deleteDocumentSQL = `DELETE FROM plugins.collections WHERE id = $1 AND plugin_slug = $2 AND collection = $3`

	deleteCollectionSQL = `
		DELETE FROM plugins.collections
		WHERE plugin_slug = $1 AND collection = $2`
)

// CollectionsRepository provides CRUD operations for plugin collections backed
// by PostgreSQL. Every method runs inside runner.RunInTx so the per-request
// app.tenant_id GUC (set by RunInTx from the tenant context) is in effect for
// the row-level-security policy on plugins.collections, and resolves its
// connection via DBFromContext so it joins any already-open transaction.
type CollectionsRepository struct {
	pool   *pgxpool.Pool
	runner txmanager.Runner
}

// NewCollectionsRepository creates a CollectionsRepository. The runner is used
// to wrap every method in a transaction so RLS session variables apply.
func NewCollectionsRepository(pool *pgxpool.Pool, runner txmanager.Runner) *CollectionsRepository {
	return &CollectionsRepository{pool: pool, runner: runner}
}

// CollectionsRepository implements the pluginstore.Store interface.
var _ pluginstore.Store = (*CollectionsRepository)(nil)

// ListDocuments returns all documents in a collection for a plugin. The read
// runs inside RunInTx so the app.tenant_id GUC is set and RLS on
// plugins.collections scopes the result to the active tenant (or all rows
// under the platform sentinel).
func (r *CollectionsRepository) ListDocuments(ctx context.Context, pluginSlug, collection string) ([]pluginstore.Document, error) {
	docs := make([]pluginstore.Document, 0)
	err := r.runner.RunInTx(ctx, func(ctx context.Context) error {
		db := DBFromContext(ctx, r.pool)
		rows, err := db.Query(ctx, listDocumentsSQL, pluginSlug, collection)
		if err != nil {
			return fmt.Errorf("list documents: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var d pluginstore.Document
			var tenantID pgtype.UUID
			if err := rows.Scan(&d.ID, &d.PluginSlug, &d.Collection, &d.Data, &tenantID, &d.CreatedAt, &d.UpdatedAt); err != nil {
				return fmt.Errorf("scan document: %w", err)
			}
			d.TenantID = pgutil.PgtypeUUIDToOptStr(tenantID)
			docs = append(docs, d)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate documents: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, nil
}

// GetDocument returns a single document by ID within a plugin collection.
// Returns pluginstore.ErrDocumentNotFound if the document does not exist (or
// is hidden from the active tenant by RLS). The read runs inside RunInTx so
// the app.tenant_id GUC is applied.
func (r *CollectionsRepository) GetDocument(ctx context.Context, pluginSlug, collection, id string) (*pluginstore.Document, error) {
	var d pluginstore.Document
	err := r.runner.RunInTx(ctx, func(ctx context.Context) error {
		db := DBFromContext(ctx, r.pool)
		var tenantID pgtype.UUID
		scanErr := db.QueryRow(ctx, getDocumentSQL, pluginSlug, collection, id).Scan(
			&d.ID, &d.PluginSlug, &d.Collection, &d.Data, &tenantID, &d.CreatedAt, &d.UpdatedAt,
		)
		if scanErr != nil {
			if errors.Is(scanErr, pgx.ErrNoRows) {
				return pluginstore.ErrDocumentNotFound
			}
			return fmt.Errorf("get document: %w", scanErr)
		}
		d.TenantID = pgutil.PgtypeUUIDToOptStr(tenantID)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// InsertDocument creates a new document in a collection and returns the created
// document. The tenant_id column is self-stamped from the app.tenant_id GUC
// (sentinel -> NULL platform doc; shop GUC -> shop UUID), so the caller never
// passes a tenant and a shop GUC can never write a foreign tenant_id. Runs
// inside RunInTx so the GUC is in effect.
func (r *CollectionsRepository) InsertDocument(ctx context.Context, pluginSlug, collection string, doc json.RawMessage) (*pluginstore.Document, error) {
	var d pluginstore.Document
	err := r.runner.RunInTx(ctx, func(ctx context.Context) error {
		db := DBFromContext(ctx, r.pool)
		scanErr := db.QueryRow(ctx, insertDocumentSQL, pluginSlug, collection, doc).Scan(
			&d.ID, &d.PluginSlug, &d.Collection, &d.Data, &d.CreatedAt, &d.UpdatedAt,
		)
		if scanErr != nil {
			return fmt.Errorf("insert document: %w", scanErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// UpdateDocument updates the document content for an existing document by ID,
// scoped to the given plugin slug. Runs inside RunInTx so the app.tenant_id
// GUC applies (RLS USING hides foreign-tenant rows -> 0 rows affected ->
// ErrDocumentNotFound). Returns pluginstore.ErrDocumentNotFound if no row was
// affected.
func (r *CollectionsRepository) UpdateDocument(ctx context.Context, pluginSlug, collection, id string, doc json.RawMessage) error {
	return r.runner.RunInTx(ctx, func(ctx context.Context) error {
		db := DBFromContext(ctx, r.pool)
		tag, err := db.Exec(ctx, updateDocumentSQL, doc, id, pluginSlug, collection)
		if err != nil {
			return fmt.Errorf("update document: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return pluginstore.ErrDocumentNotFound
		}
		return nil
	})
}

// DeleteDocument removes a document by ID, scoped to the given plugin slug.
// Runs inside RunInTx so the app.tenant_id GUC applies. Returns
// pluginstore.ErrDocumentNotFound if no row was affected.
func (r *CollectionsRepository) DeleteDocument(ctx context.Context, pluginSlug, collection, id string) error {
	return r.runner.RunInTx(ctx, func(ctx context.Context) error {
		db := DBFromContext(ctx, r.pool)
		tag, err := db.Exec(ctx, deleteDocumentSQL, id, pluginSlug, collection)
		if err != nil {
			return fmt.Errorf("delete document: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return pluginstore.ErrDocumentNotFound
		}
		return nil
	})
}

// DeleteCollection removes all documents in a collection for a plugin. Runs
// inside RunInTx so the app.tenant_id GUC applies — under a shop tenant only
// that tenant's documents are removed; under the platform sentinel all are.
func (r *CollectionsRepository) DeleteCollection(ctx context.Context, pluginSlug, collection string) error {
	return r.runner.RunInTx(ctx, func(ctx context.Context) error {
		db := DBFromContext(ctx, r.pool)
		if _, err := db.Exec(ctx, deleteCollectionSQL, pluginSlug, collection); err != nil {
			return fmt.Errorf("delete collection: %w", err)
		}
		return nil
	})
}
