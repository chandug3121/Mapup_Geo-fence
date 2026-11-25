package store

import (
	"context"
	"fmt"
	"time"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

// InsertEvent stores one geofence entry or exit.
func (q *Queries) InsertEvent(ctx context.Context, e domain.GeofenceEvent) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO geofence_events (id, vehicle_id, geofence_id, event_type, point, occurred_at)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326), $7)`,
		e.ID, e.VehicleID, e.GeofenceID, string(e.Type), e.Longitude, e.Latitude, e.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert geofence event: %w", err)
	}
	return nil
}

// InsertNotification stores an alert that was sent to WebSocket clients.
func (q *Queries) InsertNotification(ctx context.Context, id, eventID string, ruleIDs []string, payload []byte) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO alert_notifications (id, event_id, rule_ids, payload)
		VALUES ($1, $2, $3, $4)`,
		id, eventID, ruleIDs, payload,
	)
	if err != nil {
		return fmt.Errorf("insert alert notification: %w", err)
	}
	return nil
}

// ViolationFilter narrows GET /violations/history. Zero values mean "any".
type ViolationFilter struct {
	VehicleID  string
	GeofenceID string
	Start      *time.Time
	End        *time.Time
}

// ListViolations returns geofence events, newest first, and the total number
// of events matching the filter (ignoring limit/offset).
func (q *Queries) ListViolations(ctx context.Context, f ViolationFilter, page Page) ([]domain.Violation, int, error) {
	var where whereBuilder
	if f.VehicleID != "" {
		where.add("e.vehicle_id = $%d", f.VehicleID)
	}
	if f.GeofenceID != "" {
		where.add("e.geofence_id = $%d", f.GeofenceID)
	}
	if f.Start != nil {
		where.add("e.occurred_at >= $%d", *f.Start)
	}
	if f.End != nil {
		where.add("e.occurred_at <= $%d", *f.End)
	}

	var total int
	if err := q.db.QueryRow(ctx, "SELECT count(*) FROM geofence_events e"+where.sql(), where.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count violations: %w", err)
	}

	limit, args := limitClause(page, where.args)
	rows, err := q.db.Query(ctx, `
		SELECT e.id, e.vehicle_id, v.vehicle_number, e.geofence_id, g.name, g.category,
		       e.event_type, ST_Y(e.point), ST_X(e.point), e.occurred_at
		FROM geofence_events e
		JOIN vehicles v ON v.id = e.vehicle_id
		JOIN geofences g ON g.id = e.geofence_id`+where.sql()+`
		ORDER BY e.occurred_at DESC, e.id DESC`+limit, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list violations: %w", err)
	}
	defer rows.Close()

	out := []domain.Violation{}
	for rows.Next() {
		var (
			v         domain.Violation
			cat, kind string
		)
		if err := rows.Scan(&v.ID, &v.VehicleID, &v.VehicleNumber, &v.GeofenceID, &v.GeofenceName, &cat,
			&kind, &v.Latitude, &v.Longitude, &v.Timestamp); err != nil {
			return nil, 0, fmt.Errorf("scan violation: %w", err)
		}
		v.Category = domain.Category(cat)
		v.EventType = domain.EventType(kind)
		out = append(out, v)
	}
	return out, total, rows.Err()
}
