package session

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound  = errors.New("session not found")
	ErrSlotTaken = errors.New("telescope is already booked for that date")
)

// Repository is the only code that touches the session database.
type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const selectCols = `id, telescope_id, observer, to_char(session_date, 'YYYY-MM-DD'), target, status`

// Migrate creates the schema. Note there is NO foreign key on telescope_id:
// the telescopes table lives in another service's database.
//
// The partial unique index guarantees "one active booking per telescope per
// day" even under concurrent requests (no check-then-insert race).
func (r *Repository) Migrate(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS sessions (
			id           TEXT PRIMARY KEY,
			telescope_id TEXT        NOT NULL,
			observer     TEXT        NOT NULL,
			session_date DATE        NOT NULL,
			target       TEXT        NOT NULL DEFAULT '',
			status       TEXT        NOT NULL,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_booked_slot
		ON sessions (telescope_id, session_date) WHERE status = 'BOOKED'`)
	return err
}

func (r *Repository) Create(ctx context.Context, s Session) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (id, telescope_id, observer, session_date, target, status)
		 VALUES ($1, $2, $3, $4::text::date, $5, $6)`,
		s.ID, s.TelescopeID, s.Observer, s.Date, s.Target, s.Status)
	if isUniqueViolation(err) {
		return ErrSlotTaken
	}
	return err
}

func (r *Repository) Get(ctx context.Context, id string) (Session, error) {
	s, err := scan(r.db.QueryRowContext(ctx, `SELECT `+selectCols+` FROM sessions WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return s, err
}

// List returns all sessions, optionally filtered by telescope id.
func (r *Repository) List(ctx context.Context, telescopeID string) ([]Session, error) {
	query := `SELECT ` + selectCols + ` FROM sessions ORDER BY session_date, created_at`
	var args []any
	if telescopeID != "" {
		query = `SELECT ` + selectCols + ` FROM sessions WHERE telescope_id = $1 ORDER BY session_date, created_at`
		args = append(args, telescopeID)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Session
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) Update(ctx context.Context, s Session) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE sessions
			SET telescope_id = $2, observer = $3, session_date = $4::text::date, target = $5, status = $6
		  WHERE id = $1`,
		s.ID, s.TelescopeID, s.Observer, s.Date, s.Target, s.Status)
	if isUniqueViolation(err) {
		return ErrSlotTaken
	}
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

type scanner interface{ Scan(dest ...any) error }

func scan(sc scanner) (Session, error) {
	var s Session
	err := sc.Scan(&s.ID, &s.TelescopeID, &s.Observer, &s.Date, &s.Target, &s.Status)
	return s, err
}

func expectOneRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// isUniqueViolation detects PostgreSQL error 23505 (unique_violation).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
