package pluginstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// ErrDocumentNotFound is returned when a collection document does not exist or
// does not belong to the requested plugin/collection.
var ErrDocumentNotFound = errors.New("collection document not found")

// Document represents a single JSON document within a plugin collection.
type Document struct {
	ID         string          `json:"id"`
	PluginSlug string          `json:"plugin_slug"`
	Collection string          `json:"collection"`
	Data       json.RawMessage `json:"data"`
	// TenantID is the owning shop tenant of the document, mirroring the
	// plugins.collections.tenant_id column. A nil value means a NULL DB
	// tenant_id, i.e. a platform-owned document. This is provenance, set by
	// the read path, and is the trustworthy signal of ownership — never the
	// document payload. A non-nil value identifies a specific shop's own row.
	TenantID  *string   `json:"tenant_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store defines the collection storage interface for plugin documents.
type Store interface {
	ListDocuments(ctx context.Context, pluginSlug, collection string) ([]Document, error)
	GetDocument(ctx context.Context, pluginSlug, collection, id string) (*Document, error)
	InsertDocument(ctx context.Context, pluginSlug, collection string, doc json.RawMessage) (*Document, error)
	UpdateDocument(ctx context.Context, pluginSlug, collection, id string, doc json.RawMessage) error
	DeleteDocument(ctx context.Context, pluginSlug, collection, id string) error
}
