package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
)

// Stored formats (docs/architecture.md, Timestamp convention).
const (
	dateLayout      = "2006-01-02"
	timestampLayout = "2006-01-02 15:04:05"
)

// SetupRepository implements app.SetupRepository on SQLite.
type SetupRepository struct{ db *sql.DB }

// NewSetupRepository returns the setup repository for db.
func NewSetupRepository(db *sql.DB) *SetupRepository { return &SetupRepository{db: db} }

func (r *SetupRepository) SetupCompleted(ctx context.Context) (bool, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM settings`).Scan(&n); err != nil {
		return false, fmt.Errorf("count settings: %w", err)
	}
	return n > 0, nil
}

func (r *SetupRepository) LoadSettings(ctx context.Context) (app.Settings, error) {
	var s app.Settings
	var openingDate, completedAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT community_name, house_prefix, house_count, opening_date, setup_completed_at
		 FROM settings WHERE id = 1`,
	).Scan(&s.CommunityName, &s.HousePrefix, &s.HouseCount, &openingDate, &completedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return app.Settings{}, app.ErrNotFound
	}
	if err != nil {
		return app.Settings{}, fmt.Errorf("read settings: %w", err)
	}
	if s.OpeningDate, err = time.ParseInLocation(dateLayout, openingDate, time.Local); err != nil {
		return app.Settings{}, fmt.Errorf("read settings: opening_date: %w", err)
	}
	if s.SetupCompletedAt, err = time.ParseInLocation(timestampLayout, completedAt, time.Local); err != nil {
		return app.Settings{}, fmt.Errorf("read settings: setup_completed_at: %w", err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, kind, opening_balance_cents, sort_order
		 FROM accounts ORDER BY sort_order`)
	if err != nil {
		return app.Settings{}, fmt.Errorf("read accounts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a app.Account
		var kind string
		var balance int64
		if err := rows.Scan(&a.ID, &a.Name, &kind, &balance, &a.SortOrder); err != nil {
			return app.Settings{}, fmt.Errorf("read accounts: %w", err)
		}
		a.Kind, a.OpeningBalance = app.AccountKind(kind), domain.Money(balance)
		s.Accounts = append(s.Accounts, a)
	}
	if err := rows.Err(); err != nil {
		return app.Settings{}, fmt.Errorf("read accounts: %w", err)
	}
	return s, nil
}

func (r *SetupRepository) UpdateCommunityName(ctx context.Context, name string, now time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE settings SET community_name = ?, updated_at = ? WHERE id = 1`,
		name, now.Format(timestampLayout))
	return requireRow(res, err, "update community name")
}

func (r *SetupRepository) RenameAccount(ctx context.Context, id int64, name string, now time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE accounts SET name = ?, updated_at = ? WHERE id = ?`,
		name, now.Format(timestampLayout), id)
	return requireRow(res, err, "rename account")
}

func (r *SetupRepository) UpdateOpeningBalance(ctx context.Context, id int64, balance domain.Money, now time.Time) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE accounts SET opening_balance_cents = ?, updated_at = ? WHERE id = ?`,
		int64(balance), now.Format(timestampLayout), id)
	return requireRow(res, err, "update opening balance")
}

// requireRow turns an UPDATE that touched no row into ErrNotFound.
func requireRow(res sql.Result, err error, op string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if n == 0 {
		return fmt.Errorf("%s: %w", op, app.ErrNotFound)
	}
	return nil
}

func (r *SetupRepository) SaveSetup(ctx context.Context, s app.NewSetup) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save setup: %w", err)
	}
	defer tx.Rollback()

	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM settings`).Scan(&existing); err != nil {
		return fmt.Errorf("save setup: %w", err)
	}
	if existing > 0 {
		return fmt.Errorf("save setup: setup already completed: %w", app.ErrConflict)
	}

	now := s.Now.Format(timestampLayout)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO settings (id, community_name, house_prefix, house_count, opening_date, setup_completed_at, updated_at)
		 VALUES (1, ?, ?, ?, ?, ?, ?)`,
		s.CommunityName, s.HousePrefix, s.HouseCount, s.OpeningDate.Format(dateLayout), now, now,
	); err != nil {
		return fmt.Errorf("save setup: settings: %w", err)
	}
	for i, a := range s.Accounts {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO accounts (name, kind, opening_balance_cents, sort_order, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			a.Name, string(a.Kind), int64(a.OpeningBalance), i, now, now,
		); err != nil {
			return fmt.Errorf("save setup: account %d: %w", i, err)
		}
	}
	for _, h := range s.Houses {
		var covered any // NULL when nothing is covered
		if h.Opening.CoveredThrough != nil {
			covered = h.Opening.CoveredThrough.String()
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO houses (number, code, opening_covered_through, opening_credit_cents, opening_arrears_cents, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			h.Number, h.Code, covered, int64(h.Opening.Credit), int64(h.Opening.Arrears), now, now,
		); err != nil {
			return fmt.Errorf("save setup: house %d: %w", h.Number, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO fee_rates (effective_month, amount_cents, policy, recorded_at)
		 VALUES (?, ?, 'initial', ?)`,
		domain.Of(s.OpeningDate).String(), int64(s.Fee), now,
	); err != nil {
		return fmt.Errorf("save setup: fee rate: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save setup: commit: %w", err)
	}
	return nil
}
