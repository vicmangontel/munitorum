// Command munitorum is the coto treasury terminal application.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"munitorum/internal/logging"
	"munitorum/internal/store"
	"munitorum/internal/tui"
	"munitorum/internal/tui/text"
)

func main() {
	if err := run(); err != nil {
		// Phase 0 replaces this with an in-terminal error screen.
		fmt.Fprintln(os.Stderr, "munitorum:", err)
		os.Exit(1)
	}
}

func run() error {
	defaultDB, err := defaultDBPath()
	if err != nil {
		return err
	}
	dbPath := flag.String("db", defaultDB, text.FlagDB)
	flag.Parse()

	started := time.Now()
	log, closeLog, err := logging.Open(filepath.Join(filepath.Dir(*dbPath), "logs"))
	if err != nil {
		return err
	}
	defer closeLog.Close()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Error("startup", "action", "startup", "outcome", "error", "error", err.Error())
		return err
	}
	defer db.Close()

	applied, err := store.Migrate(db, store.Migrations())
	if err != nil {
		log.Error("startup", "action", "startup", "outcome", "error", "error", err.Error())
		return err
	}
	log.Info("startup",
		"action", "startup",
		"outcome", "ok",
		"duration_ms", time.Since(started).Milliseconds(),
		"db_file", filepath.Base(*dbPath),
		"migrations_applied", len(applied),
	)

	_, err = tea.NewProgram(tui.New(), tea.WithAltScreen()).Run()
	return err
}

// defaultDBPath is munitorum.db in the executable's folder.
func defaultDBPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "munitorum.db"), nil
}
