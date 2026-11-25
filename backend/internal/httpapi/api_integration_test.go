package httpapi

// End-to-end tests against a real PostGIS database. They run when
// TEST_DATABASE_URL is set (CI sets it) and are skipped otherwise:
//
//	TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/geofence_test?sslmode=disable go test ./...

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/config"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/db"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/realtime"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/store"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/tracking"
	"github.com/chandug3121/mapup_geo-fence/backend/migrations"
)

type testServer struct {
	t   *testing.T
	url string
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database tests")
	}
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	pool, err := db.Connect(ctx, dsn, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool, migrations.Files, log); err != nil {
		t.Fatal(err)
	}
	resetDatabase(t, pool)

	hubCtx, stopHub := context.WithCancel(ctx)
	hub := realtime.NewHub(log)
	go hub.Run(hubCtx)

	st := store.New(pool)
	cfg := config.Config{CORSOrigins: []string{"*"}, LocationRateLimit: 10000}
	srv := httptest.NewServer(NewRouter(cfg, st, tracking.NewService(st, hub, log), hub, log))
	t.Cleanup(func() {
		srv.Close()
		stopHub()
		pool.Close()
	})
	return &testServer{t: t, url: srv.URL}
}

func resetDatabase(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `TRUNCATE alert_notifications, geofence_events, alert_rules,
		vehicle_geofence_state, vehicle_locations, vehicles, geofences`)
	if err != nil {
		t.Fatal(err)
	}
}

// do sends a request and returns the status and decoded JSON body. Every
// response is also checked for the mandatory time_ns field.
func (s *testServer) do(method, path string, body any) (int, map[string]any) {
	s.t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, s.url+path, reader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		s.t.Fatalf("%s %s: response is not JSON: %v", method, path, err)
	}
	if _, ok := out["time_ns"].(string); !ok {
		s.t.Fatalf("%s %s: missing time_ns string in %v", method, path, out)
	}
	return resp.StatusCode, out
}

func (s *testServer) mustStatus(want int, method, path string, body any) map[string]any {
	s.t.Helper()
	got, out := s.do(method, path, body)
	if got != want {
		s.t.Fatalf("%s %s: status %d, want %d: %v", method, path, got, want, out)
	}
	return out
}

// The spec's example polygon (around San Francisco).
var downtown = [][]float64{
	{37.7749, -122.4194},
	{37.7849, -122.4194},
	{37.7849, -122.4094},
	{37.7749, -122.4094},
	{37.7749, -122.4194},
}

