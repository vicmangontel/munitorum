package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
)

// ledgerStore implements app.LedgerStore, and so app.CategoryRepository, over a
// database or a transaction.
type ledgerStore struct{ q queryer }

// LedgerRepository implements app.LedgerRepository and app.CategoryRepository
// on SQLite.
type LedgerRepository struct {
	ledgerStore
	db *sql.DB
}

// NewLedgerRepository returns the ledger repository for db.
func NewLedgerRepository(db *sql.DB) *LedgerRepository {
	return &LedgerRepository{ledgerStore: ledgerStore{q: db}, db: db}
}

func (r *LedgerRepository) InTx(ctx context.Context, fn func(app.LedgerStore) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin ledger transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fn(ledgerStore{q: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit ledger transaction: %w", err)
	}
	return nil
}

func (s ledgerStore) Settings(ctx context.Context) (app.Settings, error) {
	return loadSettings(ctx, s.q)
}

func (s ledgerStore) Vendors(ctx context.Context) ([]app.Vendor, error) {
	return vendorStore{q: s.q}.Vendors(ctx)
}

func (s ledgerStore) Categories(ctx context.Context) ([]app.Category, error) {
	rows, err := s.q.QueryContext(ctx,
		`SELECT id, name, direction, system_key, active, sort_order FROM categories ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}
	defer rows.Close()

	var all []app.Category
	for rows.Next() {
		var c app.Category
		var direction string
		var systemKey sql.NullString
		if err := rows.Scan(&c.ID, &c.Name, &direction, &systemKey, &c.Active, &c.SortOrder); err != nil {
			return nil, fmt.Errorf("read categories: %w", err)
		}
		c.Direction, c.SystemKey = domain.MovementKind(direction), systemKey.String
		all = append(all, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read categories: %w", err)
	}
	return all, nil
}

func (s ledgerStore) Movements(ctx context.Context) ([]app.Movement, error) {
	rows, err := s.q.QueryContext(ctx,
		`SELECT id, date, kind, account_id, to_account_id, amount_cents, category_id, vendor_id,
		        description, notes, receipt_link, manual_receipt
		 FROM movements WHERE deleted_at IS NULL ORDER BY date, id`)
	if err != nil {
		return nil, fmt.Errorf("read movements: %w", err)
	}
	defer rows.Close()

	var all []app.Movement
	for rows.Next() {
		var m app.Movement
		var date, kind, receipt string
		var amount int64
		var to, category, vendor sql.NullInt64
		var notes, link sql.NullString
		if err := rows.Scan(&m.ID, &date, &kind, &m.AccountID, &to, &amount, &category, &vendor,
			&m.Description, &notes, &link, &receipt); err != nil {
			return nil, fmt.Errorf("read movements: %w", err)
		}
		if m.Date, err = time.ParseInLocation(dateLayout, date, time.Local); err != nil {
			return nil, fmt.Errorf("read movement %d: date: %w", m.ID, err)
		}
		m.Kind, m.Amount, m.Receipt = domain.MovementKind(kind), domain.Money(amount), app.ReceiptStatus(receipt)
		m.Notes, m.ReceiptLink = notes.String, link.String
		m.ToAccountID, m.CategoryID, m.VendorID = idOf(to), idOf(category), idOf(vendor)
		all = append(all, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read movements: %w", err)
	}
	return all, nil
}

func (s ledgerStore) InsertMovement(ctx context.Context, m app.Movement, now time.Time) (int64, error) {
	stamp := now.Format(timestampLayout)
	var id int64
	err := s.q.QueryRowContext(ctx,
		`INSERT INTO movements (date, kind, account_id, to_account_id, amount_cents, category_id, vendor_id,
		                        description, notes, receipt_link, manual_receipt, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		m.Date.Format(dateLayout), string(m.Kind), m.AccountID, nullableID(m.ToAccountID), int64(m.Amount),
		nullableID(m.CategoryID), nullableID(m.VendorID), m.Description, nullable(m.Notes), nullable(m.ReceiptLink),
		string(m.Receipt), stamp, stamp,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert movement: %w", err)
	}
	return id, nil
}

// idOf turns a nullable id column into an optional id.
func idOf(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	return &n.Int64
}
