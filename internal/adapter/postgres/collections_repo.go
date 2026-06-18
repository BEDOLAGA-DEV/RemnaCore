package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// Collection document SQL statements.
const (
	listDocumentsSQL = `
		SELECT id, plugin_slug, collection, document, created_at, updated_at
		FROM plugins.collections
		WHERE plugin_slug = $1 AND collection = $2
		ORDER BY created_at DESC`

	getDocumentSQL = `
		SELECT id, plugin_slug, collection, document, created_at, updated_at
		FROM plugins.collections
		WHERE plugin_slug = $1 AND collection = $2 AND id = $3`

	insertDocumentSQL = `
		INSERT INTO plugins.collections (plugin_slug, collection, document)
		VALUES ($1, $2, $3)
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
			if err := rows.Scan(&d.ID, &d.PluginSlug, &d.Collection, &d.Data, &d.CreatedAt, &d.UpdatedAt); err != nil {
				return fmt.Errorf("scan document: %w", err)
			}
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
		scanErr := db.QueryRow(ctx, getDocumentSQL, pluginSlug, collection, id).Scan(
			&d.ID, &d.PluginSlug, &d.Collection, &d.Data, &d.CreatedAt, &d.UpdatedAt,
		)
		if scanErr != nil {
			if errors.Is(scanErr, pgx.ErrNoRows) {
				return pluginstore.ErrDocumentNotFound
			}
			return fmt.Errorf("get document: %w", scanErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// InsertDocument creates a new document in a collection and returns the created
// document with its generated ID and timestamps.
func (r *CollectionsRepository) InsertDocument(ctx context.Context, pluginSlug, collection string, doc json.RawMessage) (*pluginstore.Document, error) {
	var d pluginstore.Document
	err := r.pool.QueryRow(ctx, insertDocumentSQL, pluginSlug, collection, doc).Scan(
		&d.ID, &d.PluginSlug, &d.Collection, &d.Data, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert document: %w", err)
	}
	return &d, nil
}

// UpdateDocument updates the document content for an existing document by ID,
// scoped to the given plugin slug.
// Returns pluginstore.ErrDocumentNotFound if no row was affected.
func (r *CollectionsRepository) UpdateDocument(ctx context.Context, pluginSlug, collection, id string, doc json.RawMessage) error {
	tag, err := r.pool.Exec(ctx, updateDocumentSQL, doc, id, pluginSlug, collection)
	if err != nil {
		return fmt.Errorf("update document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pluginstore.ErrDocumentNotFound
	}
	return nil
}

// DeleteDocument removes a document by ID, scoped to the given plugin slug.
// Returns pluginstore.ErrDocumentNotFound if no row was affected.
func (r *CollectionsRepository) DeleteDocument(ctx context.Context, pluginSlug, collection, id string) error {
	tag, err := r.pool.Exec(ctx, deleteDocumentSQL, id, pluginSlug, collection)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pluginstore.ErrDocumentNotFound
	}
	return nil
}

// DeleteCollection removes all documents in a collection for a plugin.
func (r *CollectionsRepository) DeleteCollection(ctx context.Context, pluginSlug, collection string) error {
	_, err := r.pool.Exec(ctx, deleteCollectionSQL, pluginSlug, collection)
	if err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	return nil
}
