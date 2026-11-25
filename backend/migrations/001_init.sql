-- Initial schema for the geofencing service.
-- Geometries use SRID 4326 (WGS 84). PostGIS stores points as (x = longitude, y = latitude).

CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE geofences (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category    TEXT NOT NULL CHECK (category IN ('delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area')),
    boundary    geometry(Polygon, 4326) NOT NULL CHECK (ST_IsValid(boundary)),
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Spatial index: point-in-polygon lookups only test fences whose bounding box contains the point.
CREATE INDEX geofences_boundary_gix ON geofences USING GIST (boundary);
CREATE INDEX geofences_category_idx ON geofences (category, created_at DESC);

CREATE TABLE vehicles (
    id               TEXT PRIMARY KEY,
    vehicle_number   TEXT NOT NULL UNIQUE,
    driver_name      TEXT NOT NULL,
    vehicle_type     TEXT NOT NULL,
    phone            TEXT NOT NULL,
    status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    -- Timestamp of the newest location that changed geofence state. Older,
    -- late-arriving updates are kept as history but don't change state.
    last_recorded_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Every location report, kept for history.
CREATE TABLE vehicle_locations (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  TEXT NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    point       geometry(Point, 4326) NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX vehicle_locations_vehicle_time_idx ON vehicle_locations (vehicle_id, recorded_at DESC, id DESC);

-- Which geofences each vehicle is inside right now. Entry/exit detection is
-- a set difference between these rows and the fences covering a new point.
CREATE TABLE vehicle_geofence_state (
    vehicle_id  TEXT NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    geofence_id TEXT NOT NULL REFERENCES geofences (id) ON DELETE CASCADE,
    entered_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (vehicle_id, geofence_id)
);

-- Alert rules ("alerts" in the public API). NULL vehicle_id = all vehicles.
CREATE TABLE alert_rules (
    id          TEXT PRIMARY KEY,
    geofence_id TEXT NOT NULL REFERENCES geofences (id) ON DELETE CASCADE,
    vehicle_id  TEXT REFERENCES vehicles (id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL CHECK (event_type IN ('entry', 'exit', 'both')),
    status      TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX alert_rules_unique_idx ON alert_rules (geofence_id, COALESCE(vehicle_id, ''), event_type);
CREATE INDEX alert_rules_active_geofence_idx ON alert_rules (geofence_id) WHERE status = 'active';

-- Every entry/exit, served by GET /violations/history.
CREATE TABLE geofence_events (
    id          TEXT PRIMARY KEY,
    vehicle_id  TEXT NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    geofence_id TEXT NOT NULL REFERENCES geofences (id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL CHECK (event_type IN ('entry', 'exit')),
    point       geometry(Point, 4326) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX geofence_events_vehicle_idx ON geofence_events (vehicle_id, occurred_at DESC);
CREATE INDEX geofence_events_geofence_idx ON geofence_events (geofence_id, occurred_at DESC);
CREATE INDEX geofence_events_time_idx ON geofence_events (occurred_at DESC, id DESC);

-- Every real-time alert that was sent, with the exact payload. One row per
-- event even when several rules matched it (their ids are kept in rule_ids).
CREATE TABLE alert_notifications (
    id         TEXT PRIMARY KEY, -- the event_id sent over the WebSocket (evt_...)
    event_id   TEXT NOT NULL REFERENCES geofence_events (id) ON DELETE CASCADE,
    rule_ids   TEXT[] NOT NULL,
    payload    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX alert_notifications_created_idx ON alert_notifications (created_at DESC);
