package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

// POST /vehicles
func (a *API) createVehicle(r *http.Request) (int, any, error) {
	var in domain.VehicleInput
	if err := decodeJSON(r, &in); err != nil {
		return 0, nil, err
	}
	if err := in.Validate(); err != nil {
		return 0, nil, err
	}

	v := domain.Vehicle{
		ID:            domain.NewID(domain.PrefixVehicle),
		VehicleNumber: in.VehicleNumber,
		DriverName:    in.DriverName,
		VehicleType:   in.VehicleType,
		Phone:         in.Phone,
		Status:        domain.StatusActive,
	}
	if err := a.store.CreateVehicle(r.Context(), &v); err != nil {
		return 0, nil, err
	}

	return http.StatusCreated, map[string]any{
		"id":             v.ID,
		"vehicle_number": v.VehicleNumber,
		"status":         v.Status,
	}, nil
}

// GET /vehicles?limit=&offset=
func (a *API) listVehicles(r *http.Request) (int, any, error) {
	page, err := pageParams(r, 0, maxListLimit)
	if err != nil {
		return 0, nil, err
	}
	vehicles, total, err := a.store.ListVehicles(r.Context(), page)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{
		"vehicles":    vehicles,
		"total_count": total,
	}, nil
}

type insideFence struct {
	GeofenceID   string          `json:"geofence_id"`
	GeofenceName string          `json:"geofence_name"`
	Category     domain.Category `json:"category"`
	Status       string          `json:"status"`
}

// POST /vehicles/location
func (a *API) updateLocation(r *http.Request) (int, any, error) {
	var in domain.LocationInput
	if err := decodeJSON(r, &in); err != nil {
		return 0, nil, err
	}
	update, err := in.Validate()
	if err != nil {
		return 0, nil, err
	}

	res, err := a.tracking.ProcessLocation(r.Context(), update)
	if err != nil {
		return 0, nil, err
	}

	inside := make([]insideFence, 0, len(res.Inside))
	for _, f := range res.Inside {
		inside = append(inside, insideFence{GeofenceID: f.ID, GeofenceName: f.Name, Category: f.Category, Status: "inside"})
	}
	return http.StatusOK, map[string]any{
		"vehicle_id":        res.VehicleID,
		"location_updated":  true,
		"current_geofences": inside,
		"events":            eventSummary(res.Entered, res.Exited),
		"alerts_triggered":  len(res.Alerts),
		"stale":             res.Stale,
	}, nil
}

type eventInfo struct {
	GeofenceID   string           `json:"geofence_id"`
	GeofenceName string           `json:"geofence_name"`
	EventType    domain.EventType `json:"event_type"`
}

func eventSummary(entered, exited []domain.FenceRef) []eventInfo {
	out := make([]eventInfo, 0, len(entered)+len(exited))
	for _, f := range entered {
		out = append(out, eventInfo{GeofenceID: f.ID, GeofenceName: f.Name, EventType: domain.EventEntry})
	}
	for _, f := range exited {
		out = append(out, eventInfo{GeofenceID: f.ID, GeofenceName: f.Name, EventType: domain.EventExit})
	}
	return out
}

type currentFence struct {
	GeofenceID   string          `json:"geofence_id"`
	GeofenceName string          `json:"geofence_name"`
	Category     domain.Category `json:"category"`
}

// GET /vehicles/location/{vehicle_id}
func (a *API) getVehicleLocation(r *http.Request) (int, any, error) {
	ctx := r.Context()
	id := chi.URLParam(r, "vehicle_id")

	vehicle, err := a.store.GetVehicle(ctx, id)
	if err != nil {
		return 0, nil, err
	}
	position, err := a.store.LatestLocation(ctx, id)
	if err != nil {
		return 0, nil, err
	}
	fences, err := a.store.CurrentFences(ctx, id)
	if err != nil {
		return 0, nil, err
	}

	current := make([]currentFence, 0, len(fences))
	for _, f := range fences {
		current = append(current, currentFence{GeofenceID: f.ID, GeofenceName: f.Name, Category: f.Category})
	}
	return http.StatusOK, map[string]any{
		"vehicle_id":        vehicle.ID,
		"vehicle_number":    vehicle.VehicleNumber,
		"current_location":  position, // null if the vehicle has never reported a location
		"current_geofences": current,
	}, nil
}
