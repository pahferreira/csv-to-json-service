package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record),
		"handler must emit one valid JSON object per record")

	return record
}

func TestNewLoggerEmitsCloudLoggingFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	NewLogger(&buf, slog.LevelInfo).Info("configuration loaded", slog.Int("port", 8080))

	record := decode(t, &buf)

	assert.Equal(t, "INFO", record[severityKey])
	assert.Equal(t, "configuration loaded", record[messageKey])
	assert.Contains(t, record, slog.TimeKey, "Cloud Logging reads the entry time from this key")
	assert.EqualValues(t, 8080, record["port"])

	assert.NotContains(t, record, slog.LevelKey)
	assert.NotContains(t, record, slog.MessageKey)
}

func TestNewLoggerMapsEverySeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
		want  string
	}{
		{name: "debug", level: slog.LevelDebug, want: "DEBUG"},
		{name: "info", level: slog.LevelInfo, want: "INFO"},
		{name: "warn becomes WARNING, not WARN", level: slog.LevelWarn, want: "WARNING"},
		{name: "error", level: slog.LevelError, want: "ERROR"},
		{name: "offset level still maps", level: slog.LevelInfo + 2, want: "INFO"},
		{name: "above error maps to ERROR", level: slog.LevelError + 4, want: "ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			NewLogger(&buf, slog.LevelDebug).Log(context.Background(), tt.level, "msg")

			assert.Equal(t, tt.want, decode(t, &buf)[severityKey])
		})
	}
}

func TestNewLoggerFiltersBelowLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewLogger(&buf, slog.LevelWarn)

	logger.Debug("debug")
	logger.Info("info")
	assert.Empty(t, buf.String(), "records below the configured level must be discarded")

	logger.Warn("warn")
	assert.NotEmpty(t, buf.String(), "records at the configured level must be written")
}

func TestNewLoggerLeavesGroupedAttrsAlone(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	NewLogger(&buf, slog.LevelInfo).Info("msg",
		slog.Group("payload", slog.String(slog.LevelKey, "not-a-severity")))

	payload, ok := decode(t, &buf)["payload"].(map[string]any)
	require.True(t, ok, "group must be emitted as a nested object")

	assert.Equal(t, "not-a-severity", payload[slog.LevelKey])
	assert.NotContains(t, payload, severityKey)
}
