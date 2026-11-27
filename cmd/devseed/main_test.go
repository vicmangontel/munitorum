package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vicmangontel/munitorum/internal/store"
)

func TestReset_ReplacesAnExistingFileWithAMigratedDatabase(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "dev", "munitorum-dev.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old junk, not a database, long enough to be noticed by sqlite......."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+"-journal", []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := reset(path)

	// Assert
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("recreated file is not a database: %v", err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("migrations were not recorded: %v", err)
	}
	if _, err := os.Stat(path + "-journal"); !os.IsNotExist(err) {
		t.Fatalf("stale journal left behind (stat err = %v)", err)
	}
}

func TestReset_CreatesTheDevFolderWhenMissing(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "dev", "munitorum-dev.db")

	// Act
	_, err := reset(path)

	// Assert
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database not created: %v", err)
	}
}

func TestReset_SeedsAnEighteenHouseCotoWithMixedOpeningStates(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "munitorum-dev.db")

	// Act
	_, err := reset(path)

	// Assert
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var houses, covered, withArrears, withCredit int
	if err := db.QueryRow(`SELECT COUNT(*), COUNT(opening_covered_through),
		SUM(opening_arrears_cents > 0), SUM(opening_credit_cents > 0) FROM houses`).
		Scan(&houses, &covered, &withArrears, &withCredit); err != nil {
		t.Fatal(err)
	}
	if houses != 18 || covered != 2 || withArrears != 3 || withCredit != 1 {
		t.Fatalf("houses=%d covered=%d arrears=%d credit=%d; want 18 2 3 1", houses, covered, withArrears, withCredit)
	}
}

func TestReset_SeedsTheDirectoryWithACompanyItsTechnicianServicesAndADeletedVendor(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "munitorum-dev.db")

	// Act
	_, err := reset(path)

	// Assert
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var live, deleted, services, technicians int
	if err := db.QueryRow(`SELECT COUNT(*) FILTER (WHERE deleted_at IS NULL), COUNT(deleted_at),
		COUNT(*) FILTER (WHERE kind = 'service' AND name IN ('Basura', 'CFE')),
		COUNT(*) FILTER (WHERE company_id = (SELECT id FROM vendors WHERE name = 'Portones Acme'))
		FROM vendors`).Scan(&live, &deleted, &services, &technicians); err != nil {
		t.Fatal(err)
	}
	if live != 6 || deleted != 1 || services != 2 || technicians != 1 {
		t.Fatalf("live=%d deleted=%d services=%d technicians=%d; want 6 1 2 1", live, deleted, services, technicians)
	}
}

func TestReset_SeedsSixMonthsOfLedgerMovementsAcrossBothAccountsWithoutANegativeBalance(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "munitorum-dev.db")

	// Act
	_, err := reset(path)

	// Assert
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var months, accounts, incomes, expenses, transfers, adjustments, others, withVendor int
	if err := db.QueryRow(`SELECT COUNT(DISTINCT substr(date, 1, 7)), COUNT(DISTINCT account_id),
		COUNT(*) FILTER (WHERE kind = 'income'), COUNT(*) FILTER (WHERE kind = 'expense'),
		COUNT(*) FILTER (WHERE kind = 'transfer'), COUNT(*) FILTER (WHERE kind = 'adjustment'),
		COUNT(*) FILTER (WHERE kind NOT IN ('income', 'expense', 'transfer', 'adjustment')), COUNT(vendor_id)
		FROM movements WHERE deleted_at IS NULL`).Scan(&months, &accounts, &incomes, &expenses, &transfers, &adjustments, &others, &withVendor); err != nil {
		t.Fatal(err)
	}
	if months != 6 || accounts != 2 || incomes == 0 || expenses == 0 || others != 0 || withVendor == 0 {
		t.Fatalf("months=%d accounts=%d incomes=%d expenses=%d others=%d withVendor=%d; want 6 months, both accounts, income and expenses with vendors, no other kind",
			months, accounts, incomes, expenses, others, withVendor)
	}
	if transfers != 2 || adjustments != 1 {
		t.Fatalf("transfers=%d adjustments=%d; want two transfers and one arqueo adjustment", transfers, adjustments)
	}
	var adjustment int64
	if err := db.QueryRow(`SELECT amount_cents FROM movements WHERE kind = 'adjustment'`).Scan(&adjustment); err != nil {
		t.Fatal(err)
	}
	if adjustment != -2350 {
		t.Fatalf("arqueo adjustment = %d cents, want −2350", adjustment)
	}
	var negative int
	if err := db.QueryRow(`SELECT COUNT(*) FROM accounts a WHERE a.opening_balance_cents +
		COALESCE((SELECT SUM(CASE kind WHEN 'income' THEN amount_cents WHEN 'adjustment' THEN amount_cents ELSE -amount_cents END)
		          FROM movements m WHERE m.account_id = a.id AND m.deleted_at IS NULL), 0) +
		COALESCE((SELECT SUM(amount_cents) FROM movements m WHERE m.to_account_id = a.id AND m.deleted_at IS NULL), 0) < 0`).Scan(&negative); err != nil {
		t.Fatal(err)
	}
	if negative != 0 {
		t.Fatalf("%d accounts end below zero, want none", negative)
	}
}
