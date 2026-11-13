// Package logging writes structured JSON logs to a daily file.
// Nothing is written to stdout or stderr: the terminal belongs to the TUI.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Open creates dir if needed and returns a logger writing to
// dir/munitorum-YYYY-MM-DD.log, plus the file to close on exit.
func Open(dir string) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log folder: %w", err)
	}
	name := filepath.Join(dir, "munitorum-"+time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	return slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo})), f, nil
}
