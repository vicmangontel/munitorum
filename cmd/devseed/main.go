// Command devseed recreates the development database with sample data. It only
// ever touches dev/munitorum-dev.db, never the real database. It is a developer
// tool and is not part of a release build.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/domain"
	"github.com/vicmangontel/munitorum/internal/store"
)

var devDB = filepath.Join("dev", "munitorum-dev.db")

func main() {
	applied, err := reset(devDB)
	if err != nil {
		fmt.Fprintln(os.Stderr, "devseed:", err)
		os.Exit(1)
	}
	fmt.Printf("devseed: created %s, %d migrations applied\n", devDB, len(applied))
}

// reset deletes the database at path and creates it again with every migration
// applied. Each phase that adds a table adds its fixture data here.
func reset(path string) ([]string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create dev folder: %w", err)
	}
	// The journal goes too: a leftover one would be replayed into the new file.
	for _, p := range []string{path, path + "-journal"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("remove old dev database: %w", err)
		}
	}

	db, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	applied, err := store.Migrate(db, store.Migrations())
	if err != nil {
		return nil, err
	}
	if err := seedCoto(context.Background(), db, time.Now()); err != nil {
		return nil, err
	}
	if err := seedDirectory(context.Background(), db); err != nil {
		return nil, err
	}
	return applied, nil
}

// seedCoto completes setup for 18 houses opened on the first of the current month,
// with a mix of opening states: up to date, two months behind, a year prepaid,
// never paid, and opening arrears or credit.
func seedCoto(ctx context.Context, db *sql.DB, now time.Time) error {
	opening := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	yearAhead := domain.Of(opening).Add(11)

	houses := make([]app.SetupHouse, 18)                                                          // the rest are up to date: nothing covered, nothing owed
	houses[1] = app.SetupHouse{Arrears: "300.00"}                                                 // two months behind
	houses[2] = app.SetupHouse{CoveredThrough: yearAhead.InputString()}                           // a year prepaid
	houses[3] = app.SetupHouse{Arrears: "900.00"}                                                 // never paid
	houses[4] = app.SetupHouse{Arrears: "75.50"}                                                  // opening arrears
	houses[5] = app.SetupHouse{CoveredThrough: domain.Of(opening).InputString(), Credit: "50.00"} // covered plus credit

	svc := app.NewSetupService(store.NewSetupRepository(db), app.SystemClock{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := svc.CompleteSetup(ctx, app.SetupInput{
		CommunityName: "Coto de Desarrollo",
		HouseCount:    "18",
		HousePrefix:   "CASA ",
		FeeAmount:     "150.00",
		Accounts: [2]app.SetupAccount{
			{Name: "Efectivo", OpeningBalance: "7,500.00"},
			{Name: "Banco (NU)", OpeningBalance: "12,000.00"},
		},
		OpeningDate: domain.FormatInputDate(opening),
		Houses:      houses,
	})
	if err != nil {
		return fmt.Errorf("seed coto: %w", err)
	}
	return nil
}

// seedDirectory adds a gardener and a container-cleaning person, the company
// Portones Acme with one technician, the services Basura and CFE, and one
// deleted vendor.
func seedDirectory(ctx context.Context, db *sql.DB) error {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := app.NewDirectoryService(store.NewVendorRepository(db), app.SystemClock{}, log)

	acme, err := svc.CreateVendor(ctx, app.VendorInput{Kind: domain.VendorCompany, Name: "Portones Acme", Phone: "55 5555 0100"})
	if err != nil {
		return fmt.Errorf("seed directory: %w", err)
	}
	vendors := []app.VendorInput{
		{Kind: domain.VendorPerson, Name: "Luis Hernández", Phone: "55 5555 0101", Notes: "Jardinero"},
		{Kind: domain.VendorPerson, Name: "Rosa Martínez", Phone: "55 5555 0102", Notes: "Limpieza de contenedores"},
		{Kind: domain.VendorPerson, Name: "Juan Gómez", Phone: "55 5555 0103", Email: "juan@portonesacme.example", CompanyID: &acme.ID, Notes: "Técnico de portones"},
		{Kind: domain.VendorService, Name: "Basura"},
		{Kind: domain.VendorService, Name: "CFE"},
	}
	for _, in := range vendors {
		if _, err := svc.CreateVendor(ctx, in); err != nil {
			return fmt.Errorf("seed directory: %w", err)
		}
	}
	gone, err := svc.CreateVendor(ctx, app.VendorInput{Kind: domain.VendorPerson, Name: "Antonio Ruiz", Phone: "55 5555 0104", Notes: "Ya no trabaja con el coto"})
	if err != nil {
		return fmt.Errorf("seed directory: %w", err)
	}
	if err := svc.DeleteVendor(ctx, gone.ID); err != nil {
		return fmt.Errorf("seed directory: %w", err)
	}
	return nil
}
