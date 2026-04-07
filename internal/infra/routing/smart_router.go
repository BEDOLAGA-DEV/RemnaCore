package routing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/health"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
)

// Scoring weight constants for the default (browsing) purpose.
const (
	WeightGeo     = 0.33
	WeightLatency = 0.34
	WeightLoad    = 0.33
)

// Gaming purpose weights: heavily favour latency.
const (
	WeightGamingGeo     = 0.2
	WeightGamingLatency = 0.6
	WeightGamingLoad    = 0.2
)

// Streaming purpose weights: heavily favour geographic proximity.
const (
	WeightStreamingGeo     = 0.5
	WeightStreamingLatency = 0.2
	WeightStreamingLoad    = 0.3
)

// Scoring bounds and bonuses.
const (
	MaxGeoDistanceKm = 20000.0
	MaxLatencyMs     = 500.0
	ScoreMax         = 100.0
	PremiumTierBonus = 15.0
)

// Geo proximity fallback scores used when coordinates are unavailable.
const (
	// GeoScoreSameCountry is the geo score when user and node share a country.
	GeoScoreSameCountry = 100.0
	// GeoScoreDifferentCountry is the fallback geo score for unknown distance.
	GeoScoreDifferentCountry = 30.0
)

// Latency score returned when no real measurement is available.
const LatencyScoreUnknown = 50.0

// Load scores returned by loadScore().
const (
	// LoadScoreZeroTraffic is the load score when a node has zero traffic.
	LoadScoreZeroTraffic = 100.0
	// LoadScoreHighLoad is the minimum load score for heavily loaded nodes.
	LoadScoreHighLoad = 10.0
)

// ScoreRoundingFactor is used to round composite scores to 2 decimal places.
const ScoreRoundingFactor = 100.0

// Purpose constants identify the intended use of a VPN connection.
const (
	PurposeBrowsing  = "browsing"
	PurposeGaming    = "gaming"
	PurposeStreaming = "streaming"
)

// Subscription tier that qualifies for the premium score bonus.
const (
	TierPremium = "premium"
	TierUltra   = "ultra"
)

// HookRoutingScoreModifier is the plugin hook name dispatched after initial scoring.
const HookRoutingScoreModifier = "routing.score_modifier"

// Maximum fallback nodes returned in a response.
const maxFallbackNodes = 3

// staleCacheThreshold is the maximum age of node health data before the
// SmartRouter logs a warning about stale cache state.
const staleCacheThreshold = 5 * time.Minute

// Errors returned by the SmartRouter.
var (
	ErrNoHealthyNodes = errors.New("no healthy nodes available")
)

// purposeWeights holds the geo/latency/load weight triple for a single purpose.
type purposeWeights struct {
	geo     float64
	latency float64
	load    float64
}

// SmartRouter selects the best VPN node for a user based on a weighted scoring
// algorithm. It reads exclusively from the in-memory NodeHealthCache (no Redis
// hop, no network call).
type SmartRouter struct {
	cache      *health.NodeHealthCache
	dispatcher hookdispatch.Dispatcher
	logger     *slog.Logger

	browsing  purposeWeights
	gaming    purposeWeights
	streaming purposeWeights
}

