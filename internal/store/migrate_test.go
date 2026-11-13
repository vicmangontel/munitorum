package store

import (
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestMigrate(t *testing.T) {
	files := fstest.MapFS{
		"002_b.sql": {Data: []byte(`CREATE TABLE b (id INTEGER PRIMARY KEY);`)},
		"001_a.sql": {Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY);`)},
		"README.md": {Data: []byte(`not a migration`)},
	}

	t.Run("applies sql files in filename order", func(t *testing.T) {
		db, err := Open(filepath.Join(t.TempDir(), "m.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		got, err := Migrate(db, files)
		if err != nil {
			t.Fatalf("migrate: %v", err)
		}
		if len(got) != 2 || got[0] != "001_a.sql" || got[1] != "002_b.sql" {
			t.Fatalf("applied = %v, want [001_a.sql 002_b.sql]", got)
		}
	})

	t.Run("skips migrations already applied", func(t *testing.T) {
		db, err := Open(filepath.Join(t.TempDir(), "m.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		if _, err := Migrate(db, files); err != nil {
			t.Fatal(err)
		}
		got, err := Migrate(db, files)
		if err != nil {
			t.Fatalf("second migrate: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("second run applied %v, want none", got)
		}
	})

	t.Run("failed migration leaves no trace", func(t *testing.T) {
		db, err := Open(filepath.Join(t.TempDir(), "m.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()

		bad := fstest.MapFS{
			"001_bad.sql": {Data: []byte(`CREATE TABLE c (id INTEGER PRIMARY KEY); NOT SQL;`)},
		}
		if _, err := Migrate(db, bad); err == nil {
			t.Fatal("expected an error")
		}

		var tables, recorded int
		db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'c'`).Scan(&tables)
		db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&recorded)
		if tables != 0 || recorded != 0 {
			t.Fatalf("table c = %d, recorded = %d; want 0, 0", tables, recorded)
		}
	})
}

func TestOpenEnablesForeignKeys(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "fk.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var on int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&on); err != nil {
		t.Fatal(err)
	}
	if on != 1 {
		t.Fatalf("foreign_keys = %d, want 1", on)
	}
}

func TestEmbeddedMigrationsApply(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "e.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := Migrate(db, Migrations()); err != nil {
		t.Fatalf("embedded migrations: %v", err)
	}
}
