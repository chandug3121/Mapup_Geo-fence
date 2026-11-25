// Package tracking turns raw location updates into geofence entry/exit
// events and real-time alerts.
package tracking

import "github.com/chandug3121/mapup_geo-fence/backend/internal/domain"

// Diff compares the geofences a vehicle was inside before this update with
// the geofences that cover its new position.
//
//	entered = current  - previous
//	exited  = previous - current
//
// Order follows the input slices so results are deterministic.
func Diff(previous, current []domain.FenceRef) (entered, exited []domain.FenceRef) {
	wasInside := make(map[string]bool, len(previous))
	for _, f := range previous {
		wasInside[f.ID] = true
	}
	isInside := make(map[string]bool, len(current))
	for _, f := range current {
		isInside[f.ID] = true
	}

	for _, f := range current {
		if !wasInside[f.ID] {
			entered = append(entered, f)
		}
	}
	for _, f := range previous {
		if !isInside[f.ID] {
			exited = append(exited, f)
		}
	}
	return entered, exited
}
