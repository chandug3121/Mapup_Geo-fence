package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxNameLen        = 200
	maxDescriptionLen = 2000
	minRingPoints     = 4 // 3 unique points + the closing point
	maxRingPoints     = 10000
)

// GeofenceInput is the body of POST /geofences.
type GeofenceInput struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
}

// Validate trims text fields and checks every rule from the spec:
// closed ring, at least 4 points (3 unique), and lat/lng within range.
func (in *GeofenceInput) Validate() error {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)

	if in.Name == "" {
		return Invalid("name", "is required")
	}
	if utf8.RuneCountInString(in.Name) > maxNameLen {
		return Invalid("name", "must be at most %d characters", maxNameLen)
	}
	if utf8.RuneCountInString(in.Description) > maxDescriptionLen {
		return Invalid("description", "must be at most %d characters", maxDescriptionLen)
	}
	if in.Category == "" {
		return Invalid("category", "is required")
	}
	if !Category(in.Category).IsValid() {
		return Invalid("category", "must be one of delivery_zone, restricted_zone, toll_zone, customer_area")
	}
	return ValidateRing(in.Coordinates)
}

// ValidateRing checks a polygon ring given as [lat, lng] pairs.
func ValidateRing(ring [][]float64) error {
	if len(ring) == 0 {
		return Invalid("coordinates", "is required")
	}
	if len(ring) < minRingPoints {
		return Invalid("coordinates", "needs at least %d points (3 unique points plus the closing point), got %d", minRingPoints, len(ring))
	}
	if len(ring) > maxRingPoints {
		return Invalid("coordinates", "can have at most %d points", maxRingPoints)
	}
	for i, pair := range ring {
		if len(pair) != 2 {
			return Invalid("coordinates", "point %d must be a [latitude, longitude] pair", i)
		}
		if err := checkLatLng(pair[0], pair[1]); err != nil {
			return Invalid("coordinates", "point %d: %s", i, err.(*ValidationError).Message)
		}
	}
	first, last := ring[0], ring[len(ring)-1]
	if first[0] != last[0] || first[1] != last[1] {
		return Invalid("coordinates", "first and last points must be identical to close the polygon")
	}
	if countUnique(ring[:len(ring)-1]) < 3 {
		return Invalid("coordinates", "needs at least 3 unique points")
	}
	return nil
}

func countUnique(points [][]float64) int {
	seen := make(map[[2]float64]struct{}, len(points))
	for _, p := range points {
		seen[[2]float64{p[0], p[1]}] = struct{}{}
	}
	return len(seen)
}

func checkLatLng(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return &ValidationError{Field: "latitude", Message: "latitude must be between -90 and 90"}
	}
	if lng < -180 || lng > 180 {
		return &ValidationError{Field: "longitude", Message: "longitude must be between -180 and 180"}
	}
	return nil
}

// VehicleInput is the body of POST /vehicles.
type VehicleInput struct {
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
}

var phonePattern = regexp.MustCompile(`^\+?[0-9(][0-9 ()\-]{5,19}$`)

// Validate trims fields and checks they are present and well formed.
func (in *VehicleInput) Validate() error {
	in.VehicleNumber = strings.ToUpper(strings.TrimSpace(in.VehicleNumber))
	in.DriverName = strings.TrimSpace(in.DriverName)
	in.VehicleType = strings.ToLower(strings.TrimSpace(in.VehicleType))
	in.Phone = strings.TrimSpace(in.Phone)

	required := []struct{ field, value string }{
		{"vehicle_number", in.VehicleNumber},
		{"driver_name", in.DriverName},
		{"vehicle_type", in.VehicleType},
		{"phone", in.Phone},
	}
	for _, r := range required {
		if r.value == "" {
			return Invalid(r.field, "is required")
		}
		if utf8.RuneCountInString(r.value) > 100 {
			return Invalid(r.field, "must be at most 100 characters")
		}
	}
	if !phonePattern.MatchString(in.Phone) {
		return Invalid("phone", "must be a phone number like +1234567890")
	}
	return nil
}

// LocationInput is the body of POST /vehicles/location. Latitude and
// longitude are pointers so a missing field can be told apart from 0.
type LocationInput struct {
	VehicleID string   `json:"vehicle_id"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Timestamp string   `json:"timestamp"`
}

// Validate checks the input and converts it into a LocationUpdate.
func (in LocationInput) Validate() (LocationUpdate, error) {
	vehicleID := strings.TrimSpace(in.VehicleID)
	if vehicleID == "" {
		return LocationUpdate{}, Invalid("vehicle_id", "is required")
	}
	if in.Latitude == nil {
		return LocationUpdate{}, Invalid("latitude", "is required")
	}
	if in.Longitude == nil {
		return LocationUpdate{}, Invalid("longitude", "is required")
	}
	if err := checkLatLng(*in.Latitude, *in.Longitude); err != nil {
		return LocationUpdate{}, err
	}
	if strings.TrimSpace(in.Timestamp) == "" {
		return LocationUpdate{}, Invalid("timestamp", "is required")
	}
	ts, err := ParseTimestamp(in.Timestamp)
	if err != nil {
		return LocationUpdate{}, Invalid("timestamp", "must be an ISO 8601 timestamp with a timezone, e.g. 2025-01-15T10:35:00Z")
	}
	return LocationUpdate{
		VehicleID: vehicleID,
		Position:  Position{Latitude: *in.Latitude, Longitude: *in.Longitude, Timestamp: ts},
	}, nil
}

// ParseTimestamp accepts RFC 3339 (the common ISO 8601 profile) and returns UTC.
func ParseTimestamp(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

// AlertRuleInput is the body of POST /alerts/configure.
type AlertRuleInput struct {
	GeofenceID string  `json:"geofence_id"`
	VehicleID  *string `json:"vehicle_id"`
	EventType  string  `json:"event_type"`
}

// Validate trims fields and treats an empty vehicle_id as "all vehicles".
func (in *AlertRuleInput) Validate() error {
	in.GeofenceID = strings.TrimSpace(in.GeofenceID)
	in.EventType = strings.ToLower(strings.TrimSpace(in.EventType))
	if in.VehicleID != nil {
		trimmed := strings.TrimSpace(*in.VehicleID)
		if trimmed == "" {
			in.VehicleID = nil
		} else {
			in.VehicleID = &trimmed
		}
	}
	if in.GeofenceID == "" {
		return Invalid("geofence_id", "is required")
	}
	if in.EventType == "" {
		return Invalid("event_type", "is required")
	}
	if !RuleEventType(in.EventType).IsValid() {
		return Invalid("event_type", "must be one of entry, exit, both")
	}
	return nil
}

// AlertStatusInput is the body of PATCH /alerts/{alert_id}.
type AlertStatusInput struct {
	Status string `json:"status"`
}

// Validate checks the status is active or inactive.
func (in *AlertStatusInput) Validate() error {
	in.Status = strings.ToLower(strings.TrimSpace(in.Status))
	if in.Status != StatusActive && in.Status != StatusInactive {
		return Invalid("status", "must be active or inactive")
	}
	return nil
}
