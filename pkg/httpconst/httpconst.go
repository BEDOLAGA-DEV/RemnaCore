// Package httpconst provides shared HTTP header names and MIME type constants
// used across the RemnaCore codebase.
package httpconst

const (
	// HeaderAuthorization is the HTTP Authorization header key.
	HeaderAuthorization = "Authorization"

	// HeaderContentType is the HTTP Content-Type header key.
	HeaderContentType = "Content-Type"

	// HeaderContentLength is the HTTP Content-Length header key.
	HeaderContentLength = "Content-Length"

	// HeaderCacheControl is the HTTP Cache-Control header key.
	HeaderCacheControl = "Cache-Control"

	// BearerPrefix is prepended to a token in the Authorization header.
	BearerPrefix = "Bearer "

	// ContentTypeJSON is the MIME type for JSON request/response bodies.
	ContentTypeJSON = "application/json"

	// ContentTypeOctetStream is the MIME type for arbitrary binary data.
	ContentTypeOctetStream = "application/octet-stream"

	// ContentTypePlain is the MIME type for plain UTF-8 text.
	ContentTypePlain = "text/plain; charset=utf-8"

	// CacheControlNoStore directs caches not to store a copy of the response.
	CacheControlNoStore = "no-store"

	// HeaderRequestID is the HTTP header used to propagate request IDs.
	HeaderRequestID = "X-Request-ID"

	// HeaderAPIKey is the HTTP header used to carry the tenant API key.
	HeaderAPIKey = "X-API-Key"

	// MaxWebhookBodySize is the maximum allowed size for incoming webhook
	// request bodies (1 MiB).
	MaxWebhookBodySize = 1 << 20
)
