// Package obs builds the service's observability primitives.
package obs

import (
	"io"
	"log/slog"
)

// Cloud Logging field names.
const (
	severityKey = "severity"
	messageKey  = "message"
)

// NewLogger returns a logger writing JSON to w, discarding records below level.
func NewLogger(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: cloudLoggingAttrs,
	}))
}

// cloudLoggingAttrs renames slog's built-in keys to the ones Cloud Logging recognizes.
func cloudLoggingAttrs(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}

	switch a.Key {
	case slog.LevelKey:
		a.Key = severityKey
		if lvl, ok := a.Value.Any().(slog.Level); ok {
			a.Value = slog.StringValue(severity(lvl))
		}
	case slog.MessageKey:
		a.Key = messageKey
	}

	return a
}

// severity maps a slog level onto Cloud Logging's severity enum.
func severity(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "DEBUG"
	case l < slog.LevelWarn:
		return "INFO"
	case l < slog.LevelError:
		return "WARNING"
	default:
		return "ERROR"
	}
}
