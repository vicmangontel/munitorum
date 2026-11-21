// Command munitorum is the coto treasury terminal application.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/logging"
	"github.com/vicmangontel/munitorum/internal/store"
	"github.com/vicmangontel/munitorum/internal/tui"
	"github.com/vicmangontel/munitorum/internal/tui/text"
)

func main() {
	if err := run(); err != nil {
		// The terminal is released by now, so stderr is safe.
		fmt.Fprintln(os.Stderr, text.StartupFailed, err)
		os.Exit(1)
	}
}

func run() error {
	started := time.Now()

	defaultDB, err := defaultDBPath()
	if err != nil {
		return err
	}
	dbPath := flag.String("db", defaultDB, text.FlagDB)
	flag.Parse()

	log, closeLog, err := logging.Open(filepath.Join(filepath.Dir(*dbPath), "logs"), started)
	if err != nil {
		return err
	}
	defer closeLog.Close()

	db, err := openDatabase(*dbPath, log, started)
	if err != nil {
		// Never the setup wizard: an existing file that fails to open is not a new coto.
		if _, tuiErr := tea.NewProgram(tui.NewStartupError(*dbPath, err), tea.WithAltScreen()).Run(); tuiErr != nil {
			return fmt.Errorf("%w (and the error screen failed: %v)", err, tuiErr)
		}
		return err
	}
	defer db.Close()

	ledgerRepo := store.NewLedgerRepository(db)
	svcs := services{
		setup:      app.NewSetupService(store.NewSetupRepository(db), app.SystemClock{}, log),
		directory:  app.NewDirectoryService(store.NewVendorRepository(db), app.SystemClock{}, log),
		ledger:     app.NewLedgerService(ledgerRepo, app.SystemClock{}, log),
		categories: app.NewCategoryService(ledgerRepo),
	}
	model, err := startModel(context.Background(), svcs, time.Now())
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(model, tea.WithAltScreen()).Run()
	return err
}

// openDatabase opens the database at path, applies the migrations, and emits the
// startup wide event: the one place that event is built, on success and failure.
func openDatabase(path string, log *slog.Logger, started time.Time) (db *sql.DB, err error) {
	var applied []string
	defer func() {
		outcome, level := "ok", slog.LevelInfo
		if err != nil {
			outcome, level = "error", slog.LevelError
		}
		fields := []any{
			"action", "startup",
			"outcome", outcome,
			"duration_ms", time.Since(started).Milliseconds(),
			"db_file", filepath.Base(path),
			"migrations_applied", len(applied),
		}
		if err != nil {
			fields = append(fields, "error", err.Error())
		}
		log.Log(context.Background(), level, "startup", fields...)
	}()

	db, err = store.Open(path)
	if err != nil {
		return nil, err
	}
	applied, err = store.Migrate(db, store.Migrations())
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// defaultDBPath is munitorum.db in the executable's folder.
func defaultDBPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate executable: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "munitorum.db"), nil
}

// services are the application services the screens call.
type services struct {
	setup      *app.SetupService
	directory  *app.DirectoryService
	ledger     *app.LedgerService
	categories *app.CategoryService
}

// startModel picks the first screen: the setup wizard while no coto exists, the
// shell once setup has completed. It runs only after the migrations succeeded
// (setup rule 10), so a database that cannot be read never reaches the wizard.
func startModel(ctx context.Context, svcs services, today time.Time) (tea.Model, error) {
	state, err := svcs.setup.GetSetupState(ctx)
	if err != nil {
		return nil, err
	}
	if !state.Completed {
		return tui.NewWizard(svcs.setup, today).WithDirectory(svcs.directory).
			WithLedger(svcs.ledger, svcs.categories, svcs.directory, app.SystemClock{}), nil
	}
	settings, err := svcs.setup.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	return tui.NewShell(svcs.setup, settings).WithDirectory(svcs.directory).
		WithLedger(svcs.ledger, svcs.categories, svcs.directory, app.SystemClock{}), nil
}
