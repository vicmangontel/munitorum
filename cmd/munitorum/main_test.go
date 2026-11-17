package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vicmangontel/munitorum/internal/app"
	"github.com/vicmangontel/munitorum/internal/logging"
	"github.com/vicmangontel/munitorum/internal/store"
	"github.com/vicmangontel/munitorum/internal/tui"
)

var started = time.Date(2026, time.November, 13, 9, 30, 0, 0, time.Local)

// readEvents returns the JSON log lines written under dir/logs, after closing the log.
func readEvents(t *testing.T, logFile string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	var events []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var e map[string]any
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("log line is not JSON: %v\n%s", err, line)
		}
		events = append(events, e)
	}
	return events
}

func TestOpenDatabase_CreatesTheFileAndEmitsOneStartupEvent(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "munitorum.db")
	log, closeLog, err := logging.Open(filepath.Join(dir, "logs"), started)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	db, err := openDatabase(dbPath, log, started)
	closeLog.Close()

	// Assert
	if err != nil {
		t.Fatalf("openDatabase: %v", err)
	}
	defer db.Close()
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file not created: %v", err)
	}
	events := readEvents(t, filepath.Join(dir, "logs", "munitorum-2026-11-13.log"))
	if len(events) != 1 {
		t.Fatalf("%d events, want exactly one startup event: %v", len(events), events)
	}
	e := events[0]
	if e["action"] != "startup" || e["outcome"] != "ok" || e["level"] != "INFO" {
		t.Errorf("unexpected event: %v", e)
	}
	if e["db_file"] != "munitorum.db" {
		t.Errorf("db_file = %v, want the base name only", e["db_file"])
	}
	for _, field := range []string{"duration_ms", "migrations_applied"} {
		if _, ok := e[field].(float64); !ok {
			t.Errorf("%s missing or not a number: %v", field, e[field])
		}
	}
}

func TestOpenDatabase_ReopensAnExistingFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "munitorum.db")
	log, closeLog, err := logging.Open(filepath.Join(dir, "logs"), started)
	if err != nil {
		t.Fatal(err)
	}
	first, err := openDatabase(dbPath, log, started)
	if err != nil {
		t.Fatal(err)
	}
	first.Close()

	// Act
	second, err := openDatabase(dbPath, log, started)
	closeLog.Close()

	// Assert
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	second.Close()
	events := readEvents(t, filepath.Join(dir, "logs", "munitorum-2026-11-13.log"))
	if len(events) != 2 || events[1]["outcome"] != "ok" || events[1]["migrations_applied"] != float64(0) {
		t.Fatalf("second startup should apply nothing: %v", events)
	}
}

func TestOpenDatabase_FailsAndLeavesAnUnreadableFileUntouched(t *testing.T) {
	// Arrange: a text file where the database should be.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "munitorum.db")
	original := bytes.Repeat([]byte("this is not a sqlite database, it is a text file. "), 10)
	if err := os.WriteFile(dbPath, original, 0o644); err != nil {
		t.Fatal(err)
	}
	log, closeLog, err := logging.Open(filepath.Join(dir, "logs"), started)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	db, err := openDatabase(dbPath, log, started)
	closeLog.Close()

	// Assert
	if err == nil {
		db.Close()
		t.Fatal("expected an error for an unreadable database")
	}
	if db != nil {
		t.Error("a database handle was returned alongside the error")
	}
	after, readErr := os.ReadFile(dbPath)
	if readErr != nil || !bytes.Equal(after, original) {
		t.Fatalf("the unreadable file was modified (err=%v)", readErr)
	}
	events := readEvents(t, filepath.Join(dir, "logs", "munitorum-2026-11-13.log"))
	if len(events) != 1 {
		t.Fatalf("%d events, want exactly one: %v", len(events), events)
	}
	e := events[0]
	if e["action"] != "startup" || e["outcome"] != "error" || e["level"] != "ERROR" || e["db_file"] != "munitorum.db" {
		t.Errorf("unexpected event: %v", e)
	}
	msg, _ := e["error"].(string)
	if msg == "" {
		t.Errorf("failure event carries no error text: %v", e)
	}
	if strings.Contains(msg, dir) {
		t.Errorf("error text leaks the full database path: %q", msg)
	}
}

func TestOpenDatabase_FailsWhenTheFolderDoesNotExist(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	log, closeLog, err := logging.Open(filepath.Join(dir, "logs"), started)
	if err != nil {
		t.Fatal(err)
	}
	defer closeLog.Close()

	// Act
	_, err = openDatabase(filepath.Join(dir, "missing", "munitorum.db"), log, started)

	// Assert
	if err == nil {
		t.Fatal("expected an error when the database cannot be created")
	}
}

func newTestService(t *testing.T) (*app.SetupService, *app.DirectoryService) {
	t.Helper()
	dir := t.TempDir()
	log, closeLog, err := logging.Open(filepath.Join(dir, "logs"), started)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeLog.Close() })
	db, err := openDatabase(filepath.Join(dir, "munitorum.db"), log, started)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return app.NewSetupService(store.NewSetupRepository(db), app.SystemClock{}, log),
		app.NewDirectoryService(store.NewVendorRepository(db), app.SystemClock{}, log)
}

func TestStartModel_OpensTheWizardOnAFreshDatabase(t *testing.T) {
	// Arrange
	svc, dir := newTestService(t)

	// Act
	model, err := startModel(context.Background(), svc, dir, started)

	// Assert
	root, ok := model.(tui.Root)
	if err != nil || !ok || !root.InWizard() {
		t.Fatalf("model = %T, %v; want the wizard", model, err)
	}
}

func TestStartModel_OpensTheShellWithTheCotoOnACompletedDatabase(t *testing.T) {
	// Arrange
	svc, dir := newTestService(t)
	_, err := svc.CompleteSetup(context.Background(), app.SetupInput{
		CommunityName: "Coto Los Pinos",
		HouseCount:    "2",
		HousePrefix:   "C-",
		FeeAmount:     "150",
		Accounts: [2]app.SetupAccount{
			{Name: "Efectivo", OpeningBalance: "7,500.00"},
			{Name: "Banco (NU)", OpeningBalance: "0"},
		},
		OpeningDate: "01/01/2020",
		Houses:      make([]app.SetupHouse, 2),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Act
	model, err := startModel(context.Background(), svc, dir, started)
	sized, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// Assert
	root, ok := model.(tui.Root)
	if err != nil || !ok || root.InWizard() {
		t.Fatalf("model = %T, %v; want the shell", model, err)
	}
	view := sized.View()
	for _, want := range []string{"Coto Los Pinos", "$7,500.00"} {
		if !strings.Contains(view, want) {
			t.Errorf("shell does not show %q:\n%s", want, view)
		}
	}
}
