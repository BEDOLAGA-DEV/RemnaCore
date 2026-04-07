package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/health"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/routing"
)

func seedTestCache() *health.NodeHealthCache {
	cache := health.NewNodeHealthCache()
	cache.Update([]health.NodeHealth{
		{NodeID: "us1", Name: "US-East-01", IsOnline: true, CountryCode: "US", TrafficUsed: 0},
		{NodeID: "de1", Name: "DE-Frankfurt-01", IsOnline: true, CountryCode: "DE", TrafficUsed: 10 << 30},
	})
	return cache
}

func TestRoutingHandler_SelectNode_OK(t *testing.T) {
	cache := seedTestCache()
	router := routing.NewSmartRouter(cache, nil, nil, nil)
	h := NewRoutingHandler(router)

	body, _ := json.Marshal(routing.RouteRequest{
		UserCountry: "US",
		Purpose:     routing.PurposeBrowsing,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/routing/select", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.SelectNode(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp routing.RouteResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "us1", resp.PrimaryNode.NodeID)
}

func TestRoutingHandler_SelectNode_BadRequest(t *testing.T) {
	cache := seedTestCache()
	router := routing.NewSmartRouter(cache, nil, nil, nil)
	h := NewRoutingHandler(router)

	req := httptest.NewRequest(http.MethodPost, "/api/routing/select", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	h.SelectNode(w, req)

	// writeValidationError returns 422 for JSON decode failures.
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "COMMON.VALIDATION_ERROR", resp["code"])
}

func TestRoutingHandler_SelectNode_NoNodes(t *testing.T) {
	cache := health.NewNodeHealthCache()
	router := routing.NewSmartRouter(cache, nil, nil, nil)
	h := NewRoutingHandler(router)

	body, _ := json.Marshal(routing.RouteRequest{
		UserCountry: "US",
		Purpose:     routing.PurposeBrowsing,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/routing/select", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.SelectNode(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
