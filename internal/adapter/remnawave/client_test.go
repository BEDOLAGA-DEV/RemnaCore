package remnawave

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/httpconst"
)

func TestClient_CreateUser(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and path.
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/users/", r.URL.Path)

		// Verify auth header.
		assert.Equal(t, httpconst.BearerPrefix+"test-token", r.Header.Get(httpconst.HeaderAuthorization))
		assert.Equal(t, httpconst.ContentTypeJSON, r.Header.Get(httpconst.HeaderContentType))

		// Verify request body.
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req CreateUserRequest
		require.NoError(t, json.Unmarshal(body, &req))
		assert.Equal(t, "p_testuser_main_0", req.Username)

		resp := APIResponse[RemnawaveUser]{
			Success: true,
			Data: RemnawaveUser{
				UUID:     "uuid-123",
				Username: req.Username,
				Status:   "active",
			},
		}
		w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	user, err := client.CreateUser(context.Background(), CreateUserRequest{
		Username:       "p_testuser_main_0",
		TrafficLimitBytes: 100,
		ExpireAt:       now.Add(30 * 24 * time.Hour),
	})

	require.NoError(t, err)
	assert.Equal(t, "uuid-123", user.UUID)
	assert.Equal(t, "p_testuser_main_0", user.Username)
	assert.Equal(t, "active", user.Status)
}

func TestClient_GetNodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/nodes/", r.URL.Path)
		assert.Equal(t, httpconst.BearerPrefix+"node-token", r.Header.Get(httpconst.HeaderAuthorization))

		resp := APIResponse[[]RemnawaveNode]{
			Success: true,
			Data: []RemnawaveNode{
				{UUID: "node-1", Name: "DE-1", Address: "1.2.3.4", Port: 443, IsConnected: true},
				{UUID: "node-2", Name: "US-1", Address: "5.6.7.8", Port: 443, IsConnected: false},
			},
		}
		w.Header().Set(httpconst.HeaderContentType, httpconst.ContentTypeJSON)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "node-token")
	nodes, err := client.GetNodes(context.Background())

	require.NoError(t, err)
	require.Len(t, nodes, 2)
	assert.Equal(t, "DE-1", nodes[0].Name)
	assert.True(t, nodes[0].IsConnected)
	assert.False(t, nodes[1].IsConnected)
}

func TestClient_DeleteUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/users/uuid-456", r.URL.Path)
		assert.Equal(t, httpconst.BearerPrefix+"del-token", r.Header.Get(httpconst.HeaderAuthorization))

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "del-token")
	err := client.DeleteUser(context.Background(), "uuid-456")

	require.NoError(t, err)
}

func TestClient_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"success":false,"message":"user not found"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "err-token")
	user, err := client.CreateUser(context.Background(), CreateUserRequest{
		Username: "test",
	})

	assert.Nil(t, user)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
	assert.Contains(t, err.Error(), "user not found")
}
