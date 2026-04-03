package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra"
)

func seedTestCache() *infra.NodeHealthCache {
	cache := infra.NewNodeHealthCache()
	cache.Update([]infra.NodeHealth{
		{NodeID: "us1", Name: "US-East-01", IsOnline: true, CountryCode: "US", TrafficUsed: 0},
		{NodeID: "de1", Name: "DE-Frankfurt-01", IsOnline: true, CountryCode: "DE", TrafficUsed: 10 << 30},
	})
	return cache
}

func TestRoutingHandler_SelectNode_OK(t *testing.T) {
	cache := seedTestCache()
	router := infra.NewSmartRouter(cache, nil, nil)
	h := NewRoutingHandler(router)

	body, _ := json.Marshal(infra.RouteRequest{
		UserCountry: "US",
		Purpose:     infra.PurposeBrowsing,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/routing/select", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.SelectNode(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp infra.RouteResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "us1", resp.PrimaryNode.NodeID)
}

func TestRoutingHandler_SelectNode_BadRequest(t *testing.T) {
	cache := seedTestCache()
	router := infra.NewSmartRouter(cache, nil, nil)
	h := NewRoutingHandler(router)

	req := httptest.NewRequest(http.MethodPost, "/api/routing/select", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	h.SelectNode(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRoutingHandler_SelectNode_NoNodes(t *testing.T) {
	cache := infra.NewNodeHealthCache()
	router := infra.NewSmartRouter(cache, nil, nil)
	h := NewRoutingHandler(router)

	body, _ := json.Marshal(infra.RouteRequest{
		UserCountry: "US",
		Purpose:     infra.PurposeBrowsing,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/routing/select", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.SelectNode(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
