# Backend Design — Go API

## 1. Stack

| Concern | Library |
|---|---|
| Language | Go 1.25 |
| Router | `github.com/go-chi/chi/v5` |
| CORS | `github.com/go-chi/cors` |
| Rate limit | `github.com/go-chi/httprate` |
| Postgres | `github.com/jackc/pgx/v5` (`pgxpool`) |
| WebSocket | `github.com/gorilla/websocket` |
| Logging | `log/slog` (stdlib) |
| IDs | `crypto/rand` (stdlib) |

## 2. Folder structure

```
backend/
├── cmd/server/main.go          # wiring: config → db → migrate → hub → router → server, graceful shutdown
├── internal/
│   ├── config/config.go        # env vars with defaults
│   ├── db/
│   │   ├── db.go               # pgxpool connect with retry
│   │   └── migrate.go          # apply embedded migrations in order, tracked in schema_migrations
│   ├── httpapi/
│   │   ├── router.go           # routes + middleware
│   │   ├── respond.go          # JSON writer, time_ns wrapper, error mapping
│   │   ├── middleware.go       # API key (optional), request logging
│   │   ├── geofences.go        # POST/GET /geofences
│   │   ├── vehicles.go         # POST/GET /vehicles, location endpoints
│   │   ├── alerts.go           # alert rules
│   │   ├── violations.go       # history
│   │   └── ws.go               # /ws/alerts upgrade
│   ├── domain/
│   │   ├── models.go           # Geofence, Vehicle, AlertRule, Event, AlertMessage
│   │   ├── validate.go         # pure validation (unit-tested)
│   │   └── ids.go              # prefixed IDs: geo_, veh_, alert_, viol_, evt_
│   ├── tracking/
│   │   ├── service.go          # ProcessLocation: transaction + diff + alerts
│   │   └── diff.go             # entries/exits set difference (unit-tested)
│   ├── realtime/
│   │   ├── hub.go              # register/unregister/broadcast
│   │   └── client.go           # readPump / writePump, ping/pong
│   └── store/
│       ├── store.go            # Store struct, tx helper
│       ├── geofences.go
│       ├── vehicles.go
│       ├── locations.go
│       ├── alerts.go
│       └── events.go
├── migrations/
│   ├── embed.go                # //go:embed *.sql
│   └── 001_init.sql
├── Dockerfile
└── go.mod
```

Layering rule: **handlers → tracking/store → db**. Handlers never write SQL; store never touches HTTP.

## 3. Configuration (env)

| Var | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP port |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/geofence?sslmode=disable` | Postgres DSN |
| `CORS_ORIGINS` | `*` | Comma-separated allowed origins (also used for WS origin check) |
| `API_KEY` | *(empty = disabled)* | If set, every request except `/health` must send a matching `X-API-Key` header (off by default) |
| `LOCATION_RATE_LIMIT` | `120` | Max `POST /vehicles/location` per IP per minute |

## 4. Response conventions

### `time_ns` (mandatory on every response)

Every handler is wrapped by `handle()`:

```go
type apiFunc func(r *http.Request) (status int, body any, err error)

