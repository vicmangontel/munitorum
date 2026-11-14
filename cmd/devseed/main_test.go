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
