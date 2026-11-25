// Package domain holds the core types, validation rules and errors shared by
// the HTTP layer, the tracking service and the store. It has no dependencies
// outside the standard library so it is easy to unit-test.
package domain

import "time"

// Category is the kind of geofence.
type Category string

const (
	CategoryDelivery   Category = "delivery_zone"
	CategoryRestricted Category = "restricted_zone"
	CategoryToll       Category = "toll_zone"
	CategoryCustomer   Category = "customer_area"
)

// Categories lists every valid category in display order.
var Categories = []Category{CategoryDelivery, CategoryRestricted, CategoryToll, CategoryCustomer}

// IsValid reports whether c is one of the known categories.
func (c Category) IsValid() bool {
	for _, known := range Categories {
		if c == known {
			return true
		}
	}
	return false
}

// EventType is what happened to a vehicle relative to a geofence.
type EventType string

const (
	EventEntry EventType = "entry"
	EventExit  EventType = "exit"
)

// RuleEventType is which events an alert rule listens for.
type RuleEventType string

const (
	RuleEntry RuleEventType = "entry"
	RuleExit  RuleEventType = "exit"
	RuleBoth  RuleEventType = "both"
)

// IsValid reports whether t is entry, exit or both.
func (t RuleEventType) IsValid() bool {
	return t == RuleEntry || t == RuleExit || t == RuleBoth
}

// Matches reports whether a rule listening for t should fire for event e.
func (t RuleEventType) Matches(e EventType) bool {
	return t == RuleBoth || string(t) == string(e)
}

// Status values shared by geofences, vehicles and alert rules.
const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

// Geofence is a named polygon. Coordinates are [lat, lng] pairs forming a
// closed ring (first point == last point).
type Geofence struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    Category    `json:"category"`
	Status      string      `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
}

// FenceRef is the minimal view of a geofence used during tracking.
type FenceRef struct {
	ID       string
	Name     string
	Category Category
}

// Vehicle is a tracked vehicle and its driver.
type Vehicle struct {
	ID            string    `json:"id"`
	VehicleNumber string    `json:"vehicle_number"`
	DriverName    string    `json:"driver_name"`
	VehicleType   string    `json:"vehicle_type"`
	Phone         string    `json:"phone"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// Position is a point in time and space.
type Position struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Timestamp time.Time `json:"timestamp"`
}

// LocationUpdate is a validated location report for a vehicle.
type LocationUpdate struct {
	VehicleID string
	Position
}

// GeofenceEvent is one entry into or exit from a geofence.
type GeofenceEvent struct {
	ID         string
	VehicleID  string
	GeofenceID string
	Type       EventType
	Latitude   float64
	Longitude  float64
	OccurredAt time.Time
}

// AlertRule decides which geofence events produce real-time alerts.
// A nil VehicleID means the rule applies to every vehicle.
type AlertRule struct {
	ID            string        `json:"alert_id"`
	GeofenceID    string        `json:"geofence_id"`
	GeofenceName  string        `json:"geofence_name"`
	VehicleID     *string       `json:"vehicle_id"`
	VehicleNumber *string       `json:"vehicle_number"`
	EventType     RuleEventType `json:"event_type"`
	Status        string        `json:"status"`
	CreatedAt     time.Time     `json:"created_at"`
}

// Violation is a stored geofence event joined with readable names, as
// returned by GET /violations/history.
type Violation struct {
	ID            string    `json:"id"`
	VehicleID     string    `json:"vehicle_id"`
	VehicleNumber string    `json:"vehicle_number"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name"`
	Category      Category  `json:"category"`
	EventType     EventType `json:"event_type"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Timestamp     time.Time `json:"timestamp"`
}

// AlertMessage is the exact JSON pushed to WebSocket clients.
type AlertMessage struct {
	EventID   string        `json:"event_id"`
	EventType EventType     `json:"event_type"`
	Timestamp time.Time     `json:"timestamp"`
	Vehicle   AlertVehicle  `json:"vehicle"`
	Geofence  AlertGeofence `json:"geofence"`
	Location  AlertLocation `json:"location"`
}

type AlertVehicle struct {
	VehicleID     string `json:"vehicle_id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
}

type AlertGeofence struct {
	GeofenceID   string   `json:"geofence_id"`
	GeofenceName string   `json:"geofence_name"`
	Category     Category `json:"category"`
}

type AlertLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
