package telescope

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotFound is returned when no telescope matches the given id.
var ErrNotFound = errors.New("telescope not found")

// Repository is the only code that touches the telescope database.
type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// Migrate creates the schema if needed (idempotent, runs at start-up).
func (r *Repository) Migrate(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS telescopes (
			id          TEXT PRIMARY KEY,
			name        TEXT        NOT NULL,
			aperture_mm INTEGER     NOT NULL CHECK (aperture_mm > 0),
			type        TEXT        NOT NULL,
			status      TEXT        NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	return err
}

func (r *Repository) Create(ctx context.Context, t Telescope) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO telescopes (id, name, aperture_mm, type, status) VALUES ($1, $2, $3, $4, $5)`,
		t.ID, t.Name, t.ApertureMM, t.Type, t.Status)
	return err
}

func (r *Repository) Get(ctx context.Context, id string) (Telescope, error) {
	var t Telescope
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, aperture_mm, type, status FROM telescopes WHERE id = $1`, id).
		Scan(&t.ID, &t.Name, &t.ApertureMM, &t.Type, &t.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return Telescope{}, ErrNotFound
	}
	return t, err
}

func (r *Repository) List(ctx context.Context) ([]Telescope, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, aperture_mm, type, status FROM telescopes ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Telescope
	for rows.Next() {
		var t Telescope
		if err := rows.Scan(&t.ID, &t.Name, &t.ApertureMM, &t.Type, &t.Status); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) Update(ctx context.Context, t Telescope) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE telescopes SET name = $2, aperture_mm = $3, type = $4, status = $5 WHERE id = $1`,
		t.ID, t.Name, t.ApertureMM, t.Type, t.Status)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM telescopes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	return expectOneRow(res)
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
