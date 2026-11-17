package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
)

// queryer is what *sql.DB and *sql.Tx share.
type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// vendorStore implements app.VendorStore over a database or a transaction.
type vendorStore struct{ q queryer }

// VendorRepository implements app.VendorRepository on SQLite.
type VendorRepository struct {
	vendorStore
	db *sql.DB
}

// NewVendorRepository returns the vendor repository for db.
func NewVendorRepository(db *sql.DB) *VendorRepository {
	return &VendorRepository{vendorStore: vendorStore{q: db}, db: db}
}

func (r *VendorRepository) InTx(ctx context.Context, fn func(app.VendorStore) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin vendor transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fn(vendorStore{q: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit vendor transaction: %w", err)
	}
	return nil
}

func (s vendorStore) Vendors(ctx context.Context) ([]app.Vendor, error) {
	rows, err := s.q.QueryContext(ctx,
		`SELECT id, kind, name, phone, email, company_id, notes, deleted_at IS NOT NULL
		 FROM vendors ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("read vendors: %w", err)
	}
	defer rows.Close()

	var all []app.Vendor
	for rows.Next() {
		var v app.Vendor
		var kind string
		var phone, email, notes sql.NullString
		var company sql.NullInt64
		if err := rows.Scan(&v.ID, &kind, &v.Name, &phone, &email, &company, &notes, &v.Deleted); err != nil {
			return nil, fmt.Errorf("read vendors: %w", err)
		}
		v.Kind, v.Phone, v.Email, v.Notes = domain.VendorKind(kind), phone.String, email.String, notes.String
		if company.Valid {
			v.CompanyID = &company.Int64
		}
		all = append(all, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read vendors: %w", err)
	}
	return all, nil
}

func (s vendorStore) InsertVendor(ctx context.Context, v app.Vendor, now time.Time) (int64, error) {
	stamp := now.Format(timestampLayout)
	var id int64
	err := s.q.QueryRowContext(ctx,
		`INSERT INTO vendors (kind, name, phone, email, company_id, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		string(v.Kind), v.Name, nullable(v.Phone), nullable(v.Email), nullableID(v.CompanyID), nullable(v.Notes), stamp, stamp,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert vendor: %w", err)
	}
	return id, nil
}

func (s vendorStore) UpdateVendor(ctx context.Context, v app.Vendor, now time.Time) error {
	res, err := s.q.ExecContext(ctx,
		`UPDATE vendors SET kind = ?, name = ?, phone = ?, email = ?, company_id = ?, notes = ?, updated_at = ?
		 WHERE id = ?`,
		string(v.Kind), v.Name, nullable(v.Phone), nullable(v.Email), nullableID(v.CompanyID), nullable(v.Notes),
		now.Format(timestampLayout), v.ID)
	return requireRow(res, err, "update vendor")
}

func (s vendorStore) SetVendorDeleted(ctx context.Context, id int64, deleted bool, now time.Time) error {
	stamp := now.Format(timestampLayout)
	var deletedAt any // NULL when restoring
	if deleted {
		deletedAt = stamp
	}
	res, err := s.q.ExecContext(ctx,
		`UPDATE vendors SET deleted_at = ?, updated_at = ? WHERE id = ?`, deletedAt, stamp, id)
	return requireRow(res, err, "set vendor deleted")
}

// nullable stores an empty string as NULL.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullableID(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}
