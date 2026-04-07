package routing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra/health"
)

func TestHaversine(t *testing.T) {
	tests := []struct {
		name                         string
		lat1, lng1, lat2, lng2       float64
		wantKm                       float64
		delta                        float64
	}{
		{
			name:   "Moscow to Berlin",
			lat1:   55.7558, lng1: 37.6173,
			lat2:   52.5200, lng2: 13.4050,
			wantKm: 1610, delta: 50,
		},
		{
			name:   "Moscow to Sydney",
			lat1:   55.7558, lng1: 37.6173,
			lat2:   -33.8688, lng2: 151.2093,
			wantKm: 14500, delta: 200,
		},
		{
			name:   "same point",
			lat1:   40.7128, lng1: -74.0060,
			lat2:   40.7128, lng2: -74.0060,
			wantKm: 0, delta: 0.001,
		},
		{
			name:   "antipodal points",
			lat1:   0, lng1: 0,
			lat2:   0, lng2: 180,
			wantKm: 20015, delta: 100,
		},
		{
			name:   "London to Tokyo",
			lat1:   51.5074, lng1: -0.1278,
			lat2:   35.6762, lng2: 139.6503,
			wantKm: 9560, delta: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist := Haversine(tt.lat1, tt.lng1, tt.lat2, tt.lng2)
			assert.InDelta(t, tt.wantKm, dist, tt.delta)
		})
	}
}

func TestHaversine_Symmetry(t *testing.T) {
	d1 := Haversine(55.7558, 37.6173, 52.5200, 13.4050)
	d2 := Haversine(52.5200, 13.4050, 55.7558, 37.6173)
	assert.InDelta(t, d1, d2, 0.001)
}

func TestGeoScore(t *testing.T) {
	tests := []struct {
		name       string
		distanceKm float64
		want       float64
		delta      float64
	}{
		{"zero distance", 0, 100.0, 0.001},
		{"negative distance", -100, 100.0, 0.001},
		{"half max distance", 10000, 50.0, 1.0},
		{"at max distance", MaxGeoDistanceKm, 0.0, 0.001},
		{"beyond max distance", 30000, 0.0, 0.001},
		{"quarter max distance", 5000, 75.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := GeoScore(tt.distanceKm)
			assert.InDelta(t, tt.want, score, tt.delta)
		})
	}
}

func TestGeoScore_PropertyBased(t *testing.T) {
	for lat := -90.0; lat <= 90; lat += 30 {
		for lng := -180.0; lng <= 180; lng += 60 {
			dist := Haversine(0, 0, lat, lng)
			score := GeoScore(dist)
			assert.GreaterOrEqual(t, score, 0.0, "score must be >= 0 for lat=%.0f lng=%.0f", lat, lng)
			assert.LessOrEqual(t, score, 100.0, "score must be <= 100 for lat=%.0f lng=%.0f", lat, lng)
		}
	}
}

func TestGeoScore_Monotonic(t *testing.T) {
	prev := GeoScore(0)
	for d := 100.0; d <= MaxGeoDistanceKm; d += 100 {
		cur := GeoScore(d)
		assert.LessOrEqual(t, cur, prev, "score must decrease as distance increases (d=%.0f)", d)
		prev = cur
	}
}

func TestLatencyScore(t *testing.T) {
	tests := []struct {
		name      string
		latencyMs float64
		want      float64
		delta     float64
	}{
		{"no measurement", 0, LatencyScoreUnknown, 0.001},
		{"negative measurement", -10, LatencyScoreUnknown, 0.001},
		{"low latency 50ms", 50, 90.0, 0.1},
		{"mid latency 250ms", 250, 50.0, 0.1},
		{"at max latency", MaxLatencyMs, 0.0, 0.001},
		{"beyond max latency", 1000, 0.0, 0.001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := health.NodeHealth{LastLatencyMs: tt.latencyMs}
			score := latencyScore(node)
			assert.InDelta(t, tt.want, score, tt.delta)
		})
	}
}

func TestGeoProximityScore_WithCoordinates(t *testing.T) {
	// Berlin user, Frankfurt node vs Sydney node.
	berlinReq := RouteRequest{
		UserLatitude:  52.5200,
		UserLongitude: 13.4050,
		UserCountry:   "DE",
	}

	frankfurt := health.NodeHealth{
		NodeID:      "de1",
		CountryCode: "DE",
		Latitude:    50.1109,
		Longitude:   8.6821,
	}

	sydney := health.NodeHealth{
		NodeID:      "au1",
		CountryCode: "AU",
		Latitude:    -33.8688,
		Longitude:   151.2093,
	}

	frankfurtScore := geoProximityScore(berlinReq, frankfurt)
	sydneyScore := geoProximityScore(berlinReq, sydney)

	assert.Greater(t, frankfurtScore, sydneyScore, "Frankfurt should score higher than Sydney for a Berlin user")
	assert.Greater(t, frankfurtScore, 90.0, "Frankfurt should be very close to Berlin")
	assert.Less(t, sydneyScore, 30.0, "Sydney should score low for a Berlin user")
}

func TestGeoProximityScore_FallbackToCountryCode(t *testing.T) {
	// No coordinates: falls back to country match.
	req := RouteRequest{UserCountry: "US"}
	sameCountry := health.NodeHealth{CountryCode: "US"}
	diffCountry := health.NodeHealth{CountryCode: "JP"}

	assert.Equal(t, GeoScoreSameCountry, geoProximityScore(req, sameCountry))
	assert.Equal(t, GeoScoreDifferentCountry, geoProximityScore(req, diffCountry))
}

func TestGeoProximityScore_NoCountryNoCoords(t *testing.T) {
	req := RouteRequest{}
	node := health.NodeHealth{CountryCode: "US"}

	assert.Equal(t, GeoScoreDifferentCountry, geoProximityScore(req, node))
}
