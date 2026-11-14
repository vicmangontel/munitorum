// Package logging writes structured JSON logs to a daily file.
// Nothing is written to stdout or stderr: the terminal belongs to the TUI.
package logging

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	filePrefix = "munitorum-"
	fileSuffix = ".log"
	dateLayout = "2006-01-02"
	keepDays   = 30
)

// Open creates dir if needed and returns a logger writing to
// dir/munitorum-YYYY-MM-DD.log for the day of now, plus the file to close on
// exit. Log files more than 30 days old are deleted; a file that cannot be
// deleted is reported as a warning and never stops the application.
func Open(dir string, now time.Time) (*slog.Logger, io.Closer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create log folder: %w", err)
	}
	name := filepath.Join(dir, filePrefix+now.Format(dateLayout)+fileSuffix)
	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file: %w", err)
	}
	log := slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := prune(dir, now); err != nil {
		log.Warn("log_prune", "action", "log_prune", "outcome", "error", "error", err.Error())
	}
	return log, f, nil
}

// prune removes daily log files dated before the 30-day window ending today.
// Age comes from the date in the file name, not the modification time, so
// copying or restoring a file does not change it.
func prune(dir string, now time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("list log folder: %w", osError(err))
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := today.AddDate(0, 0, -keepDays)

	var errs []error
	for _, e := range entries {
		date, ok := logFileDate(e.Name(), now.Location())
		if !ok || !date.Before(cutoff) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			errs = append(errs, fmt.Errorf("delete old log %s: %w", e.Name(), osError(err)))
		}
	}
	return errors.Join(errs...)
}

// logFileDate extracts the date from munitorum-YYYY-MM-DD.log. Any other file
// name is not ours and is left alone.
func logFileDate(name string, loc *time.Location) (time.Time, bool) {
	date, ok := strings.CutPrefix(name, filePrefix)
	if !ok {
		return time.Time{}, false
	}
	date, ok = strings.CutSuffix(date, fileSuffix)
	if !ok {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(dateLayout, date, loc)
	return t, err == nil
}

// osError drops the path an os function put in its error: the prune warning
// names the file by its base name only, so the folder (which includes the
// Windows user name) never reaches the log.
func osError(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
