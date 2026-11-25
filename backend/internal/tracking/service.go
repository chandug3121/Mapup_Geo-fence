package tracking

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/store"
)

// Publisher delivers alert payloads to connected clients. Publish must not
// block; the WebSocket hub satisfies this.
type Publisher interface {
	Publish(payload []byte)
}

// Service processes location updates.
type Service struct {
	store *store.Store
	pub   Publisher
	log   *slog.Logger
}

// NewService wires the tracking service.
func NewService(s *store.Store, pub Publisher, log *slog.Logger) *Service {
	return &Service{store: s, pub: pub, log: log}
}

// Result is what POST /vehicles/location returns.
type Result struct {
	VehicleID string
	Inside    []domain.FenceRef
	Entered   []domain.FenceRef
	Exited    []domain.FenceRef
	Alerts    []domain.AlertMessage
	// Stale is true when the update was older than one already applied: it
	// is stored as history but did not change geofence state.
	Stale bool
}

// ProcessLocation stores a location update, detects geofence entries and
// exits, saves them, fires matching alert rules and returns the geofences
// the vehicle is now inside.
//
// Everything that touches the database runs in one transaction. Alerts are
// published only after the commit, so clients never hear about an event
// that was rolled back, and publishing never blocks the HTTP response.
func (s *Service) ProcessLocation(ctx context.Context, u domain.LocationUpdate) (Result, error) {
	res := Result{VehicleID: u.VehicleID}
	var payloads [][]byte

	err := s.store.InTx(ctx, func(q *store.Queries) error {
		vehicle, lastRecorded, err := q.LockVehicle(ctx, u.VehicleID)
		if err != nil {
			return err
		}
		if err := q.InsertLocation(ctx, u); err != nil {
			return err
		}

		previous, err := q.CurrentFences(ctx, u.VehicleID)
		if err != nil {
			return err
		}

		// A late update (older than one already applied) must not move the
		// vehicle backwards in time: keep it as history only.
		if lastRecorded != nil && u.Timestamp.Before(*lastRecorded) {
			res.Inside, res.Stale = previous, true
			return nil
		}

		current, err := q.FencesCovering(ctx, u.Latitude, u.Longitude)
		if err != nil {
			return err
		}
		res.Inside = current
		res.Entered, res.Exited = Diff(previous, current)

		events := make([]domain.GeofenceEvent, 0, len(res.Entered)+len(res.Exited))
		fences := make(map[string]domain.FenceRef, len(res.Entered)+len(res.Exited))
		for _, f := range res.Entered {
			if err := q.EnterFence(ctx, u.VehicleID, f.ID, u.Timestamp); err != nil {
				return err
			}
			events = append(events, newEvent(u, f.ID, domain.EventEntry))
			fences[f.ID] = f
		}
		for _, f := range res.Exited {
			if err := q.ExitFence(ctx, u.VehicleID, f.ID); err != nil {
				return err
			}
			events = append(events, newEvent(u, f.ID, domain.EventExit))
			fences[f.ID] = f
		}
		for _, e := range events {
			if err := q.InsertEvent(ctx, e); err != nil {
				return err
			}
		}
		if err := q.SetLastRecordedAt(ctx, u.VehicleID, u.Timestamp); err != nil {
			return err
		}

		if len(events) == 0 {
			return nil
		}
		rules, err := q.ActiveRulesFor(ctx, u.VehicleID, keys(fences))
		if err != nil {
			return err
		}

		for _, e := range events {
			ruleIDs := matchingRuleIDs(rules, e)
			if len(ruleIDs) == 0 {
				continue
			}
			msg := buildAlert(vehicle, fences[e.GeofenceID], e)
			payload, err := json.Marshal(msg)
			if err != nil {
				return fmt.Errorf("encode alert: %w", err)
			}
			if err := q.InsertNotification(ctx, msg.EventID, e.ID, ruleIDs, payload); err != nil {
				return err
			}
			res.Alerts = append(res.Alerts, msg)
			payloads = append(payloads, payload)
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	for _, p := range payloads {
		s.pub.Publish(p)
	}
	if len(res.Entered)+len(res.Exited) > 0 {
		s.log.Info("geofence events",
			"vehicle_id", u.VehicleID, "entered", len(res.Entered), "exited", len(res.Exited), "alerts", len(res.Alerts))
	}
	return res, nil
}

func newEvent(u domain.LocationUpdate, geofenceID string, kind domain.EventType) domain.GeofenceEvent {
	return domain.GeofenceEvent{
		ID:         domain.NewID(domain.PrefixViolation),
		VehicleID:  u.VehicleID,
		GeofenceID: geofenceID,
		Type:       kind,
		Latitude:   u.Latitude,
		Longitude:  u.Longitude,
		OccurredAt: u.Timestamp,
	}
}

// matchingRuleIDs returns the ids of rules that should fire for event e.
// Several rules can match one event (e.g. an "all vehicles" rule and a rule
// for this vehicle); they still produce a single alert.
func matchingRuleIDs(rules []store.MatchingRule, e domain.GeofenceEvent) []string {
	var ids []string
	for _, r := range rules {
		if r.GeofenceID == e.GeofenceID && r.EventType.Matches(e.Type) {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func buildAlert(v domain.Vehicle, f domain.FenceRef, e domain.GeofenceEvent) domain.AlertMessage {
	return domain.AlertMessage{
		EventID:   domain.NewID(domain.PrefixAlert),
		EventType: e.Type,
		Timestamp: e.OccurredAt,
		Vehicle: domain.AlertVehicle{
			VehicleID:     v.ID,
			VehicleNumber: v.VehicleNumber,
			DriverName:    v.DriverName,
		},
		Geofence: domain.AlertGeofence{
			GeofenceID:   f.ID,
			GeofenceName: f.Name,
			Category:     f.Category,
		},
		Location: domain.AlertLocation{Latitude: e.Latitude, Longitude: e.Longitude},
	}
}

func keys(m map[string]domain.FenceRef) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
