package httpapi

import (
	"net/http"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

const maxListLimit = 500

// POST /geofences
func (a *API) createGeofence(r *http.Request) (int, any, error) {
	var in domain.GeofenceInput
	if err := decodeJSON(r, &in); err != nil {
		return 0, nil, err
	}
	if err := in.Validate(); err != nil {
		return 0, nil, err
	}

	geoJSON, err := domain.RingToGeoJSON(in.Coordinates)
	if err != nil {
		return 0, nil, err
	}
	// The structural checks above can't catch shapes like a "bow tie" whose
	// edges cross; PostGIS can, and tells us where.
	valid, reason, err := a.store.CheckPolygon(r.Context(), geoJSON)
	if err != nil {
		return 0, nil, err
	}
	if !valid {
		return 0, nil, domain.Invalid("coordinates", "polygon is not valid: %s (edges must not cross each other)", reason)
	}

	g := domain.Geofence{
		ID:          domain.NewID(domain.PrefixGeofence),
		Name:        in.Name,
		Description: in.Description,
		Coordinates: in.Coordinates,
		Category:    domain.Category(in.Category),
		Status:      domain.StatusActive,
	}
	if err := a.store.CreateGeofence(r.Context(), &g, geoJSON); err != nil {
		return 0, nil, err
	}

	return http.StatusCreated, map[string]any{
		"id":     g.ID,
		"name":   g.Name,
		"status": g.Status,
	}, nil
}

// GET /geofences?category=&limit=&offset=
func (a *API) listGeofences(r *http.Request) (int, any, error) {
	category := queryParam(r, "category")
	if category != "" && !domain.Category(category).IsValid() {
		return 0, nil, badRequest("category must be one of delivery_zone, restricted_zone, toll_zone, customer_area")
	}
	page, err := pageParams(r, 0, maxListLimit)
	if err != nil {
		return 0, nil, err
	}

	fences, total, err := a.store.ListGeofences(r.Context(), category, page)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{
		"geofences":   fences,
		"total_count": total,
	}, nil
}
