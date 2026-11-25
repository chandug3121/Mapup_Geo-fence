package store

import (
	"context"
	"fmt"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

// CheckPolygon asks PostGIS whether a GeoJSON polygon is valid (for example,
// not self-intersecting) and, if not, why.
func (q *Queries) CheckPolygon(ctx context.Context, geoJSON string) (valid bool, reason string, err error) {
	err = q.db.QueryRow(ctx, `
		SELECT ST_IsValid(g), ST_IsValidReason(g)
		FROM (SELECT ST_SetSRID(ST_GeomFromGeoJSON($1::text), 4326) AS g) AS input`,
		geoJSON,
	).Scan(&valid, &reason)
	if err != nil {
		return false, "", fmt.Errorf("check polygon: %w", err)
	}
	return valid, reason, nil
}

// CreateGeofence inserts g. g.Coordinates must already be validated and
// geoJSON must be the same ring in GeoJSON (lng, lat) order.
func (q *Queries) CreateGeofence(ctx context.Context, g *domain.Geofence, geoJSON string) error {
	err := q.db.QueryRow(ctx, `
		INSERT INTO geofences (id, name, description, category, boundary, status)
		VALUES ($1, $2, $3, $4, ST_SetSRID(ST_GeomFromGeoJSON($5::text), 4326), $6)
		RETURNING created_at`,
		g.ID, g.Name, g.Description, string(g.Category), geoJSON, g.Status,
	).Scan(&g.CreatedAt)
	if err != nil {
		if pgErrorCode(err) == pgCheckViolation {
			return domain.Invalid("coordinates", "polygon is not valid")
		}
		return fmt.Errorf("insert geofence: %w", err)
	}
	return nil
}

// ListGeofences returns geofences, newest first, optionally filtered by
// category, plus the total count ignoring paging.
func (q *Queries) ListGeofences(ctx context.Context, category string, page Page) ([]domain.Geofence, int, error) {
	var where whereBuilder
	if category != "" {
		where.add("category = $%d", category)
	}

	var total int
	if err := q.db.QueryRow(ctx, "SELECT count(*) FROM geofences"+where.sql(), where.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count geofences: %w", err)
	}

	limit, args := limitClause(page, where.args)
	rows, err := q.db.Query(ctx, `
		SELECT id, name, description, category, ST_AsGeoJSON(boundary), status, created_at
		FROM geofences`+where.sql()+`
		ORDER BY created_at DESC, id`+limit, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list geofences: %w", err)
	}
	defer rows.Close()

	fences := []domain.Geofence{}
	for rows.Next() {
		var (
			g       domain.Geofence
			cat     string
			geoJSON string
		)
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &cat, &geoJSON, &g.Status, &g.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan geofence: %w", err)
		}
		g.Category = domain.Category(cat)
		if g.Coordinates, err = domain.RingFromGeoJSON(geoJSON); err != nil {
			return nil, 0, fmt.Errorf("geofence %s: %w", g.ID, err)
		}
		fences = append(fences, g)
	}
	return fences, total, rows.Err()
}

// GetFenceRef returns a geofence's id, name and category.
func (q *Queries) GetFenceRef(ctx context.Context, id string) (domain.FenceRef, error) {
	var (
		f        domain.FenceRef
		category string
	)
	err := q.db.QueryRow(ctx, "SELECT id, name, category FROM geofences WHERE id = $1", id).
		Scan(&f.ID, &f.Name, &category)
	if err != nil {
		if isNoRows(err) {
			return domain.FenceRef{}, domain.NotFound("geofence", id)
		}
		return domain.FenceRef{}, fmt.Errorf("get geofence: %w", err)
	}
	f.Category = domain.Category(category)
	return f, nil
}

// FencesCovering returns the active geofences that contain the point,
// including points exactly on an edge or vertex.
//
// ST_Covers (not ST_Contains) is deliberate: ST_Contains is false for points
// on the boundary. The GiST index on boundary is used for the bounding-box
// pre-filter, so only nearby candidates get the exact polygon test.
func (q *Queries) FencesCovering(ctx context.Context, lat, lng float64) ([]domain.FenceRef, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, name, category
		FROM geofences
		WHERE status = 'active'
		  AND ST_Covers(boundary, ST_SetSRID(ST_MakePoint($1, $2), 4326))
		ORDER BY created_at, id`,
		lng, lat, // ST_MakePoint takes (x, y) = (longitude, latitude)
	)
	if err != nil {
		return nil, fmt.Errorf("find covering geofences: %w", err)
	}
	return scanFenceRefs(rows)
}
