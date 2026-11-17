package app

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// logEvent writes the wide event of one operation: its action, outcome, duration
// and the fields learned on the way. A rejected input (ValidationError or
// VendorConflict) is outcome "validation" and carries no error text, because
// that text can name people; any other error is outcome "error". It never logs
// names or other free text.
func logEvent(ctx context.Context, log *slog.Logger, action string, started time.Time, err error, fields []any) {
	outcome, level := "ok", slog.LevelInfo
	var ve ValidationError
	var vc VendorConflict
	switch {
	case errors.As(err, &ve):
		outcome, level = "validation", slog.LevelWarn
		fields = append(fields, "invalid_field_count", len(ve.Fields))
	case errors.As(err, &vc):
		outcome, level = "validation", slog.LevelWarn
	case err != nil:
		outcome, level = "error", slog.LevelError
		fields = append(fields, "error", err.Error())
	}
	fields = append(fields, "action", action, "outcome", outcome, "duration_ms", time.Since(started).Milliseconds())
	log.Log(ctx, level, action, fields...)
}
