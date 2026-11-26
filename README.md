# Fencewatch — Geofencing & Real-time Vehicle Alerts

Define virtual boundaries on a map, track vehicles, and get an alert the moment a vehicle enters or leaves a zone.

| | |
|---|---|
| **Live dashboard** | _add Vercel URL_ |
| **API** | https://geofence-api-bcvn.onrender.com (health: [`/health`](https://geofence-api-bcvn.onrender.com/health)) — free tier, first request after idle can take ~1 min |
| **Docker image** | [`chandu31210/geofence-backend`](https://hub.docker.com/r/chandu31210/geofence-backend) |
| **Video walkthrough** | _add video link_ |

Setup, API examples and deployment steps: **[SETUP.md](SETUP.md)**.

## What it does

- **Geofences**: draw polygons on the map or paste coordinates; four categories (delivery, restricted, toll, customer area), validated for closed rings, coordinate ranges and self-intersection.
- **Vehicles**: register vehicles and see each one's latest position and the zones it is in.
- **Location updates**: type coordinates, click the map, or play back a planned route. Each update detects entries and exits.
- **Alert rules**: per geofence, for all vehicles or one vehicle, on entry, exit or both; switch on/off or delete.
- **Real-time alerts** over WebSocket (`/ws/alerts`): toasts, a live feed and a connection indicator, with automatic reconnect.
- **History**: every entry/exit with filters (vehicle, geofence, date range) and paging.

## Tech stack

| Layer | Choice |
|---|---|
| Backend | Go 1.25, chi router, pgx, gorilla/websocket |
| Database | PostgreSQL + PostGIS (GiST spatial index, `ST_Covers`) |
| Frontend | React 19, TypeScript, Vite, TanStack Query, react-leaflet + Leaflet-Geoman, Tailwind CSS |
| Infra | Docker (multi-stage, distroless), Docker Compose, GitHub Actions, Render, Vercel, Neon |

## Repository layout

```
backend/     Go API — cmd/server, internal/{config,db,domain,httpapi,realtime,store,tracking}, migrations
frontend/    React dashboard — src/{api,components,context,hooks,lib,pages}
docs/        Architecture, backend and frontend design notes
scripts/     seed.sh — demo data
docker-compose.yml, SETUP.md
```

## Quick start

```bash
docker compose up --build      # dashboard http://localhost:3000, API http://localhost:8080
./scripts/seed.sh              # optional demo data
```

## Bonus features

- Rate limiting on `POST /vehicles/location` (per IP, configurable)
- Optional API-key authentication (`API_KEY` env var; off by default)
- Pagination (`limit`/`offset`) on all list endpoints
- Unit tests plus end-to-end API tests against real PostGIS, run in CI

## Design highlights

- Every response carries `time_ns`, added by one wrapper so no handler (or error) can miss it.
- Points on a polygon edge or corner count as inside (`ST_Covers`, not `ST_Contains`) — the spec's own sample point is a polygon corner.
- Entry/exit detection runs in one transaction with a row lock per vehicle; alerts are published after commit through a non-blocking hub.
- Late (out-of-order) updates are kept as history without moving the vehicle back in time.
