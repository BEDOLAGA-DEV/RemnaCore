// Package routing provides the SmartRouter which selects optimal VPN nodes
// based on a weighted scoring algorithm considering geographic proximity,
// latency, and load.
package routing

import "math"

// EarthRadiusKm is the Earth's mean radius in kilometres.
const EarthRadiusKm = 6371.0

// Haversine returns the great-circle distance in kilometres between two points
// specified in decimal degrees.
func Haversine(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := degreesToRadians(lat2 - lat1)
	dLng := degreesToRadians(lng2 - lng1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(degreesToRadians(lat1))*math.Cos(degreesToRadians(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return EarthRadiusKm * c
}

// GeoScore returns a 0-100 score inversely proportional to distance.
// 0 km yields 100, MaxGeoDistanceKm or beyond yields 0.
func GeoScore(distanceKm float64) float64 {
	if distanceKm <= 0 {
		return 100
	}
	if distanceKm >= MaxGeoDistanceKm {
		return 0
	}
	return 100 * (1 - distanceKm/MaxGeoDistanceKm)
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
