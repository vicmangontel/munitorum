package store

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

//go:embed migrations
var embedded embed.FS

// Migrations returns the migration files shipped with the binary.
func Migrations() fs.FS {
	sub, err := fs.Sub(embedded, "migrations")
	if err != nil {
		panic(err) // the embedded directory always exists
	}
	return sub
}

// Migrate applies every *.sql file in files that is not yet recorded in
// schema_migrations, in filename order, each in its own transaction.
// It returns the names of the files it applied.
func Migrate(db *sql.DB, files fs.FS) ([]string, error) {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var done []string
	for _, name := range names {
		if applied[name] {
			continue
		}
		script, err := fs.ReadFile(files, path.Clean(name))
		if err != nil {
			return done, fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := applyOne(db, name, string(script)); err != nil {
			return done, err
		}
		done = append(done, name)
	}
	return done, nil
}

func appliedVersions(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("list applied migrations: %w", err)
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyOne(db *sql.DB, name, script string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migration %s: %w", name, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(script); err != nil {
		return fmt.Errorf("migration %s: %w", name, err)
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		name, time.Now().Format("2006-01-02 15:04:05"),
	); err != nil {
		return fmt.Errorf("migration %s: record: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration %s: commit: %w", name, err)
	}
	return nil
}
