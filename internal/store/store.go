// Package store implements the application repositories on SQLite.
package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Open opens (or creates) the SQLite database at path with the pragmas every
// connection needs. The pool is limited to one connection: Munitorum has a
// single user and SQLite has a single writer.
func Open(path string) (*sql.DB, error) {
	// The driver splits the DSN at the first '?'. A path containing one would open
	// a different file, so it is refused. The path is deliberately not given a
	// "file:" prefix: that makes SQLite parse it as a URI, where '#' truncates it
	// and %XX is decoded.
	if strings.Contains(path, "?") {
		return nil, fmt.Errorf("open database: the path must not contain '?'")
	}
	dsn := path +
		"?_pragma=foreign_keys(1)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(DELETE)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	return db, nil
}