func handle(fn apiFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        status, body, err := fn(r)
        if err != nil {
            status, body = toErrorResponse(err)
        }
        writeJSON(w, status, body, time.Since(start)) // injects "time_ns" as a string
    }
}
```

`writeJSON` encodes the body to a JSON object and adds `"time_ns": "<nanoseconds>"`. Because it happens in one place, **no endpoint (including errors and 404s) can forget it**. Time is measured from handler entry to just before encoding.

### Errors

```json
{ "error": { "code": "validation_error", "message": "coordinates: first and last point must be identical" }, "time_ns": "51234" }
```

| Status | When |
|---|---|
| 400 | Malformed JSON, failed validation, invalid polygon, bad query params |
| 401 | `API_KEY` set and header missing/wrong |
| 404 | Unknown vehicle / geofence / alert / route |
| 409 | Duplicate vehicle number or duplicate alert rule |
| 429 | Rate limit hit (custom handler so `time_ns` is included) |
| 500 | Unexpected error (logged with request id; message is generic) |

### IDs

Prefix + 12 random base32 chars: `geo_k3j9x2mq8ptw`, `veh_…`, `alert_…`, `viol_…`, `evt_…`. Matches the spec's `geo_123` style, not guessable, no sequence leaks.

## 5. Database schema (`migrations/001_init.sql`)

```sql
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE geofences (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL CHECK (category IN ('delivery_zone','restricted_zone','toll_zone','customer_area')),
    boundary    geometry(Polygon, 4326) NOT NULL,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX geofences_boundary_gix ON geofences USING GIST (boundary);
CREATE INDEX geofences_category_idx ON geofences (category);

CREATE TABLE vehicles (
    id             TEXT PRIMARY KEY,
    vehicle_number TEXT NOT NULL UNIQUE,
    driver_name    TEXT NOT NULL,
    vehicle_type   TEXT NOT NULL,
    phone          TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active',
    last_recorded_at TIMESTAMPTZ,            -- newest processed ping (out-of-order guard)
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE vehicle_locations (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    point       geometry(Point, 4326) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX vehicle_locations_vehicle_time_idx ON vehicle_locations (vehicle_id, recorded_at DESC);

CREATE TABLE vehicle_geofence_state (
    vehicle_id  TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    entered_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (vehicle_id, geofence_id)
);

CREATE TABLE alert_rules (
    id          TEXT PRIMARY KEY,
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    vehicle_id  TEXT REFERENCES vehicles(id) ON DELETE CASCADE,   -- NULL = all vehicles
    event_type  TEXT NOT NULL CHECK (event_type IN ('entry','exit','both')),
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX alert_rules_unique_idx
    ON alert_rules (geofence_id, COALESCE(vehicle_id, ''), event_type);
CREATE INDEX alert_rules_geofence_idx ON alert_rules (geofence_id) WHERE status = 'active';

CREATE TABLE geofence_events (
    id          TEXT PRIMARY KEY,
    vehicle_id  TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL CHECK (event_type IN ('entry','exit')),
    point       geometry(Point, 4326) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX geofence_events_vehicle_idx  ON geofence_events (vehicle_id, occurred_at DESC);
CREATE INDEX geofence_events_geofence_idx ON geofence_events (geofence_id, occurred_at DESC);
CREATE INDEX geofence_events_time_idx     ON geofence_events (occurred_at DESC);

CREATE TABLE alert_notifications (
    id          TEXT PRIMARY KEY,                -- evt_… (the WebSocket event_id)
    event_id    TEXT NOT NULL REFERENCES geofence_events(id) ON DELETE CASCADE,
    rule_ids    TEXT[] NOT NULL,                 -- every rule that matched this event
    payload     JSONB NOT NULL,                  -- exact message sent over WS
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Coordinate order

API uses `[lat, lng]`; PostGIS uses `(x=lng, y=lat)`.

- **Write:** Go builds GeoJSON with swapped pairs → `ST_SetSRID(ST_GeomFromGeoJSON($1), 4326)`.
- **Read:** `ST_AsGeoJSON(boundary)` → Go swaps back to `[lat, lng]`.
- Points: `ST_SetSRID(ST_MakePoint($lng, $lat), 4326)` — note argument order.

Swapping lives in one helper (`domain.ToGeoJSONRing` / `FromGeoJSONRing`) with unit tests.

## 6. Endpoints

All paths exactly as in the spec. Extra endpoints are additive only.

### POST /geofences
Validation (`domain.ValidateGeofence`):
- `name` non-empty after trim (≤ 200 chars); `category` in the 4 allowed values
- `coordinates` ≥ 4 pairs, each pair exactly 2 numbers, lat ∈ [-90, 90], lng ∈ [-180, 180]
- first == last (closed ring); at least 3 distinct points
- then DB check `ST_IsValid` (rejects self-intersection) → 400 with `ST_IsValidReason`

Response `201`: `{ id, name, status, time_ns }`

### GET /geofences?category=
- Optional `category` (validated), optional `limit` / `offset` (bonus pagination; default returns all).
- `SELECT …, ST_AsGeoJSON(boundary) … ORDER BY created_at DESC`

### POST /vehicles
- All 4 fields required, trimmed. Phone matches `^\+?[0-9 ()-]{7,20}$`.
- Unique violation on `vehicle_number` → 409.
- Response `201`: `{ id, vehicle_number, status, time_ns }`

### GET /vehicles
- Ordered by `created_at DESC`, optional `limit` / `offset`.

### POST /vehicles/location
- `vehicle_id` required; `latitude` / `longitude` decoded as `*float64` (so `0` is valid, missing is an error); `timestamp` RFC 3339.
- Rate limited per IP. Logic in section 7.
- Response `200`: `{ vehicle_id, location_updated: true, current_geofences: [{geofence_id, geofence_name, status: "inside"}], time_ns }`

### GET /vehicles/location/{vehicle_id}
- 404 if vehicle unknown.
- Latest row from `vehicle_locations` by `recorded_at DESC`; geofences from `vehicle_geofence_state` joined with `geofences`.
- If no location yet: `current_location: null`, `current_geofences: []`.

### POST /alerts/configure
- `geofence_id` must exist (404), `vehicle_id` if given must exist (404), `event_type` ∈ entry/exit/both.
- Duplicate → 409.
- Response `201`: `{ alert_id, geofence_id, vehicle_id, event_type, status, time_ns }` (`vehicle_id` is `null` when omitted).

### GET /alerts?geofence_id=&vehicle_id=
- Joins geofence name and vehicle number (LEFT JOIN for "all vehicles" rules).

### Extra (additive, for "manage existing alerts")
- `PATCH /alerts/{alert_id}` body `{ "status": "active" | "inactive" }`
- `DELETE /alerts/{alert_id}`

### GET /violations/history
- Filters: `vehicle_id`, `geofence_id`, `start_date`, `end_date` (RFC 3339), `limit` (default 50, max 500, 400 if out of range), `offset` (bonus).
- Single query with `COUNT(*) OVER()` for `total_count`, ordered by `occurred_at DESC`.
- Dynamic `WHERE` built from a list of parameterised clauses (`$1, $2…`), never string-concatenated values.

### GET /ws/alerts
WebSocket upgrade. Server → client messages only, exactly the spec's format.

### GET /health
`{ "status": "ok", "database": "ok", "websocket_clients": 2, "time_ns": "…" }` — used by Docker healthcheck and the frontend "waking server" check.

## 7. Location processing (`tracking.Service.ProcessLocation`)

```
BEGIN
  SELECT id, vehicle_number, driver_name, last_recorded_at
    FROM vehicles WHERE id = $1 FOR UPDATE           -- 404 if missing; serialises same-vehicle pings
  INSERT INTO vehicle_locations (...)                -- always keep history

  IF last_recorded_at IS NOT NULL AND ts < last_recorded_at:
      current := state rows                           -- out-of-order: don't change state
      COMMIT; return current
  
  inside := SELECT id, name, category FROM geofences
            WHERE status = 'active'
              AND ST_Covers(boundary, ST_SetSRID(ST_MakePoint($lng, $lat), 4326))
  previous := SELECT geofence_id FROM vehicle_geofence_state WHERE vehicle_id = $1

  entries, exits := diff(previous, inside)            -- pure function, unit-tested

  INSERT state rows for entries; DELETE state rows for exits
  INSERT geofence_events (one per entry/exit)
  UPDATE vehicles SET last_recorded_at = ts

  rules := SELECT … FROM alert_rules
           WHERE status = 'active'
             AND geofence_id = ANY($fences_with_events)
             AND (vehicle_id IS NULL OR vehicle_id = $1)
  for each event × matching rule (rule.event_type = event.type OR 'both'):
      build AlertMessage, INSERT alert_notifications
COMMIT
hub.Publish(messages...)   -- after commit, non-blocking
```

Why `ST_Covers` not `ST_Contains`: `ST_Contains` is false for points on the boundary. The spec's sample point `(37.7849, -122.4194)` is a vertex of its sample polygon and the expected answer is `inside`.

Dedup: if two rules match the same event (e.g. "all vehicles" + "this vehicle"), one notification is stored with all matching `rule_ids`, and the hub sends **one message per event** so the UI doesn't double-toast.

## 8. WebSocket hub (`internal/realtime`)

```go
type Hub struct {
    clients    map[*Client]struct{}
    register   chan *Client
    unregister chan *Client
    broadcast  chan []byte   // buffered (256)
}

func (h *Hub) Run(ctx context.Context)          // single goroutine owns `clients`
func (h *Hub) Publish(msg []byte)               // select { case h.broadcast <- msg: default: log drop }
```

Client:
- `send chan []byte` (buffer 64). If full during broadcast → client is closed (slow consumer).
- `writePump`: writes messages, sends ping every 30s, write deadline 10s.
- `readPump`: discards incoming messages, sets read deadline 60s, extended on pong; on error → unregister.
- `Upgrader.CheckOrigin` checks against `CORS_ORIGINS`.

Message (exact spec shape):

```json
{
  "event_id": "evt_…", "event_type": "entry", "timestamp": "2025-01-15T10:35:00Z",
  "vehicle":  { "vehicle_id": "veh_…", "vehicle_number": "KA-01-AB-1234", "driver_name": "John Doe" },
  "geofence": { "geofence_id": "geo_…", "geofence_name": "Downtown Delivery Zone", "category": "delivery_zone" },
  "location": { "latitude": 37.7849, "longitude": -122.4194 }
}
```

## 9. Middleware order

```
RequestID → RealIP → slog logger → Recoverer (JSON 500 with time_ns) → CORS → MaxBodySize(1MB) → APIKey (if enabled)
  POST /vehicles/location additionally: httprate.LimitByIP(LOCATION_RATE_LIMIT, 1m) with JSON 429
NotFound / MethodNotAllowed → JSON with time_ns
```

## 10. Startup & shutdown

1. Load config.
2. Connect `pgxpool` with retry (DB may start slower than the API in compose).
3. Run migrations (idempotent, tracked in `schema_migrations`).
4. Start hub goroutine.
5. `http.Server` with read/write timeouts (WebSocket route excluded from write timeout via hijack).
6. On SIGINT/SIGTERM: `server.Shutdown(ctx)` with 10s timeout, stop hub, close pool.

## 11. Testing (bonus)

| Test | Type |
|---|---|
| Geofence/vehicle/location/alert validation | Table-driven unit tests |
| `[lat,lng]` ↔ GeoJSON swap round-trip | Unit |
| Entry/exit `diff` (empty→in, in→out, overlapping fences, no change) | Unit |
| `time_ns` present + string on success and error | `httptest` unit |
| Hub: broadcast reaches N clients, slow client dropped | Unit with fake clients |
| Point-in-polygon incl. vertex/edge cases | Integration (runs only when `TEST_DATABASE_URL` set) |

Run: `go test ./...`

## 12. Docker

```dockerfile
# build
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# run
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/server"]
```

`docker-compose.yml` (repo root): `db` (`postgis/postgis:17-3.5`, healthcheck `pg_isready`, named volume) → `api` (build `./backend`, `depends_on: db: condition: service_healthy`) → optional `web` (frontend via nginx for a one-command local demo).

## 13. Example requests

```bash
API=http://localhost:8080

curl -s -X POST $API/geofences -H 'Content-Type: application/json' -d '{
  "name":"Downtown Delivery Zone","description":"Main area",
  "coordinates":[[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]],
  "category":"delivery_zone"}'

curl -s -X POST $API/vehicles -H 'Content-Type: application/json' -d '{
  "vehicle_number":"KA-01-AB-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890"}'

curl -s -X POST $API/alerts/configure -H 'Content-Type: application/json' -d '{
  "geofence_id":"geo_…","event_type":"both"}'

curl -s -X POST $API/vehicles/location -H 'Content-Type: application/json' -d '{
  "vehicle_id":"veh_…","latitude":37.78,"longitude":-122.415,"timestamp":"2025-01-15T10:35:00Z"}'

curl -s "$API/violations/history?vehicle_id=veh_…&limit=100"

# watch alerts
npx wscat -c ws://localhost:8080/ws/alerts
```
