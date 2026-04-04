package plugin

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestHostFunctions creates a HostFunctions wired for testing. The
// pluginRegistry always returns the given plugin. If httpClient is non-nil it
// replaces the default SSRF-safe client (useful for httptest.Server tests).
func newTestHostFunctions(t *testing.T, p *Plugin, httpClient *http.Client) *HostFunctions {
	t.Helper()
	logger := slog.Default()
	hf := NewHostFunctions(logger, clock.NewReal())
	hf.SetPluginRegistry(func(slug string) (*Plugin, error) {
		if slug == p.Slug {
			return p, nil
		}
		return nil, ErrPluginNotFound
	})
	if httpClient != nil {
		hf.HTTPClient = httpClient
	}
	return hf
}

// newTestHostFunctionsNoSSRF is like newTestHostFunctions but disables the
// pre-flight hostname SSRF check. Use this when the test HTTP server binds to
// 127.0.0.1 (httptest.NewServer) and the test needs to exercise the full
// request pipeline without the SSRF guard interfering.
func newTestHostFunctionsNoSSRF(t *testing.T, p *Plugin, httpClient *http.Client) *HostFunctions {
	t.Helper()
	hf := newTestHostFunctions(t, p, httpClient)
	hf.urlChecker = func(string) (bool, error) { return false, nil }
	return hf
}

// testPluginWithHTTP creates a minimal Plugin whose manifest has the given HTTP
// allowlist. The Slug is always "test-plugin".
func testPluginWithHTTP(httpAllowlist []string) *Plugin {
	return &Plugin{
		ID:   "test-id",
		Slug: "test-plugin",
		Manifest: &Manifest{
			Permissions: ManifestPermissions{
				HTTP: httpAllowlist,
			},
		},
	}
}

func TestHTTPRequest_AllowedExternalURL(t *testing.T) {
	// httptest.NewServer binds to 127.0.0.1. We disable the pre-flight SSRF
	// check so we can test the allowlist + request execution + response
	// parsing pipeline end-to-end. SSRF is tested independently.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	p := testPluginWithHTTP([]string{server.URL + "/*"})
	hf := newTestHostFunctionsNoSSRF(t, p, server.Client())

	resp, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    server.URL + "/v1/charges",
	})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"status":"ok"}`, string(resp.Body))
	assert.Equal(t, "ok", resp.Headers["X-Test"])
}

func TestHTTPRequest_DeniedExternalURL(t *testing.T) {
	p := testPluginWithHTTP([]string{"https://api.stripe.com/*"})
	hf := newTestHostFunctions(t, p, nil)

	_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "https://evil.com/callback",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPermissionDenied),
		"expected ErrPermissionDenied, got: %v", err)
}

func TestHTTPRequest_BlockedInternalURL_Localhost(t *testing.T) {
	// Even if a plugin declares localhost in the allowlist, the SSRF guard
	// blocks it. The exact match must pass the allowlist first.
	p := testPluginWithHTTP([]string{"http://localhost:3000/*"})
	hf := newTestHostFunctions(t, p, nil)

	_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "http://localhost:3000/api",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInternalNetworkAccess),
		"expected ErrInternalNetworkAccess, got: %v", err)
}

func TestHTTPRequest_BlockedInternalURL_LoopbackIP(t *testing.T) {
	p := testPluginWithHTTP([]string{"http://127.0.0.1:4000/*"})
	hf := newTestHostFunctions(t, p, nil)

	_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "http://127.0.0.1:4000/api",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInternalNetworkAccess),
		"expected ErrInternalNetworkAccess, got: %v", err)
}

func TestHTTPRequest_BlockedInternalURL_IPv6Loopback(t *testing.T) {
	p := testPluginWithHTTP([]string{"http://[::1]:8080/*"})
	hf := newTestHostFunctions(t, p, nil)

	_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "http://[::1]:8080/",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInternalNetworkAccess),
		"expected ErrInternalNetworkAccess, got: %v", err)
}

func TestHTTPRequest_PluginNotFound(t *testing.T) {
	p := testPluginWithHTTP(nil)
	hf := newTestHostFunctions(t, p, nil)

	_, err := hf.HTTPRequest(context.Background(), "nonexistent-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "https://example.com",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPluginNotFound),
		"expected ErrPluginNotFound, got: %v", err)
}

func TestHTTPRequest_EmptyAllowlist(t *testing.T) {
	p := testPluginWithHTTP(nil)
	hf := newTestHostFunctions(t, p, nil)

	_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "https://api.stripe.com/v1/charges",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPermissionDenied),
		"expected ErrPermissionDenied, got: %v", err)
}

func TestHTTPRequest_NilManifest(t *testing.T) {
	p := &Plugin{
		ID:       "test-id",
		Slug:     "test-plugin",
		Manifest: nil,
	}
	hf := newTestHostFunctions(t, p, nil)

	_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "https://api.stripe.com/v1/charges",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPermissionDenied),
		"expected ErrPermissionDenied, got: %v", err)
}

func TestHTTPRequest_RegistryNotConfigured(t *testing.T) {
	hf := NewHostFunctions(slog.Default(), clock.NewReal())

	_, err := hf.HTTPRequest(context.Background(), "any-slug", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "https://example.com",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin registry not configured")
}

func TestHTTPRequest_PostWithBody(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body error", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()
		receivedBody = body
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	p := testPluginWithHTTP([]string{server.URL + "/*"})
	hf := newTestHostFunctionsNoSSRF(t, p, server.Client())

	reqBody := []byte(`{"amount":1000,"currency":"usd"}`)
	resp, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
		Method:  http.MethodPost,
		URL:     server.URL + "/v1/charges",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    reqBody,
	})

	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, reqBody, receivedBody)
}

func TestHTTPRequest_BlockedInternalURLTable(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		allowlist string
	}{
		{"localhost", "http://localhost:3000/api", "http://localhost:3000/*"},
		{"127.0.0.1", "http://127.0.0.1:4000/api", "http://127.0.0.1:4000/*"},
		{"0.0.0.0", "http://0.0.0.0:5000/api", "http://0.0.0.0:5000/*"},
		{"IPv6 loopback", "http://[::1]:8080/path", "http://[::1]:8080/*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := testPluginWithHTTP([]string{tt.allowlist})
			hf := newTestHostFunctions(t, p, nil)

			_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
				Method: http.MethodGet,
				URL:    tt.url,
			})

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInternalNetworkAccess),
				"URL %q should be blocked, got: %v", tt.url, err)
		})
	}
}

func TestHTTPRequest_InternalURL_DeniedByAllowlistFirst(t *testing.T) {
	// When an internal URL is NOT in the allowlist, the allowlist check fires
	// first, returning ErrPermissionDenied (not ErrInternalNetworkAccess).
	// This is correct: the allowlist is layer 1, SSRF guard is layer 2.
	p := testPluginWithHTTP([]string{"https://api.stripe.com/*"})
	hf := newTestHostFunctions(t, p, nil)

	_, err := hf.HTTPRequest(context.Background(), "test-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    "http://192.168.1.1/admin",
	})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPermissionDenied),
		"expected ErrPermissionDenied (allowlist layer), got: %v", err)
}

func TestHTTPRequest_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := testPluginWithHTTP([]string{server.URL + "/*"})
	hf := newTestHostFunctionsNoSSRF(t, p, server.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := hf.HTTPRequest(ctx, "test-plugin", sdk.HTTPRequest{
		Method: http.MethodGet,
		URL:    server.URL + "/v1/test",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}
