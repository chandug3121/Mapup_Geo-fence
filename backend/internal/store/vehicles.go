package store

import (
	"context"
	"fmt"
	"time"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

const vehicleColumns = "id, vehicle_number, driver_name, vehicle_type, phone, status, created_at"

type rowScanner interface {
	Scan(dest ...any) error
}

func scanVehicle(row rowScanner) (domain.Vehicle, error) {
	var v domain.Vehicle
	err := row.Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Status, &v.CreatedAt)
	return v, err
}

// CreateVehicle inserts v. A duplicate vehicle number returns ErrConflict.
func (q *Queries) CreateVehicle(ctx context.Context, v *domain.Vehicle) error {
	err := q.db.QueryRow(ctx, `
		INSERT INTO vehicles (id, vehicle_number, driver_name, vehicle_type, phone, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at`,
		v.ID, v.VehicleNumber, v.DriverName, v.VehicleType, v.Phone, v.Status,
	).Scan(&v.CreatedAt)
	if err != nil {
		if pgErrorCode(err) == pgUniqueViolation {
			return fmt.Errorf("vehicle number %s is already registered: %w", v.VehicleNumber, domain.ErrConflict)
		}
		return fmt.Errorf("insert vehicle: %w", err)
	}
	return nil
}

// ListVehicles returns vehicles, newest first, plus the total count.
func (q *Queries) ListVehicles(ctx context.Context, page Page) ([]domain.Vehicle, int, error) {
	var total int
	if err := q.db.QueryRow(ctx, "SELECT count(*) FROM vehicles").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count vehicles: %w", err)
	}

	limit, args := limitClause(page, nil)
	rows, err := q.db.Query(ctx, "SELECT "+vehicleColumns+" FROM vehicles ORDER BY created_at DESC, id"+limit, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list vehicles: %w", err)
	}
	defer rows.Close()

	vehicles := []domain.Vehicle{}
	for rows.Next() {
		v, err := scanVehicle(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan vehicle: %w", err)
		}
		vehicles = append(vehicles, v)
	}
	return vehicles, total, rows.Err()
}

// GetVehicle returns one vehicle or ErrNotFound.
func (q *Queries) GetVehicle(ctx context.Context, id string) (domain.Vehicle, error) {
	v, err := scanVehicle(q.db.QueryRow(ctx, "SELECT "+vehicleColumns+" FROM vehicles WHERE id = $1", id))
	if err != nil {
		if isNoRows(err) {
			return domain.Vehicle{}, domain.NotFound("vehicle", id)
		}
		return domain.Vehicle{}, fmt.Errorf("get vehicle: %w", err)
	}
	return v, nil
}

// LockVehicle loads a vehicle with SELECT ... FOR UPDATE. Inside a
// transaction this serialises concurrent location updates for the same
// vehicle (so an entry can't be detected twice), while updates for
// different vehicles still run in parallel.
func (q *Queries) LockVehicle(ctx context.Context, id string) (domain.Vehicle, *time.Time, error) {
	var (
		v            domain.Vehicle
		lastRecorded *time.Time
	)
	err := q.db.QueryRow(ctx,
		"SELECT "+vehicleColumns+", last_recorded_at FROM vehicles WHERE id = $1 FOR UPDATE", id,
	).Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Status, &v.CreatedAt, &lastRecorded)
	if err != nil {
		if isNoRows(err) {
			return domain.Vehicle{}, nil, domain.NotFound("vehicle", id)
		}
		return domain.Vehicle{}, nil, fmt.Errorf("lock vehicle: %w", err)
	}
	return v, lastRecorded, nil
}

// SetLastRecordedAt remembers the newest location timestamp that was applied.
func (q *Queries) SetLastRecordedAt(ctx context.Context, vehicleID string, ts time.Time) error {
	_, err := q.db.Exec(ctx, "UPDATE vehicles SET last_recorded_at = $2 WHERE id = $1", vehicleID, ts)
	if err != nil {
		return fmt.Errorf("update last_recorded_at: %w", err)
	}
	return nil
}
