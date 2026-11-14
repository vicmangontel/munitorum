// Command devseed recreates the development database with sample data. It only
// ever touches dev/munitorum-dev.db, never the real database. It is a developer
// tool and is not part of a release build.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	return applied, nil
}
