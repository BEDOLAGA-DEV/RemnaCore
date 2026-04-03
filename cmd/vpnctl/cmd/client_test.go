package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIClient_Get(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/admin/plugins", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	c := &apiClient{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	body, status, err := c.get("/api/admin/plugins")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(body), `"data"`)
}

func TestAPIClient_Post(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "bar", payload["foo"])

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	c := &apiClient{
		baseURL:    server.URL,
		token:      "test-token",
		httpClient: server.Client(),
	}

	body, status, err := c.post("/api/admin/plugins", map[string]string{"foo": "bar"})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, status)
	assert.Contains(t, string(body), `"id"`)
}

func TestAPIClient_Delete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/admin/plugins/abc", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := &apiClient{
		baseURL:    server.URL,
		token:      "",
		httpClient: server.Client(),
	}

	_, status, err := c.delete("/api/admin/plugins/abc")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, status)
}

func TestNewAPIClient_DefaultValues(t *testing.T) {
	// Save and restore package-level vars.
	origURL := apiURL
	origToken := apiToken
	t.Cleanup(func() {
		apiURL = origURL
		apiToken = origToken
	})

	apiURL = "http://custom:9000"
	apiToken = "my-token"

	c := newAPIClient()
	assert.Equal(t, "http://custom:9000", c.baseURL)
	assert.Equal(t, "my-token", c.token)
}