func TestFullFlow(t *testing.T) {
	s := newTestServer(t)

	geo := s.mustStatus(201, "POST", "/geofences", map[string]any{
		"name": "Downtown Delivery Zone", "description": "Main area",
		"coordinates": downtown, "category": "restricted_zone",
	})
	geoID := geo["id"].(string)
	if !strings.HasPrefix(geoID, "geo_") || geo["status"] != "active" {
		t.Fatalf("unexpected create response: %v", geo)
	}

	veh := s.mustStatus(201, "POST", "/vehicles", map[string]any{
		"vehicle_number": "KA-01-AB-1234", "driver_name": "John Doe", "vehicle_type": "truck", "phone": "+1234567890",
	})
	vehID := veh["id"].(string)

	// Duplicate vehicle number is a conflict.
	s.mustStatus(409, "POST", "/vehicles", map[string]any{
		"vehicle_number": "ka-01-ab-1234", "driver_name": "Jane", "vehicle_type": "car", "phone": "+1987654321",
	})

	rule := s.mustStatus(201, "POST", "/alerts/configure", map[string]any{
		"geofence_id": geoID, "event_type": "both",
	})
	if rule["vehicle_id"] != nil {
		t.Errorf("rule without vehicle_id should apply to all vehicles, got %v", rule["vehicle_id"])
	}
	s.mustStatus(409, "POST", "/alerts/configure", map[string]any{"geofence_id": geoID, "event_type": "both"})
	s.mustStatus(404, "POST", "/alerts/configure", map[string]any{"geofence_id": "geo_missing", "event_type": "entry"})

	// Connect a WebSocket client before moving the vehicle.
	wsURL := "ws" + strings.TrimPrefix(s.url, "http") + "/ws/alerts"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	waitHealthClients(t, s, 1)

	// Outside the fence: no events.
	out := s.mustStatus(200, "POST", "/vehicles/location", location(vehID, 37.70, -122.50, "2025-01-15T10:30:00Z"))
	if n := len(out["current_geofences"].([]any)); n != 0 {
		t.Fatalf("expected no geofences, got %d", n)
	}

	// The spec's sample point is exactly a corner of the polygon; it must count as inside.
	out = s.mustStatus(200, "POST", "/vehicles/location", location(vehID, 37.7849, -122.4194, "2025-01-15T10:35:00Z"))
	fences := out["current_geofences"].([]any)
	if len(fences) != 1 || fences[0].(map[string]any)["status"] != "inside" {
		t.Fatalf("vertex point should be inside, got %v", out)
	}
	alert := readAlert(t, ws)
	if alert["event_type"] != "entry" {
		t.Fatalf("expected entry alert, got %v", alert)
	}
	if alert["vehicle"].(map[string]any)["vehicle_number"] != "KA-01-AB-1234" ||
		alert["geofence"].(map[string]any)["category"] != "restricted_zone" {
		t.Errorf("alert payload incomplete: %v", alert)
	}

	// Still inside: no new alert.
	s.mustStatus(200, "POST", "/vehicles/location", location(vehID, 37.78, -122.415, "2025-01-15T10:36:00Z"))

	// A late update from before the entry must not change state.
	out = s.mustStatus(200, "POST", "/vehicles/location", location(vehID, 37.70, -122.50, "2025-01-15T10:00:00Z"))
	if out["stale"] != true || len(out["current_geofences"].([]any)) != 1 {
		t.Fatalf("stale update changed state: %v", out)
	}

	// Leave: exit alert.
	s.mustStatus(200, "POST", "/vehicles/location", location(vehID, 37.79, -122.40, "2025-01-15T10:40:00Z"))
	if alert := readAlert(t, ws); alert["event_type"] != "exit" {
		t.Fatalf("expected exit alert, got %v", alert)
	}

	// Current location reflects the newest update.
	loc := s.mustStatus(200, "GET", "/vehicles/location/"+vehID, nil)
	cur := loc["current_location"].(map[string]any)
	if cur["latitude"] != 37.79 || len(loc["current_geofences"].([]any)) != 0 {
		t.Errorf("unexpected current location: %v", loc)
	}

	// History: entry then exit, newest first.
	hist := s.mustStatus(200, "GET", "/violations/history?vehicle_id="+vehID, nil)
	if hist["total_count"] != float64(2) {
		t.Fatalf("expected 2 events, got %v", hist)
	}
	first := hist["violations"].([]any)[0].(map[string]any)
	if first["event_type"] != "exit" || first["vehicle_number"] != "KA-01-AB-1234" {
		t.Errorf("unexpected newest event: %v", first)
	}
	hist = s.mustStatus(200, "GET", "/violations/history?limit=1&offset=1&start_date=2025-01-01T00:00:00Z", nil)
	if len(hist["violations"].([]any)) != 1 || hist["total_count"] != float64(2) {
		t.Errorf("pagination wrong: %v", hist)
	}
	s.mustStatus(400, "GET", "/violations/history?limit=501", nil)

	list := s.mustStatus(200, "GET", "/geofences?category=restricted_zone", nil)
	got := list["geofences"].([]any)[0].(map[string]any)["coordinates"].([]any)[0].([]any)
	if got[0] != 37.7749 || got[1] != -122.4194 {
		t.Errorf("coordinates should come back as [lat, lng], got %v", got)
	}
	if n := len(s.mustStatus(200, "GET", "/geofences?category=toll_zone", nil)["geofences"].([]any)); n != 0 {
		t.Errorf("category filter returned %d fences", n)
	}

	alerts := s.mustStatus(200, "GET", "/alerts?geofence_id="+geoID, nil)["alerts"].([]any)
	if len(alerts) != 1 || alerts[0].(map[string]any)["geofence_name"] != "Downtown Delivery Zone" {
		t.Errorf("unexpected alerts list: %v", alerts)
	}
}

