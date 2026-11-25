package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestRingToGeoJSONSwapsToLngLat(t *testing.T) {
	ring := [][]float64{{10, 20}, {11, 20}, {11, 21}, {10, 20}}
	got, err := RingToGeoJSON(ring)
	if err != nil {
		t.Fatal(err)
	}
	// GeoJSON is [longitude, latitude], so [10 (lat), 20 (lng)] becomes [20,10].
	if !strings.HasPrefix(got, `{"type":"Polygon","coordinates":[[[20,10],[20,11],[21,11],[20,10]]]}`) {
		t.Errorf("unexpected GeoJSON: %s", got)
	}
}

func TestGeoJSONRoundTrip(t *testing.T) {
	ring := square()
	geoJSON, err := RingToGeoJSON(ring)
	if err != nil {
		t.Fatal(err)
	}
	back, err := RingFromGeoJSON(geoJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, ring) {
		t.Errorf("round trip changed the ring:\n got %v\nwant %v", back, ring)
	}
}

func TestRingFromGeoJSONRejectsOtherTypes(t *testing.T) {
	if _, err := RingFromGeoJSON(`{"type":"Point","coordinates":[1,2]}`); err == nil {
		t.Error("expected an error for a Point")
	}
}