// NewSmartRouter creates a SmartRouter backed by the shared health cache.
// Weights are read from cfg; when cfg is nil (e.g. in tests) the compiled-in
// constants are used.
func NewSmartRouter(cache *health.NodeHealthCache, dispatcher hookdispatch.Dispatcher, logger *slog.Logger, cfg *config.Config) *SmartRouter {
	r := &SmartRouter{
		cache:      cache,
		dispatcher: dispatcher,
		logger:     logger,
		browsing:   purposeWeights{geo: WeightGeo, latency: WeightLatency, load: WeightLoad},
		gaming:     purposeWeights{geo: WeightGamingGeo, latency: WeightGamingLatency, load: WeightGamingLoad},
		streaming:  purposeWeights{geo: WeightStreamingGeo, latency: WeightStreamingLatency, load: WeightStreamingLoad},
	}

	if cfg != nil {
		sc := cfg.SmartRouter
		if sc.WeightGeo != 0 || sc.WeightLatency != 0 || sc.WeightLoad != 0 {
			r.browsing = purposeWeights{geo: sc.WeightGeo, latency: sc.WeightLatency, load: sc.WeightLoad}
		}
		if sc.WeightGamingGeo != 0 || sc.WeightGamingLatency != 0 || sc.WeightGamingLoad != 0 {
			r.gaming = purposeWeights{geo: sc.WeightGamingGeo, latency: sc.WeightGamingLatency, load: sc.WeightGamingLoad}
		}
		if sc.WeightStreamingGeo != 0 || sc.WeightStreamingLatency != 0 || sc.WeightStreamingLoad != 0 {
			r.streaming = purposeWeights{geo: sc.WeightStreamingGeo, latency: sc.WeightStreamingLatency, load: sc.WeightStreamingLoad}
		}
	}

	return r
}

// RouteRequest describes the parameters used for node selection.
type RouteRequest struct {
	UserCountry      string   `json:"user_country"`
	UserLatitude     float64  `json:"user_latitude"`
	UserLongitude    float64  `json:"user_longitude"`
	Protocol         string   `json:"protocol"`
	Purpose          string   `json:"purpose"`
	AllowedNodes     []string `json:"allowed_nodes"`
	SubscriptionTier string   `json:"subscription_tier"`
}

// RouteResponse contains the recommended primary node and ordered fallbacks.
type RouteResponse struct {
	PrimaryNode   NodeScore   `json:"primary_node"`
	FallbackNodes []NodeScore `json:"fallback_nodes"`
}

// NodeScore holds the final score and metadata for a candidate node.
type NodeScore struct {
	NodeID  string  `json:"node_id"`
	Name    string  `json:"name"`
	Country string  `json:"country"`
	Score   float64 `json:"score"`
	Reason  string  `json:"reason"`
}

// SelectNode evaluates healthy nodes against the request parameters and returns
// the best match plus fallback alternatives.
func (r *SmartRouter) SelectNode(ctx context.Context, req RouteRequest) (*RouteResponse, error) {
	if !r.cache.IsFresh(staleCacheThreshold) {
		if r.logger != nil {
			r.logger.Warn("node health cache is stale",
				slog.Duration("stale_for", r.cache.StaleDuration()))
		}
	}

	healthy := r.cache.GetHealthy()
	if len(healthy) == 0 {
		return nil, ErrNoHealthyNodes
	}

	// Build allowed-node lookup for O(1) filtering.
	allowedSet := make(map[string]struct{}, len(req.AllowedNodes))
	for _, id := range req.AllowedNodes {
		allowedSet[id] = struct{}{}
	}

	wGeo, wLatency, wLoad := r.weightsForPurpose(req.Purpose)

	scored := make([]NodeScore, 0, len(healthy))
	for _, node := range healthy {
		// Filter by allowed nodes when specified.
		if len(allowedSet) > 0 {
			if _, ok := allowedSet[node.NodeID]; !ok {
				continue
			}
		}

		geoScore := geoProximityScore(req, node)
		latScore := latencyScore(node)
		loadScr := loadScore(node.TrafficUsed)

		composite := wGeo*geoScore + wLatency*latScore + wLoad*loadScr

		// Premium tier bonus.
		if req.SubscriptionTier == TierPremium || req.SubscriptionTier == TierUltra {
			composite += PremiumTierBonus
		}

		// Clamp to ScoreMax.
		if composite > ScoreMax {
			composite = ScoreMax
		}

		scored = append(scored, NodeScore{
			NodeID:  node.NodeID,
			Name:    node.Name,
			Country: node.CountryCode,
			Score:   math.Round(composite*ScoreRoundingFactor) / ScoreRoundingFactor,
			Reason:  fmt.Sprintf("geo=%.1f lat=%.1f load=%.1f", geoScore, latScore, loadScr),
		})
	}

	if len(scored) == 0 {
		return nil, ErrNoHealthyNodes
	}

	// Dispatch routing.score_modifier plugin hook (best-effort).
	scored = r.applyPluginModifiers(ctx, req, scored)

	// Sort descending by score.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	resp := &RouteResponse{
		PrimaryNode: scored[0],
	}

	fallbackEnd := min(len(scored), maxFallbackNodes+1)
	if fallbackEnd > 1 {
		resp.FallbackNodes = scored[1:fallbackEnd]
	}

	return resp, nil
}

