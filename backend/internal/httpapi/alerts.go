package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/store"
)

// POST /alerts/configure
func (a *API) configureAlert(r *http.Request) (int, any, error) {
	ctx := r.Context()
	var in domain.AlertRuleInput
	if err := decodeJSON(r, &in); err != nil {
		return 0, nil, err
	}
	if err := in.Validate(); err != nil {
		return 0, nil, err
	}

	// Check references up front so the error names what is missing.
	if _, err := a.store.GetFenceRef(ctx, in.GeofenceID); err != nil {
		return 0, nil, err
	}
	if in.VehicleID != nil {
		if _, err := a.store.GetVehicle(ctx, *in.VehicleID); err != nil {
			return 0, nil, err
		}
	}

	rule := domain.AlertRule{
		ID:         domain.NewID(domain.PrefixAlertRule),
		GeofenceID: in.GeofenceID,
		VehicleID:  in.VehicleID,
		EventType:  domain.RuleEventType(in.EventType),
		Status:     domain.StatusActive,
	}
	if err := a.store.CreateRule(ctx, &rule); err != nil {
		return 0, nil, err
	}

	return http.StatusCreated, map[string]any{
		"alert_id":    rule.ID,
		"geofence_id": rule.GeofenceID,
		"vehicle_id":  rule.VehicleID, // null when the rule applies to all vehicles
		"event_type":  rule.EventType,
		"status":      rule.Status,
	}, nil
}

// GET /alerts?geofence_id=&vehicle_id=&limit=&offset=
func (a *API) listAlerts(r *http.Request) (int, any, error) {
	page, err := pageParams(r, 0, maxListLimit)
	if err != nil {
		return 0, nil, err
	}
	filter := store.RuleFilter{
		GeofenceID: queryParam(r, "geofence_id"),
		VehicleID:  queryParam(r, "vehicle_id"),
	}
	rules, total, err := a.store.ListRules(r.Context(), filter, page)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{
		"alerts":      rules,
		"total_count": total,
	}, nil
}

// PATCH /alerts/{alert_id} — turn a rule on or off. (Addition to the spec,
// so rules can be managed from the dashboard.)
func (a *API) updateAlertStatus(r *http.Request) (int, any, error) {
	ctx := r.Context()
	id := chi.URLParam(r, "alert_id")

	var in domain.AlertStatusInput
	if err := decodeJSON(r, &in); err != nil {
		return 0, nil, err
	}
	if err := in.Validate(); err != nil {
		return 0, nil, err
	}
	if err := a.store.SetRuleStatus(ctx, id, in.Status); err != nil {
		return 0, nil, err
	}
	rule, err := a.store.GetRule(ctx, id)
	if err != nil {
		return 0, nil, err
	}
	return http.StatusOK, rule, nil
}

// DELETE /alerts/{alert_id} (addition to the spec)
func (a *API) deleteAlert(r *http.Request) (int, any, error) {
	id := chi.URLParam(r, "alert_id")
	if err := a.store.DeleteRule(r.Context(), id); err != nil {
		return 0, nil, err
	}
	return http.StatusOK, map[string]any{"alert_id": id, "deleted": true}, nil
}
