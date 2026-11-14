package logging

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, time.November, 13, 9, 30, 0, 0, time.Local)

func TestOpen_WritesJSONLinesToTheDailyFile(t *testing.T) {
	// Arrange
	dir := filepath.Join(t.TempDir(), "logs")
	log, closer, err := Open(dir, now)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	log.Info("startup", "action", "startup", "migrations_applied", 3)
	closer.Close()

	// Assert
	data, err := os.ReadFile(filepath.Join(dir, "munitorum-2026-11-13.log"))
	if err != nil {
		t.Fatalf("daily file not created: %v", err)
	}
	var line map[string]any
	if err := json.Unmarshal(data, &line); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, data)
	}
	if line["action"] != "startup" || line["migrations_applied"] != float64(3) || line["level"] != "INFO" {
		t.Fatalf("unexpected fields: %v", line)
	}
}

func TestOpen_AppendsWhenTheFileAlreadyExists(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	first, c1, err := Open(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	first.Info("one")
	c1.Close()

	// Act
	second, c2, err := Open(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	second.Info("two")
	c2.Close()

	// Assert
	data, _ := os.ReadFile(filepath.Join(dir, "munitorum-2026-11-13.log"))
	if lines := strings.Count(string(data), "\n"); lines != 2 {
		t.Fatalf("%d lines, want 2:\n%s", lines, data)
	}
}

func TestOpen_DeletesLogFilesOlderThan30Days(t *testing.T) {
	// Arrange: today is 2026-11-13, so the window starts on 2026-10-14.
	dir := t.TempDir()
	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []string{
		"munitorum-2026-10-13.log", // 31 days old: deleted
		"munitorum-2025-01-01.log", // deleted
		"munitorum-2026-10-14.log", // exactly 30 days old: kept
		"munitorum-2026-11-12.log", // kept
		"munitorum-2026-11-14.log", // dated in the future: kept
		"notes.txt",                // not ours: kept
		"munitorum-old.log",        // not a date: kept
		"other-2020-01-01.log",     // not ours: kept
	} {
		touch(n)
	}

	// Act
	_, closer, err := Open(dir, now)
	if err != nil {
		t.Fatal(err)
	}
	closer.Close()

	// Assert
	entries, _ := os.ReadDir(dir)
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	want := []string{
		"munitorum-2026-10-14.log", "munitorum-2026-11-12.log", "munitorum-2026-11-13.log",
		"munitorum-2026-11-14.log", "munitorum-old.log", "notes.txt", "other-2020-01-01.log",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("files = %v, want %v", got, want)
	}
}

func TestOpen_WarnsAndContinuesWhenAnOldLogCannotBeDeleted(t *testing.T) {
	// Arrange: a non-empty directory with an old log name cannot be removed as a file.
	dir := t.TempDir()
	stuck := filepath.Join(dir, "munitorum-2020-01-01.log")
	if err := os.MkdirAll(stuck, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stuck, "inside"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	log, closer, err := Open(dir, now)
	if err != nil {
		t.Fatalf("a prune failure must not fail Open: %v", err)
	}
	log.Info("after")
	closer.Close()

	// Assert
	data, _ := os.ReadFile(filepath.Join(dir, "munitorum-2026-11-13.log"))
	text := string(data)
	if !strings.Contains(text, `"level":"WARN"`) || !strings.Contains(text, `"action":"log_prune"`) || !strings.Contains(text, `"outcome":"error"`) {
		t.Fatalf("no prune warning logged:\n%s", text)
	}
	var warn map[string]any
	if err := json.Unmarshal([]byte(strings.SplitN(text, "\n", 2)[0]), &warn); err != nil {
		t.Fatalf("first line is not JSON: %v\n%s", err, text)
	}
	if msg, _ := warn["error"].(string); strings.Contains(msg, dir) {
		t.Fatalf("prune warning leaks the folder path: %q", msg)
	}
	if !strings.Contains(text, "munitorum-2020-01-01.log") {
		t.Fatalf("prune warning does not name the file:\n%s", text)
	}
	if !strings.Contains(text, `"msg":"after"`) {
		t.Fatalf("logger unusable after prune failure:\n%s", text)
	}
}

func TestOpen_FailsWhenTheLogFolderCannotBeCreated(t *testing.T) {
	// Arrange: a file where the folder should be.
	blocker := filepath.Join(t.TempDir(), "logs")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Act
	_, _, err := Open(blocker, now)

	// Assert
	if err == nil {
		t.Fatal("expected an error")
	}
}