// weightsForPurpose returns the geo/latency/load weight triple for the given purpose.
func (r *SmartRouter) weightsForPurpose(purpose string) (geo, latency, load float64) {
	switch purpose {
	case PurposeGaming:
		return r.gaming.geo, r.gaming.latency, r.gaming.load
	case PurposeStreaming:
		return r.streaming.geo, r.streaming.latency, r.streaming.load
	default:
		return r.browsing.geo, r.browsing.latency, r.browsing.load
	}
}

// geoProximityScore returns a 0-100 score. When both user and node have
// coordinates it uses Haversine great-circle distance for a continuous score.
// Otherwise it falls back to binary country-code matching.
func geoProximityScore(req RouteRequest, node health.NodeHealth) float64 {
	if req.UserLatitude != 0 && node.Latitude != 0 {
		dist := Haversine(req.UserLatitude, req.UserLongitude, node.Latitude, node.Longitude)
		return GeoScore(dist)
	}
	if req.UserCountry != "" && node.CountryCode == req.UserCountry {
		return GeoScoreSameCountry
	}
	return GeoScoreDifferentCountry
}

// latencyScore returns a 0-100 score based on real latency measurements.
// When no measurement is available it returns LatencyScoreUnknown.
func latencyScore(node health.NodeHealth) float64 {
	if node.LastLatencyMs <= 0 {
		return LatencyScoreUnknown
	}
	if node.LastLatencyMs >= MaxLatencyMs {
		return 0
	}
	return 100 * (1 - node.LastLatencyMs/MaxLatencyMs)
}

// loadScore returns a 0-100 score inversely proportional to traffic load.
// Lower traffic -> higher score.
func loadScore(trafficUsedBytes int64) float64 {
	const highLoadThreshold int64 = 100 * 1 << 30 // 100 GB
	if trafficUsedBytes <= 0 {
		return LoadScoreZeroTraffic
	}
	if trafficUsedBytes >= highLoadThreshold {
		return LoadScoreHighLoad
	}
	ratio := float64(trafficUsedBytes) / float64(highLoadThreshold)
	return LoadScoreZeroTraffic * (1.0 - ratio)
}

// scoreModifierPayload is the JSON payload sent to the routing.score_modifier hook.
type scoreModifierPayload struct {
	Request RouteRequest `json:"request"`
	Scores  []NodeScore  `json:"scores"`
}

// applyPluginModifiers dispatches the routing.score_modifier sync hook and
// applies any score modifications returned by plugins.
func (r *SmartRouter) applyPluginModifiers(ctx context.Context, req RouteRequest, scores []NodeScore) []NodeScore {
	if r.dispatcher == nil {
		return scores
	}

	payload := scoreModifierPayload{
		Request: req,
		Scores:  scores,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		r.logger.Error("failed to marshal score modifier payload", slog.Any("error", err))
		return scores
	}

	modified, err := r.dispatcher.DispatchSync(ctx, HookRoutingScoreModifier, data)
	if err != nil {
		r.logger.Warn("score modifier hook failed, using original scores", slog.Any("error", err))
		return scores
	}

	var result scoreModifierPayload
	if err := json.Unmarshal(modified, &result); err != nil {
		r.logger.Warn("failed to unmarshal modified scores, using originals", slog.Any("error", err))
		return scores
	}

	if len(result.Scores) > 0 {
		return result.Scores
	}

	return scores
}
