package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_UsesThePathAsAPlainFilename(t *testing.T) {
	// A '#' or a %XX in a folder name must not be read as URI syntax.
	for _, folder := range []string{"C#datos", "a%41b", "Coto #3"} {
		t.Run(folder, func(t *testing.T) {
			// Arrange
			root := t.TempDir()
			dir := filepath.Join(root, folder)
			if err := os.Mkdir(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "munitorum.db")

			// Act
			db, err := Open(path)

			// Assert
			if err != nil {
				t.Fatalf("Open(%q): %v", path, err)
			}
			defer db.Close()
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("database not created at the exact path: %v", err)
			}
			rootEntries, _ := os.ReadDir(root)
			dirEntries, _ := os.ReadDir(dir)
			if len(rootEntries) != 1 || len(dirEntries) != 1 {
				t.Fatalf("unexpected files created: root=%v dir=%v", rootEntries, dirEntries)
			}
			var fk int
			var mode string
			if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
				t.Errorf("foreign_keys = %d, %v; want 1", fk, err)
			}
			if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "delete" {
				t.Errorf("journal_mode = %q, %v; want delete", mode, err)
			}
		})
	}
}

func TestOpen_RejectsAPathContainingAQuestionMark(t *testing.T) {
	// Arrange: '?' would start the pragma query string and open a different file.
	root := t.TempDir()

	// Act
	_, err := Open(filepath.Join(root, "a?b.db"))

	// Assert
	if err == nil {
		t.Fatal("expected an error")
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatalf("a file was created: %v", entries)
	}
}