func TestVehicleSpecificRuleOnlyFiresForThatVehicle(t *testing.T) {
	s := newTestServer(t)
	geoID := s.mustStatus(201, "POST", "/geofences", map[string]any{
		"name": "Yard", "coordinates": downtown, "category": "delivery_zone",
	})["id"].(string)
	watched := s.mustStatus(201, "POST", "/vehicles", map[string]any{
		"vehicle_number": "A-1", "driver_name": "A", "vehicle_type": "van", "phone": "+1111111111",
	})["id"].(string)
	other := s.mustStatus(201, "POST", "/vehicles", map[string]any{
		"vehicle_number": "B-2", "driver_name": "B", "vehicle_type": "van", "phone": "+2222222222",
	})["id"].(string)
	s.mustStatus(201, "POST", "/alerts/configure", map[string]any{
		"geofence_id": geoID, "vehicle_id": watched, "event_type": "entry",
	})

	out := s.mustStatus(200, "POST", "/vehicles/location", location(other, 37.78, -122.415, "2025-01-15T10:00:00Z"))
	if out["alerts_triggered"] != float64(0) {
		t.Errorf("rule for another vehicle fired: %v", out)
	}
	out = s.mustStatus(200, "POST", "/vehicles/location", location(watched, 37.78, -122.415, "2025-01-15T10:00:00Z"))
	if out["alerts_triggered"] != float64(1) {
		t.Errorf("rule for this vehicle did not fire: %v", out)
	}
	// Events are recorded for both vehicles even without a matching rule.
	hist := s.mustStatus(200, "GET", "/violations/history?geofence_id="+geoID, nil)
	if hist["total_count"] != float64(2) {
		t.Errorf("expected 2 events, got %v", hist["total_count"])
	}
}

func TestValidationAndNotFound(t *testing.T) {
	s := newTestServer(t)

	bowTie := [][]float64{{0, 0}, {1, 1}, {1, 0}, {0, 1}, {0, 0}}
	_, out := s.do("POST", "/geofences", map[string]any{"name": "x", "coordinates": bowTie, "category": "toll_zone"})
	if msg := fmt.Sprint(out["error"]); !strings.Contains(msg, "Self-intersection") {
		t.Errorf("self-intersecting polygon should be rejected, got %v", out)
	}

	open := [][]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	s.mustStatus(400, "POST", "/geofences", map[string]any{"name": "x", "coordinates": open, "category": "toll_zone"})
	s.mustStatus(400, "GET", "/geofences?category=parking", nil)
	s.mustStatus(404, "GET", "/vehicles/location/veh_missing", nil)
	s.mustStatus(404, "POST", "/vehicles/location", location("veh_missing", 1, 1, "2025-01-15T10:00:00Z"))
	s.mustStatus(400, "POST", "/vehicles/location", map[string]any{"vehicle_id": "veh_x", "longitude": 1, "timestamp": "2025-01-15T10:00:00Z"})
	s.mustStatus(404, "GET", "/no/such/route", nil)
	s.mustStatus(200, "GET", "/health", nil)

	veh := s.mustStatus(201, "POST", "/vehicles", map[string]any{
		"vehicle_number": "C-3", "driver_name": "C", "vehicle_type": "car", "phone": "+3333333333",
	})["id"].(string)
	loc := s.mustStatus(200, "GET", "/vehicles/location/"+veh, nil)
	if loc["current_location"] != nil {
		t.Errorf("vehicle with no updates should have null location: %v", loc)
	}
}

func location(vehicleID string, lat, lng float64, ts string) map[string]any {
	return map[string]any{"vehicle_id": vehicleID, "latitude": lat, "longitude": lng, "timestamp": ts}
}

func readAlert(t *testing.T, ws *websocket.Conn) map[string]any {
	t.Helper()
	_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	var msg map[string]any
	if err := ws.ReadJSON(&msg); err != nil {
		t.Fatalf("no alert received: %v", err)
	}
	return msg
}

func waitHealthClients(t *testing.T, s *testServer, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, out := s.do("GET", "/health", nil)
		if out["websocket_clients"] == float64(n) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected %d websocket clients, health says %v", n, out["websocket_clients"])
		}
		time.Sleep(20 * time.Millisecond)
	}
}
