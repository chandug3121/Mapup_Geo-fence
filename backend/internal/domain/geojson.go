package domain

import (
	"encoding/json"
	"fmt"
)

// The API speaks [latitude, longitude] but GeoJSON and PostGIS use
// [longitude, latitude] (x, y). These two helpers are the only place the
// order is swapped, so the rest of the code never has to think about it.

type geoJSONPolygon struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

// RingToGeoJSON converts a [lat, lng] ring into a GeoJSON Polygon string.
func RingToGeoJSON(ring [][]float64) (string, error) {
	out := make([][2]float64, len(ring))
	for i, p := range ring {
		if len(p) != 2 {
			return "", fmt.Errorf("point %d is not a [lat, lng] pair", i)
		}
		out[i] = [2]float64{p[1], p[0]}
	}
	b, err := json.Marshal(geoJSONPolygon{Type: "Polygon", Coordinates: [][][2]float64{out}})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// RingFromGeoJSON reads the outer ring of a GeoJSON Polygon and returns it
// as [lat, lng] pairs.
func RingFromGeoJSON(raw string) ([][]float64, error) {
	var poly geoJSONPolygon
	if err := json.Unmarshal([]byte(raw), &poly); err != nil {
		return nil, fmt.Errorf("decode polygon: %w", err)
	}
	if poly.Type != "Polygon" || len(poly.Coordinates) == 0 {
		return nil, fmt.Errorf("expected a Polygon with at least one ring, got %q", poly.Type)
	}
	outer := poly.Coordinates[0]
	ring := make([][]float64, len(outer))
	for i, p := range outer {
		ring[i] = []float64{p[1], p[0]}
	}
	return ring, nil
}
