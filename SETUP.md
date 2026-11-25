# Setup Guide

This guide covers running the project locally, testing the API, using the dashboard, and deploying it.

- [1. Prerequisites](#1-prerequisites)
- [2. Run everything with Docker Compose](#2-run-everything-with-docker-compose)
- [3. Run without Docker (development)](#3-run-without-docker-development)
- [4. Configuration](#4-configuration)
- [5. API testing guide](#5-api-testing-guide)
- [6. Frontend usage guide](#6-frontend-usage-guide)
- [7. Running the tests](#7-running-the-tests)
- [8. Deployment](#8-deployment)
- [9. Architecture overview](#9-architecture-overview)

---

## 1. Prerequisites

| Tool | Version | Needed for |
|---|---|---|
| Docker + Docker Compose | Docker 24+ (Compose v2) | Running the full stack with one command |
| Go | 1.25+ | Backend development without Docker |
| Node.js | 20.19+ or 22.12+ (npm 10+) | Frontend development |
| PostgreSQL + PostGIS | Postgres 15+ with PostGIS 3.x | Only if running the backend without Docker |
| curl, optionally `wscat` (`npx wscat`) | any | Trying the API and WebSocket |

Backend dependencies (see `backend/go.mod`): `go-chi/chi` (router), `go-chi/cors`, `go-chi/httprate` (rate limiting), `jackc/pgx` (PostgreSQL driver), `gorilla/websocket`.

Frontend dependencies (see `frontend/package.json`): React 19, Vite, TypeScript, TanStack Query, React Router, Leaflet + react-leaflet, Leaflet-Geoman (polygon drawing), Tailwind CSS, sonner (toasts), date-fns, lucide-react.

## 2. Run everything with Docker Compose

```bash
git clone https://github.com/chandug3121/Mapup_Geo-fence.git
cd Mapup_Geo-fence
docker compose up --build
```

| Service | URL | What it is |
|---|---|---|
| `web` | http://localhost:3000 | React dashboard (nginx) |
| `api` | http://localhost:8080 | Go API + WebSocket at `ws://localhost:8080/ws/alerts` |
| `db` | `localhost:5432` (postgres / postgres) | PostgreSQL 17 + PostGIS |

The API waits for the database, applies migrations on startup and is ready when the log shows `server listening`.

Load demo data (4 zones in San Francisco, 3 vehicles, 3 alert rules):

```bash
./scripts/seed.sh            # or: ./scripts/seed.sh http://localhost:8080
```

Useful commands:

```bash
docker compose logs -f api        # follow API logs
docker compose down               # stop (keeps data)
docker compose down -v            # stop and delete the database volume
```

## 3. Run without Docker (development)

**Database.** Start PostGIS any way you like; the simplest is just the compose database:

```bash
docker compose up -d db
```

**Backend:**

```bash
cd backend
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/geofence?sslmode=disable"
go run ./cmd/server
```

**Frontend:**

```bash
cd frontend
cp .env.example .env.local      # VITE_API_URL=http://localhost:8080
npm install
npm run dev                     # http://localhost:5173
```

## 4. Configuration

### Backend (environment variables)

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port (hosting platforms like Render set this automatically) |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/geofence?sslmode=disable` | PostgreSQL connection string. The database user must be able to run `CREATE EXTENSION postgis` (or PostGIS must already be enabled). |
| `CORS_ORIGINS` | `*` | Comma-separated list of allowed browser origins, e.g. `https://my-app.vercel.app`. Also applied to the WebSocket handshake. |
| `API_KEY` | *(empty)* | When set, every request except `/health` must send `X-API-Key: <key>` (WebSocket: `?api_key=<key>`). Off by default. |
| `LOCATION_RATE_LIMIT` | `120` | Max `POST /vehicles/location` requests per client IP per minute. |

### Frontend (build-time variables)

| Variable | Description |
|---|---|
| `VITE_API_URL` | Backend base URL, e.g. `https://geofence-api.onrender.com`. The WebSocket URL is derived from it (`https` → `wss`). |
| `VITE_API_KEY` | Only if the backend has `API_KEY` set. |

Vite bakes these into the bundle at build time, so rebuild/redeploy after changing them.

## 5. API testing guide

Every response is JSON and includes `"time_ns"`: the handler's execution time in nanoseconds, as a string. Errors look like:

```json
{ "error": { "code": "validation_error", "message": "coordinates: first and last points must be identical to close the polygon" }, "time_ns": "48211" }
```

Status codes: `200`/`201` success, `400` invalid input, `401` missing API key (only when enabled), `404` unknown resource, `409` duplicate, `429` rate limit, `500` server error.

Set a base URL first:

```bash
API=http://localhost:8080
```

### Health

```bash
curl -s $API/health
# {"database":"ok","status":"ok","websocket_clients":0,"time_ns":"512334"}
```

### 1. Create a geofence — `POST /geofences`

```bash
curl -s -X POST $API/geofences -H 'Content-Type: application/json' -d '{
  "name": "Downtown Delivery Zone",
  "description": "Main delivery area for downtown customers",
  "coordinates": [[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]],
  "category": "restricted_zone"
}'
# {"id":"geo_…","name":"Downtown Delivery Zone","status":"active","time_ns":"…"}
```

Save the id: `GEO=geo_…`

Validation examples (each returns `400`):

```bash
# not closed
curl -s -X POST $API/geofences -H 'Content-Type: application/json' \
  -d '{"name":"x","category":"toll_zone","coordinates":[[0,0],[1,0],[1,1],[0,1]]}'
# self-intersecting "bow tie"
curl -s -X POST $API/geofences -H 'Content-Type: application/json' \
  -d '{"name":"x","category":"toll_zone","coordinates":[[0,0],[1,1],[1,0],[0,1],[0,0]]}'
```

### 2. List geofences — `GET /geofences`

```bash
curl -s "$API/geofences"
curl -s "$API/geofences?category=restricted_zone"
curl -s "$API/geofences?limit=10&offset=0"     # optional paging
```

### 3. Register a vehicle — `POST /vehicles`

```bash
curl -s -X POST $API/vehicles -H 'Content-Type: application/json' -d '{
  "vehicle_number": "KA-01-AB-1234",
  "driver_name": "John Doe",
  "vehicle_type": "truck",
  "phone": "+1234567890"
}'
# {"id":"veh_…","vehicle_number":"KA-01-AB-1234","status":"active","time_ns":"…"}
```

Save the id: `VEH=veh_…` (registering the same number again returns `409`).

### 4. List vehicles — `GET /vehicles`

```bash
curl -s $API/vehicles
```

### 7. Configure an alert — `POST /alerts/configure`

```bash
# all vehicles, entry and exit
curl -s -X POST $API/alerts/configure -H 'Content-Type: application/json' \
  -d "{\"geofence_id\":\"$GEO\",\"event_type\":\"both\"}"

# one vehicle, entry only
curl -s -X POST $API/alerts/configure -H 'Content-Type: application/json' \
  -d "{\"geofence_id\":\"$GEO\",\"vehicle_id\":\"$VEH\",\"event_type\":\"entry\"}"
```

### Watch alerts in real time — `GET /ws/alerts`

In a second terminal:

```bash
npx wscat -c ws://localhost:8080/ws/alerts
```

### 5. Update a location — `POST /vehicles/location`

```bash
# outside the zone
curl -s -X POST $API/vehicles/location -H 'Content-Type: application/json' \
  -d "{\"vehicle_id\":\"$VEH\",\"latitude\":37.70,\"longitude\":-122.50,\"timestamp\":\"2025-01-15T10:30:00Z\"}"

# inside the zone -> entry event, alert appears in wscat
curl -s -X POST $API/vehicles/location -H 'Content-Type: application/json' \
  -d "{\"vehicle_id\":\"$VEH\",\"latitude\":37.7849,\"longitude\":-122.4194,\"timestamp\":\"2025-01-15T10:35:00Z\"}"
# {"vehicle_id":"veh_…","location_updated":true,
#  "current_geofences":[{"geofence_id":"geo_…","geofence_name":"Downtown Delivery Zone","category":"restricted_zone","status":"inside"}],
#  "events":[{"geofence_id":"geo_…","geofence_name":"Downtown Delivery Zone","event_type":"entry"}],
#  "alerts_triggered":1,"stale":false,"time_ns":"…"}

# leave again -> exit event
curl -s -X POST $API/vehicles/location -H 'Content-Type: application/json' \
  -d "{\"vehicle_id\":\"$VEH\",\"latitude\":37.79,\"longitude\":-122.40,\"timestamp\":\"2025-01-15T10:40:00Z\"}"
```

Notes:
- The point `37.7849, -122.4194` is exactly a corner of the polygon; points on an edge or corner count as **inside**.
- An update with a timestamp older than the latest one is stored in history but does not change the inside/outside state (`"stale": true`).
- Over `LOCATION_RATE_LIMIT` requests per minute from one IP returns `429`.

### 6. Current location — `GET /vehicles/location/{vehicle_id}`

```bash
curl -s $API/vehicles/location/$VEH
```

### 8. List alert rules — `GET /alerts`

```bash
curl -s "$API/alerts"
curl -s "$API/alerts?geofence_id=$GEO"
curl -s "$API/alerts?vehicle_id=$VEH"
```

Extra endpoints used by the dashboard to manage rules:

```bash
curl -s -X PATCH $API/alerts/alert_… -H 'Content-Type: application/json' -d '{"status":"inactive"}'
curl -s -X DELETE $API/alerts/alert_…
```

### 9. Violation history — `GET /violations/history`

```bash
curl -s "$API/violations/history"
curl -s "$API/violations/history?vehicle_id=$VEH&limit=100"
curl -s "$API/violations/history?geofence_id=$GEO&start_date=2025-01-01T00:00:00Z&end_date=2025-12-31T23:59:59Z"
curl -s "$API/violations/history?limit=50&offset=50"     # page 2
```

`limit` defaults to 50 (max 500). `total_count` is the number of matching events across all pages.

## 6. Frontend usage guide

The dashboard ("Fencewatch") has five screens; on phones they become tabs at the bottom.

**Live map**
- Geofences are drawn by category: blue delivery, red dashed restricted, amber dotted toll, green customer. Tap the chips at the top to show or hide a category.
- **Draw zone**: click to add corners, click the first corner to finish, then fill in the name, description and category. For a restricted zone you can tick *Alert me when any vehicle enters* to create the alert rule at the same time.
- **Move vehicle** tab: choose a vehicle, then either type latitude/longitude or press **Pick on map** and click the map. Keep *Use the current time* ticked or set a timestamp. The result shows which zones the vehicle is inside and which entries/exits happened.
- **Simulate a route**: press **Plan on map**, click several points across zone boundaries, then **Play**. One update is sent every 1.5 s, so entries, exits and alerts appear live.
- Vehicles show as markers with their number; a marker pulses red while the vehicle is inside a restricted zone.
- **Live alerts** tab: every alert received over the WebSocket, newest first; click one to fly the map to it. The top bar shows the connection state (Live / Reconnecting / Offline) and a bell with the unread count. Alerts also pop up as toasts (restricted-zone entries in red).

**Geofences**: list with category filter, and **Add by coordinates** for pasting a JSON ring (with live validation and a *Close the polygon for me* helper).

**Vehicles**: register vehicles (with validation) and see each vehicle's latest position, how long ago it reported, and the zones it is in. *Move on map* jumps to the live map with that vehicle selected.

**Alert rules**: create rules (geofence, optional vehicle, entry/exit/both), filter by geofence or vehicle, switch rules on/off, and delete them.

**History**: every entry/exit with filters for vehicle, geofence and date range, page size and paging. Filters are kept in the URL, so a filtered view can be refreshed or shared.

If the backend is asleep (free hosting), a banner explains that it is waking up; requests continue once it responds.

## 7. Running the tests

**Backend** — unit tests (validation, coordinate conversion, entry/exit diff, response format, WebSocket hub) plus end-to-end API tests against a real PostGIS database:

```bash
cd backend
go test ./...                                   # unit tests; database tests are skipped

docker compose up -d db
docker compose exec db createdb -U postgres geofence_test
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/geofence_test?sslmode=disable" go test -race ./...
```

**Frontend:**

```bash
cd frontend
npm test          # unit tests (polygon validation)
npx tsc           # type check
npm run build     # production build
```

GitHub Actions runs both suites on every push (`.github/workflows/`).

## 8. Deployment

The setup below uses free tiers only: **Neon** (PostgreSQL with PostGIS), **Docker Hub** (backend image), **Render** (runs the container) and **Vercel** (frontend). GitHub Actions builds and pushes the image on every push to `main`.

### 8.1 Database — Neon

1. Create a project at neon.com.
2. In the SQL editor run `CREATE EXTENSION IF NOT EXISTS postgis;` (the API also does this on startup).
3. Copy the **direct** connection string (the host without `-pooler`), e.g.
   `postgresql://user:pass@ep-xxx.region.aws.neon.tech/neondb?sslmode=require`.
   If it ends with `&channel_binding=require`, that can stay; remove it only if the connection fails.

### 8.2 Backend image — Docker Hub + GitHub Actions

1. On hub.docker.com: **Account settings → Personal access tokens → Generate new token** (Read & Write).
2. In the GitHub repo: **Settings → Secrets and variables → Actions → New repository secret**:
   - `DOCKERHUB_USERNAME` — your Docker Hub username
   - `DOCKERHUB_TOKEN` — the token
3. Push to `main` (or re-run the *backend* workflow). After tests pass, the image is pushed as `<username>/geofence-backend:latest` and `:<commit-sha>`.

Manual alternative:

```bash
docker build -t <username>/geofence-backend:latest ./backend
docker push <username>/geofence-backend:latest
```

### 8.3 Run the container — Render

1. dashboard.render.com → **New → Web Service → Existing image** → `docker.io/<username>/geofence-backend:latest`, instance type **Free**.
2. Environment variables: `DATABASE_URL` (Neon), `CORS_ORIGINS` (your Vercel URL, e.g. `https://fencewatch.vercel.app`).
3. Health check path: `/health`.
4. Optional: copy **Settings → Deploy hook** into a GitHub secret named `RENDER_DEPLOY_HOOK`; CI will redeploy after each new image.

The free tier sleeps after ~15 minutes without traffic and takes about a minute to wake up. Open `/health` a couple of minutes before a demo.

### 8.4 Frontend — Vercel

1. vercel.com → **Add New → Project** → import this repository.
2. **Root Directory**: `frontend` (framework preset: Vite).
3. Environment variable: `VITE_API_URL=https://<your-service>.onrender.com`.
4. Deploy. `frontend/vercel.json` rewrites all routes to `index.html` so links like `/history` work.

## 9. Architecture overview

```
Browser (React, Vercel) ──REST──▶ Go API (Docker) ──▶ PostgreSQL + PostGIS
          ▲                         │
          └────── WebSocket ◀── alert hub
```

- **Point-in-polygon** runs in PostGIS with `ST_Covers` on a GiST-indexed `geometry(Polygon, 4326)` column, so only geofences whose bounding box contains the point are tested exactly. Points on an edge count as inside.
- **Entry/exit detection**: `vehicle_geofence_state` stores which zones each vehicle is in. On each update the API compares that set with the zones covering the new point (entered = new − old, exited = old − new), inside one transaction that locks the vehicle row so concurrent updates for the same vehicle can't double-fire.
- **Alerts** are saved in the same transaction and pushed to WebSocket clients only after commit, through a non-blocking hub, so the HTTP response is never delayed by slow browsers.
- **`time_ns`** is added by a single wrapper around every handler.

Full design notes: [`docs/architecture.md`](docs/architecture.md), [`docs/backend-design.md`](docs/backend-design.md), [`docs/frontend-design.md`](docs/frontend-design.md).
