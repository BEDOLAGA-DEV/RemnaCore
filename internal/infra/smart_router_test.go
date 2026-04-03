package infra

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedCache(cache *NodeHealthCache, nodes []NodeHealth) {
	cache.Update(nodes)
}

func defaultNodes() []NodeHealth {
	return []NodeHealth{
		{NodeID: "us1", Name: "US-East-01", IsOnline: true, CountryCode: "US", TrafficUsed: 0},
		{NodeID: "de1", Name: "DE-Frankfurt-01", IsOnline: true, CountryCode: "DE", TrafficUsed: 10 << 30},
		{NodeID: "jp1", Name: "JP-Tokyo-01", IsOnline: true, CountryCode: "JP", TrafficUsed: 50 << 30},
	}
}

func TestSelectNode_Browsing(t *testing.T) {
	cache := NewNodeHealthCache()
	seedCache(cache, defaultNodes())

	router := NewSmartRouter(cache, nil, nil)

	resp, err := router.SelectNode(context.Background(), RouteRequest{
		UserCountry: "US",
		Purpose:     PurposeBrowsing,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// US node should rank highest for a US user with browsing purpose.
	assert.Equal(t, "us1", resp.PrimaryNode.NodeID)
	assert.NotEmpty(t, resp.PrimaryNode.Reason)
	assert.Greater(t, resp.PrimaryNode.Score, 0.0)
}

func TestSelectNode_Gaming(t *testing.T) {
	cache := NewNodeHealthCache()
	seedCache(cache, defaultNodes())

	router := NewSmartRouter(cache, nil, nil)

	resp, err := router.SelectNode(context.Background(), RouteRequest{
		UserCountry: "US",
		Purpose:     PurposeGaming,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Gaming prioritises latency; US node should still win for US user.
	assert.Equal(t, "us1", resp.PrimaryNode.NodeID)
}

func TestSelectNode_Streaming(t *testing.T) {
	cache := NewNodeHealthCache()
	seedCache(cache, defaultNodes())

	router := NewSmartRouter(cache, nil, nil)

	resp, err := router.SelectNode(context.Background(), RouteRequest{
		UserCountry: "DE",
		Purpose:     PurposeStreaming,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Streaming prioritises geo; DE node should rank highest for DE user.
	assert.Equal(t, "de1", resp.PrimaryNode.NodeID)
}

func TestSelectNode_NoNodes(t *testing.T) {
	cache := NewNodeHealthCache()

	router := NewSmartRouter(cache, nil, nil)

	resp, err := router.SelectNode(context.Background(), RouteRequest{
		UserCountry: "US",
		Purpose:     PurposeBrowsing,
	})
	assert.ErrorIs(t, err, ErrNoHealthyNodes)
	assert.Nil(t, resp)
}

func TestSelectNode_PremiumBonus(t *testing.T) {
	cache := NewNodeHealthCache()
	seedCache(cache, []NodeHealth{
		{NodeID: "us1", Name: "US-East-01", IsOnline: true, CountryCode: "US", TrafficUsed: 0},
	})

	router := NewSmartRouter(cache, nil, nil)

	// Without premium
	basicResp, err := router.SelectNode(context.Background(), RouteRequest{
		UserCountry:      "US",
		Purpose:          PurposeBrowsing,
		SubscriptionTier: "basic",
	})
	require.NoError(t, err)

	// With premium
	premiumResp, err := router.SelectNode(context.Background(), RouteRequest{
		UserCountry:      "US",
		Purpose:          PurposeBrowsing,
		SubscriptionTier: TierPremium,
	})
	require.NoError(t, err)

	// Premium should have a higher or equal score (clamped at ScoreMax).
	assert.GreaterOrEqual(t, premiumResp.PrimaryNode.Score, basicResp.PrimaryNode.Score)
}

func TestSelectNode_AllowedNodesFilter(t *testing.T) {
	cache := NewNodeHealthCache()
	seedCache(cache, defaultNodes())

	router := NewSmartRouter(cache, nil, nil)

	resp, err := router.SelectNode(context.Background(), RouteRequest{
		UserCountry:  "US",
		Purpose:      PurposeBrowsing,
		AllowedNodes: []string{"de1"},
	})
	require.NoError(t, err)

	// Only the DE node is allowed.
	assert.Equal(t, "de1", resp.PrimaryNode.NodeID)
	assert.Empty(t, resp.FallbackNodes)
}

func TestSelectNode_AllowedNodesFilter_NoneMatch(t *testing.T) {
	cache := NewNodeHealthCache()
	seedCache(cache, defaultNodes())

	router := NewSmartRouter(cache, nil, nil)

	resp, err := router.SelectNode(context.Background(), RouteRequest{
		UserCountry:  "US",
		Purpose:      PurposeBrowsing,
		AllowedNodes: []string{"nonexistent"},
	})
	assert.ErrorIs(t, err, ErrNoHealthyNodes)
	assert.Nil(t, resp)
}

func TestSelectNode_FallbackNodes(t *testing.T) {
	cache := NewNodeHealthCache()
	seedCache(cache, defaultNodes())

	router := NewSmartRouter(cache, nil, nil)

	resp, err := router.SelectNode(context.Background(), RouteRequest{
		UserCountry: "US",
		Purpose:     PurposeBrowsing,
	})
	require.NoError(t, err)
	assert.Len(t, resp.FallbackNodes, 2) // 3 total nodes - 1 primary = 2 fallbacks
}

func TestWeightsForPurpose(t *testing.T) {
	tests := []struct {
		purpose              string
		wantGeo, wantLat, wantLoad float64
	}{
		{PurposeBrowsing, WeightGeo, WeightLatency, WeightLoad},
		{PurposeGaming, WeightGamingGeo, WeightGamingLatency, WeightGamingLoad},
		{PurposeStreaming, WeightStreamingGeo, WeightStreamingLatency, WeightStreamingLoad},
		{"unknown", WeightGeo, WeightLatency, WeightLoad},
	}

	for _, tt := range tests {
		t.Run(tt.purpose, func(t *testing.T) {
			g, l, ld := weightsForPurpose(tt.purpose)
			assert.InDelta(t, tt.wantGeo, g, 0.001)
			assert.InDelta(t, tt.wantLat, l, 0.001)
			assert.InDelta(t, tt.wantLoad, ld, 0.001)
		})
	}
}

func TestLoadScore(t *testing.T) {
	assert.InDelta(t, 100.0, loadScore(0), 0.01)
	assert.InDelta(t, 10.0, loadScore(100<<30), 0.01)
	assert.InDelta(t, 100.0, loadScore(-1), 0.01)

	mid := loadScore(50 << 30)
	assert.Greater(t, mid, 10.0)
	assert.Less(t, mid, 100.0)
}
