package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
)

func newVendorRepo(t *testing.T) *VendorRepository {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "v.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := Migrate(db, Migrations()); err != nil {
		t.Fatal(err)
	}
	return NewVendorRepository(db)
}

var now = time.Date(2026, time.November, 13, 9, 30, 0, 0, time.Local)

func TestVendorsTable_RejectsAnUnknownKindAndADanglingCompany(t *testing.T) {
	cases := []struct {
		name   string
		vendor app.Vendor
	}{
		{"unknown kind", app.Vendor{Kind: "robot", Name: "R2"}},
		{"company that does not exist", app.Vendor{Kind: domain.VendorPerson, Name: "Ana", CompanyID: new(int64(99))}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			repo := newVendorRepo(t)

			// Act
			_, err := repo.InsertVendor(context.Background(), c.vendor, now)

			// Assert
			if err == nil {
				t.Fatal("want the database to refuse the row")
			}
		})
	}
}

func TestVendorRepositoryInTx_RollsBackEveryWriteWhenTheFunctionFails(t *testing.T) {
	// Arrange
	repo := newVendorRepo(t)
	ctx := context.Background()
	boom := errors.New("boom")

	// Act
	err := repo.InTx(ctx, func(tx app.VendorStore) error {
		if _, err := tx.InsertVendor(ctx, app.Vendor{Kind: domain.VendorService, Name: "Basura"}, now); err != nil {
			return err
		}
		return boom
	})

	// Assert
	all, listErr := repo.Vendors(ctx)
	if !errors.Is(err, boom) || listErr != nil || len(all) != 0 {
		t.Fatalf("error = %v, vendors = %v (%v); want the insert rolled back", err, all, listErr)
	}
}

func TestVendorRepository_UpdateAndDeleteReturnNotFoundForAnUnknownId(t *testing.T) {
	// Arrange
	repo := newVendorRepo(t)
	ctx := context.Background()

	// Act
	updateErr := repo.UpdateVendor(ctx, app.Vendor{ID: 7, Kind: domain.VendorService, Name: "X"}, now)
	deleteErr := repo.SetVendorDeleted(ctx, 7, true, now)

	// Assert
	if !errors.Is(updateErr, app.ErrNotFound) || !errors.Is(deleteErr, app.ErrNotFound) {
		t.Fatalf("errors = %v, %v; want ErrNotFound for both", updateErr, deleteErr)
	}
}
