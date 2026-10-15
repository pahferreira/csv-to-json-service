// Command server runs the csv-to-json conversion service. It:
// - Loads Configuration from the environment
// - Serves the HTTP API
// - Shuts down cleanly on SIGTERM.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/pahferreira/csv-to-json-service/internal/platform/config"
	"github.com/pahferreira/csv-to-json-service/internal/platform/obs"
)

// version is overwritten at link time by -ldflags "-X main.version=...", which
// the Makefile and Dockerfile both supply from `git describe`.
var version = "dev"

// main does nothing but run and report, because os.Exit skips deferred calls.
// Every defer the service needs -- draining the server, flushing exporters --
// belongs inside run, where it will actually fire.
func main() {
	if err := run(); err != nil {
		// A failure here may predate the logger: config supplies the log level,
		// so a bad config has nothing to log through. Plain stderr on purpose.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := obs.NewLogger(os.Stdout, cfg.LogLevel)
	logger.Info("configuration loaded",
		slog.String("version", version),
		slog.Int("port", cfg.Port),
		slog.Int64("max_bytes", cfg.MaxBytes),
		slog.Int("max_rows", cfg.MaxRows),
		slog.String("log_level", cfg.LogLevel.String()),
	)

	return nil
}
