package domain

import (
	"errors"
	"strings"
	"testing"
)

func square() [][]float64 {
	return [][]float64{
		{37.7749, -122.4194},
		{37.7849, -122.4194},
		{37.7849, -122.4094},
		{37.7749, -122.4094},
		{37.7749, -122.4194},
	}
}

func TestGeofenceInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(in *GeofenceInput)
		wantErr string // substring; empty means valid
	}{
		{name: "valid", mutate: func(in *GeofenceInput) {}},
		{name: "missing name", mutate: func(in *GeofenceInput) { in.Name = "   " }, wantErr: "name: is required"},
		{name: "missing category", mutate: func(in *GeofenceInput) { in.Category = "" }, wantErr: "category: is required"},
		{name: "unknown category", mutate: func(in *GeofenceInput) { in.Category = "parking" }, wantErr: "category: must be one of"},
		{name: "no coordinates", mutate: func(in *GeofenceInput) { in.Coordinates = nil }, wantErr: "coordinates: is required"},
		{
			name:    "too few points",
			mutate:  func(in *GeofenceInput) { in.Coordinates = [][]float64{{1, 1}, {2, 2}, {1, 1}} },
			wantErr: "at least 4 points",
		},
		{
			name:    "not closed",
			mutate:  func(in *GeofenceInput) { in.Coordinates[4] = []float64{37.0, -122.0} },
			wantErr: "first and last points must be identical",
		},
		{
			name:    "latitude out of range",
			mutate:  func(in *GeofenceInput) { in.Coordinates[1] = []float64{91, -122.4194} },
			wantErr: "latitude must be between -90 and 90",
		},
		{
			name:    "longitude out of range",
			mutate:  func(in *GeofenceInput) { in.Coordinates[2] = []float64{37.7849, 181} },
			wantErr: "longitude must be between -180 and 180",
		},
		{
			name:    "pair with three numbers",
			mutate:  func(in *GeofenceInput) { in.Coordinates[1] = []float64{37.7849, -122.4194, 5} },
			wantErr: "point 1 must be a [latitude, longitude] pair",
		},
		{
			name: "only two unique points",
			mutate: func(in *GeofenceInput) {
				in.Coordinates = [][]float64{{1, 1}, {2, 2}, {1, 1}, {2, 2}, {1, 1}}
			},
			wantErr: "at least 3 unique points",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := GeofenceInput{Name: " Downtown ", Category: "delivery_zone", Coordinates: square()}
			tt.mutate(&in)
			err := in.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				if in.Name != "Downtown" {
					t.Errorf("name was not trimmed: %q", in.Name)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("expected *ValidationError, got %T", err)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestVehicleInputValidate(t *testing.T) {
	valid := VehicleInput{VehicleNumber: " ka-01-ab-1234 ", DriverName: "John Doe", VehicleType: "Truck", Phone: "+1234567890"}
	in := valid
	if err := in.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	if in.VehicleNumber != "KA-01-AB-1234" || in.VehicleType != "truck" {
		t.Errorf("fields not normalised: %+v", in)
	}

	bad := []struct {
		name    string
		mutate  func(in *VehicleInput)
		wantErr string
	}{
		{"missing number", func(in *VehicleInput) { in.VehicleNumber = "" }, "vehicle_number: is required"},
		{"missing driver", func(in *VehicleInput) { in.DriverName = "" }, "driver_name: is required"},
		{"missing type", func(in *VehicleInput) { in.VehicleType = " " }, "vehicle_type: is required"},
		{"missing phone", func(in *VehicleInput) { in.Phone = "" }, "phone: is required"},
		{"letters in phone", func(in *VehicleInput) { in.Phone = "call me" }, "phone: must be a phone number"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			in := valid
			tt.mutate(&in)
			err := in.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func ptr(f float64) *float64 { return &f }

func TestLocationInputValidate(t *testing.T) {
	ok := LocationInput{VehicleID: "veh_1", Latitude: ptr(0), Longitude: ptr(0), Timestamp: "2025-01-15T10:35:00+05:30"}
	u, err := ok.Validate()
	if err != nil {
		t.Fatalf("latitude/longitude 0 should be valid, got %v", err)
	}
	if got := u.Timestamp.Format("2006-01-02T15:04:05Z07:00"); got != "2025-01-15T05:05:00Z" {
		t.Errorf("timestamp not converted to UTC: %s", got)
	}

	bad := []struct {
		name    string
		in      LocationInput
		wantErr string
	}{
		{"missing vehicle", LocationInput{Latitude: ptr(1), Longitude: ptr(1), Timestamp: "2025-01-15T10:35:00Z"}, "vehicle_id"},
		{"missing latitude", LocationInput{VehicleID: "v", Longitude: ptr(1), Timestamp: "2025-01-15T10:35:00Z"}, "latitude: is required"},
		{"missing longitude", LocationInput{VehicleID: "v", Latitude: ptr(1), Timestamp: "2025-01-15T10:35:00Z"}, "longitude: is required"},
		{"latitude too big", LocationInput{VehicleID: "v", Latitude: ptr(90.1), Longitude: ptr(1), Timestamp: "2025-01-15T10:35:00Z"}, "latitude must be between"},
		{"longitude too small", LocationInput{VehicleID: "v", Latitude: ptr(1), Longitude: ptr(-180.5), Timestamp: "2025-01-15T10:35:00Z"}, "longitude must be between"},
		{"missing timestamp", LocationInput{VehicleID: "v", Latitude: ptr(1), Longitude: ptr(1)}, "timestamp: is required"},
		{"no timezone", LocationInput{VehicleID: "v", Latitude: ptr(1), Longitude: ptr(1), Timestamp: "2025-01-15 10:35"}, "timestamp: must be an ISO 8601"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.in.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestAlertRuleInputValidate(t *testing.T) {
	empty := "  "
	in := AlertRuleInput{GeofenceID: "geo_1", VehicleID: &empty, EventType: "BOTH"}
	if err := in.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	if in.VehicleID != nil {
		t.Error("blank vehicle_id should mean all vehicles (nil)")
	}
	if in.EventType != "both" {
		t.Errorf("event type not normalised: %q", in.EventType)
	}

	in = AlertRuleInput{GeofenceID: "geo_1", EventType: "enter"}
	if err := in.Validate(); err == nil || !strings.Contains(err.Error(), "entry, exit, both") {
		t.Fatalf("expected event_type error, got %v", err)
	}
	in = AlertRuleInput{EventType: "entry"}
	if err := in.Validate(); err == nil || !strings.Contains(err.Error(), "geofence_id") {
		t.Fatalf("expected geofence_id error, got %v", err)
	}
}

func TestRuleEventTypeMatches(t *testing.T) {
	cases := []struct {
		rule  RuleEventType
		event EventType
		want  bool
	}{
		{RuleEntry, EventEntry, true},
		{RuleEntry, EventExit, false},
		{RuleExit, EventExit, true},
		{RuleExit, EventEntry, false},
		{RuleBoth, EventEntry, true},
		{RuleBoth, EventExit, true},
	}
	for _, c := range cases {
		if got := c.rule.Matches(c.event); got != c.want {
			t.Errorf("%s.Matches(%s) = %v, want %v", c.rule, c.event, got, c.want)
		}
	}
}

func TestNewIDHasPrefixAndIsUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		id := NewID(PrefixGeofence)
		if !strings.HasPrefix(id, "geo_") || len(id) != len("geo_")+13 {
			t.Fatalf("unexpected id format %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}
