package store

import (
	"context"
	"fmt"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

const ruleSelect = `
	SELECT r.id, r.geofence_id, g.name, r.vehicle_id, v.vehicle_number, r.event_type, r.status, r.created_at
	FROM alert_rules r
	JOIN geofences g ON g.id = r.geofence_id
	LEFT JOIN vehicles v ON v.id = r.vehicle_id`

func scanRule(row rowScanner) (domain.AlertRule, error) {
	var (
		r    domain.AlertRule
		kind string
	)
	err := row.Scan(&r.ID, &r.GeofenceID, &r.GeofenceName, &r.VehicleID, &r.VehicleNumber, &kind, &r.Status, &r.CreatedAt)
	r.EventType = domain.RuleEventType(kind)
	return r, err
}

// CreateRule inserts an alert rule. An identical existing rule returns ErrConflict.
func (q *Queries) CreateRule(ctx context.Context, r *domain.AlertRule) error {
	err := q.db.QueryRow(ctx, `
		INSERT INTO alert_rules (id, geofence_id, vehicle_id, event_type, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`,
		r.ID, r.GeofenceID, r.VehicleID, string(r.EventType), r.Status,
	).Scan(&r.CreatedAt)
	if err != nil {
		switch pgErrorCode(err) {
		case pgUniqueViolation:
			return fmt.Errorf("an identical alert rule already exists: %w", domain.ErrConflict)
		case pgForeignKeyViolation:
			return fmt.Errorf("geofence or vehicle %w", domain.ErrNotFound)
		}
		return fmt.Errorf("insert alert rule: %w", err)
	}
	return nil
}

// RuleFilter narrows GET /alerts. Empty fields mean "any".
type RuleFilter struct {
	GeofenceID string
	VehicleID  string
}

// ListRules returns alert rules, newest first, plus the total count.
func (q *Queries) ListRules(ctx context.Context, f RuleFilter, page Page) ([]domain.AlertRule, int, error) {
	var where whereBuilder
	if f.GeofenceID != "" {
		where.add("r.geofence_id = $%d", f.GeofenceID)
	}
	if f.VehicleID != "" {
		where.add("r.vehicle_id = $%d", f.VehicleID)
	}

	var total int
	if err := q.db.QueryRow(ctx, "SELECT count(*) FROM alert_rules r"+where.sql(), where.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count alert rules: %w", err)
	}

	limit, args := limitClause(page, where.args)
	rows, err := q.db.Query(ctx, ruleSelect+where.sql()+" ORDER BY r.created_at DESC, r.id"+limit, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()

	rules := []domain.AlertRule{}
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan alert rule: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, total, rows.Err()
}

// GetRule returns one alert rule or ErrNotFound.
func (q *Queries) GetRule(ctx context.Context, id string) (domain.AlertRule, error) {
	r, err := scanRule(q.db.QueryRow(ctx, ruleSelect+" WHERE r.id = $1", id))
	if err != nil {
		if isNoRows(err) {
			return domain.AlertRule{}, domain.NotFound("alert", id)
		}
		return domain.AlertRule{}, fmt.Errorf("get alert rule: %w", err)
	}
	return r, nil
}

// SetRuleStatus turns a rule on or off.
func (q *Queries) SetRuleStatus(ctx context.Context, id, status string) error {
	tag, err := q.db.Exec(ctx, "UPDATE alert_rules SET status = $2 WHERE id = $1", id, status)
	if err != nil {
		return fmt.Errorf("update alert rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("alert", id)
	}
	return nil
}

// DeleteRule removes a rule.
func (q *Queries) DeleteRule(ctx context.Context, id string) error {
	tag, err := q.db.Exec(ctx, "DELETE FROM alert_rules WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.NotFound("alert", id)
	}
	return nil
}

// MatchingRule is the part of a rule needed to decide whether it fires.
type MatchingRule struct {
	ID         string
	GeofenceID string
	EventType  domain.RuleEventType
}

// ActiveRulesFor returns active rules on any of the given geofences that
// apply to this vehicle (rules for this vehicle, or for all vehicles).
func (q *Queries) ActiveRulesFor(ctx context.Context, vehicleID string, geofenceIDs []string) ([]MatchingRule, error) {
	if len(geofenceIDs) == 0 {
		return nil, nil
	}
	rows, err := q.db.Query(ctx, `
		SELECT id, geofence_id, event_type
		FROM alert_rules
		WHERE status = 'active'
		  AND geofence_id = ANY($1)
		  AND (vehicle_id IS NULL OR vehicle_id = $2)
		ORDER BY created_at, id`,
		geofenceIDs, vehicleID,
	)
	if err != nil {
		return nil, fmt.Errorf("find alert rules: %w", err)
	}
	defer rows.Close()

	var rules []MatchingRule
	for rows.Next() {
		var (
			r    MatchingRule
			kind string
		)
		if err := rows.Scan(&r.ID, &r.GeofenceID, &kind); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		r.EventType = domain.RuleEventType(kind)
		rules = append(rules, r)
	}
	return rules, rows.Err()
}
