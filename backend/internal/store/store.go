// Package store contains every SQL query. Nothing outside this package
// writes SQL, and nothing in here knows about HTTP.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is satisfied by both *pgxpool.Pool and pgx.Tx, so the same query
// methods work inside and outside a transaction.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Queries runs SQL against a DBTX.
type Queries struct {
	db DBTX
}

// Store is the entry point: Queries bound to the pool, plus transactions.
type Store struct {
	*Queries
	pool *pgxpool.Pool
}

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{Queries: &Queries{db: pool}, pool: pool}
}

// Ping checks the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// InTx runs fn inside a transaction. It commits when fn returns nil and
// rolls back otherwise (including on panic).
func (s *Store) InTx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit

	if err := fn(&Queries{db: tx}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// Page limits a list query. Limit 0 means "no limit".
type Page struct {
	Limit  int
	Offset int
}

// Postgres error codes we translate into domain errors.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
)

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// limitClause appends LIMIT/OFFSET placeholders for p to args.
func limitClause(p Page, args []any) (string, []any) {
	clause := ""
	if p.Limit > 0 {
		args = append(args, p.Limit)
		clause += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if p.Offset > 0 {
		args = append(args, p.Offset)
		clause += fmt.Sprintf(" OFFSET $%d", len(args))
	}
	return clause, args
}

// whereBuilder collects parameterised conditions. Values always go through
// placeholders ($1, $2, ...), never into the SQL string.
type whereBuilder struct {
	conds []string
	args  []any
}

func (w *whereBuilder) add(condition string, value any) {
	w.args = append(w.args, value)
	w.conds = append(w.conds, fmt.Sprintf(condition, len(w.args)))
}

func (w *whereBuilder) sql() string {
	if len(w.conds) == 0 {
		return ""
	}
	out := " WHERE " + w.conds[0]
	for _, c := range w.conds[1:] {
		out += " AND " + c
	}
	return out
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// scanFenceRefs reads rows of (id, name, category) and closes rows.
func scanFenceRefs(rows pgx.Rows) ([]domain.FenceRef, error) {
	defer rows.Close()
	fences := []domain.FenceRef{}
	for rows.Next() {
		var (
			f   domain.FenceRef
			cat string
		)
		if err := rows.Scan(&f.ID, &f.Name, &cat); err != nil {
			return nil, fmt.Errorf("scan geofence: %w", err)
		}
		f.Category = domain.Category(cat)
		fences = append(fences, f)
	}
	return fences, rows.Err()
}
