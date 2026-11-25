package store

import (
	"context"
	"fmt"
	"time"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

// InsertLocation stores one location report in the history table.
func (q *Queries) InsertLocation(ctx context.Context, u domain.LocationUpdate) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO vehicle_locations (vehicle_id, point, recorded_at)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326), $4)`,
		u.VehicleID, u.Longitude, u.Latitude, u.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert location: %w", err)
	}
	return nil
}

// LatestLocation returns the most recent reported position, or nil if the
// vehicle has never reported one.
func (q *Queries) LatestLocation(ctx context.Context, vehicleID string) (*domain.Position, error) {
	var p domain.Position
	err := q.db.QueryRow(ctx, `
		SELECT ST_Y(point), ST_X(point), recorded_at
		FROM vehicle_locations
		WHERE vehicle_id = $1
		ORDER BY recorded_at DESC, id DESC
		LIMIT 1`, vehicleID,
	).Scan(&p.Latitude, &p.Longitude, &p.Timestamp)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("latest location: %w", err)
	}
	p.Timestamp = p.Timestamp.UTC()
	return &p, nil
}

// CurrentFences returns the geofences the vehicle is inside right now.
func (q *Queries) CurrentFences(ctx context.Context, vehicleID string) ([]domain.FenceRef, error) {
	rows, err := q.db.Query(ctx, `
		SELECT g.id, g.name, g.category
		FROM vehicle_geofence_state s
		JOIN geofences g ON g.id = s.geofence_id
		WHERE s.vehicle_id = $1
		ORDER BY s.entered_at, g.id`, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("current geofences: %w", err)
	}
	return scanFenceRefs(rows)
}

// EnterFence records that the vehicle is now inside the geofence.
func (q *Queries) EnterFence(ctx context.Context, vehicleID, geofenceID string, at time.Time) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO vehicle_geofence_state (vehicle_id, geofence_id, entered_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (vehicle_id, geofence_id) DO NOTHING`,
		vehicleID, geofenceID, at)
	if err != nil {
		return fmt.Errorf("enter geofence: %w", err)
	}
	return nil
}

// ExitFence records that the vehicle has left the geofence.
func (q *Queries) ExitFence(ctx context.Context, vehicleID, geofenceID string) error {
	_, err := q.db.Exec(ctx,
		"DELETE FROM vehicle_geofence_state WHERE vehicle_id = $1 AND geofence_id = $2",
		vehicleID, geofenceID)
	if err != nil {
		return fmt.Errorf("exit geofence: %w", err)
	}
	return nil
}
