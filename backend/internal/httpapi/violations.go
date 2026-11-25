package httpapi

import (
	"net/http"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/store"
)

const (
	defaultHistoryLimit = 50
	maxHistoryLimit     = 500
)

// GET /violations/history?vehicle_id=&geofence_id=&start_date=&end_date=&limit=&offset=
func (a *API) violationHistory(r *http.Request) (int, any, error) {
	page, err := pageParams(r, defaultHistoryLimit, maxHistoryLimit)
	if err != nil {
		return 0, nil, err
	}
	start, err := timeParam(r, "start_date")
	if err != nil {
		return 0, nil, err
	}
	end, err := timeParam(r, "end_date")
	if err != nil {
		return 0, nil, err
	}
	if start != nil && end != nil && end.Before(*start) {
		return 0, nil, badRequest("end_date must be after start_date")
	}

	filter := store.ViolationFilter{
		VehicleID:  queryParam(r, "vehicle_id"),
		GeofenceID: queryParam(r, "geofence_id"),
		Start:      start,
		End:        end,
	}
	violations, total, err := a.store.ListViolations(r.Context(), filter, page)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{
		"violations":  violations,
		"total_count": total,
		"limit":       page.Limit,
		"offset":      page.Offset,
	}, nil
}
