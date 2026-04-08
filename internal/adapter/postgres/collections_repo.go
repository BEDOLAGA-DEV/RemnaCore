package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
		WHERE id = $2`

	deleteDocumentSQL = `DELETE FROM plugins.collections WHERE id = $1`

	deleteCollectionSQL = `
		DELETE FROM plugins.collections
		WHERE plugin_slug = $1 AND collection = $2`
)

// CollectionDocument represents a single JSON document within a plugin
// collection.
type CollectionDocument struct {
	ID         string          `json:"id"`
	PluginSlug string          `json:"plugin_slug"`
	Collection string          `json:"collection"`
	Data       json.RawMessage `json:"data"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// CollectionsRepository provides CRUD operations for plugin collections backed
// by PostgreSQL. It uses raw SQL queries against the plugins.collections table.
type CollectionsRepository struct {
	pool *pgxpool.Pool
}

// NewCollectionsRepository creates a CollectionsRepository.
func NewCollectionsRepository(pool *pgxpool.Pool) *CollectionsRepository {
	return &CollectionsRepository{pool: pool}
}

// ListDocuments returns all documents in a collection for a plugin.
func (r *CollectionsRepository) ListDocuments(ctx context.Context, pluginSlug, collection string) ([]CollectionDocument, error) {
	rows, err := r.pool.Query(ctx, listDocumentsSQL, pluginSlug, collection)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	docs := make([]CollectionDocument, 0)
	for rows.Next() {
		var d CollectionDocument
		if err := rows.Scan(&d.ID, &d.PluginSlug, &d.Collection, &d.Data, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}

	return docs, nil
}

// GetDocument returns a single document by ID within a plugin collection.
// Returns ErrCollectionDocNotFound if the document does not exist.
func (r *CollectionsRepository) GetDocument(ctx context.Context, pluginSlug, collection, id string) (*CollectionDocument, error) {
	var d CollectionDocument
	err := r.pool.QueryRow(ctx, getDocumentSQL, pluginSlug, collection, id).Scan(
		&d.ID, &d.PluginSlug, &d.Collection, &d.Data, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrCollectionDocNotFound
		}
		return nil, fmt.Errorf("get document: %w", err)
	}
	return &d, nil
}

// InsertDocument creates a new document in a collection and returns the created
// document with its generated ID and timestamps.
func (r *CollectionsRepository) InsertDocument(ctx context.Context, pluginSlug, collection string, doc json.RawMessage) (*CollectionDocument, error) {
	var d CollectionDocument
	err := r.pool.QueryRow(ctx, insertDocumentSQL, pluginSlug, collection, doc).Scan(
		&d.ID, &d.PluginSlug, &d.Collection, &d.Data, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert document: %w", err)
	}
	return &d, nil
}

// UpdateDocument updates the document content for an existing document by ID.
// Returns ErrCollectionDocNotFound if no row was affected.
func (r *CollectionsRepository) UpdateDocument(ctx context.Context, id string, doc json.RawMessage) error {
	tag, err := r.pool.Exec(ctx, updateDocumentSQL, doc, id)
	if err != nil {
		return fmt.Errorf("update document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCollectionDocNotFound
	}
	return nil
}

// DeleteDocument removes a document by ID.
// Returns ErrCollectionDocNotFound if no row was affected.
func (r *CollectionsRepository) DeleteDocument(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, deleteDocumentSQL, id)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrCollectionDocNotFound
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
