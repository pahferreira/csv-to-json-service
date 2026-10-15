// Command server runs the csv-to-json conversion service. It:
// - Loads Configuration from the environment
// - Serves the HTTP API
// - Shuts down cleanly on SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pahferreira/csv-to-json-service/internal/adapter/httpapi"
	"github.com/pahferreira/csv-to-json-service/internal/platform/config"
	"github.com/pahferreira/csv-to-json-service/internal/platform/obs"
)

// version is overwritten at link time by -ldflags "-X main.version=...".
var version = "dev"

// Connection deadlines. ADR-0011 requires all four to be explicit: the zero
// value is no timeout at all, which is a slowloris vulnerability on a public
// endpoint. WriteTimeout is deliberately generous -- its clock starts once the
// request headers are read, so it covers Conversion and not just the write, and
// a handler cannot detect it firing. Too tight means a truncated response that
// still logs as a 200.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
)

// ADR-0011: Cloud Run SIGKILLs after a grace period defaulting to 10s, so a
// deadline longer than that would make this code decorative.
const drainTimeout = 5 * time.Second

// main does nothing but run and report, because os.Exit skips deferred calls.
func main() {
	if err := run(); err != nil {
		// A failure may predate the logger: config supplies the log level, so a
		// bad config has nothing to log through. Plain stderr on purpose.
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

	// Readiness is a flag so shutdown can flip it before draining.
	var ready atomic.Bool
	ready.Store(true)

	srv := newServer(cfg, httpapi.NewRouter(logger, ready.Load), logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Bound here rather than inside ListenAndServe so a port clash is reported
	// as a startup failure instead of after we have claimed to be listening.
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", srv.Addr, err)
	}

	logger.Info("listening",
		slog.String("version", version),
		slog.String("addr", srv.Addr),
		slog.Int64("max_bytes", cfg.MaxBytes),
		slog.Int("max_rows", cfg.MaxRows),
		slog.String("log_level", cfg.LogLevel.String()),
	)

	return serve(ctx, srv, ln, drainTimeout, func() { ready.Store(false) }, logger)
}

// serve runs srv until ctx is canceled, then drains it within drainFor.
//
// onDrain runs before draining starts so readiness can flip first: a load
// balancer must stop sending new requests while the in-flight ones finish.
func serve(
	ctx context.Context,
	srv *http.Server,
	ln net.Listener,
	drainFor time.Duration,
	onDrain func(),
	logger *slog.Logger,
) error {
	// Buffered so the goroutine can exit even if nobody is receiving yet.
	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.Serve(ln) }()

	select {
	case err := <-srvErr:
		// Died on its own, before any signal.
		return err
	case <-ctx.Done():
	}

	onDrain()
	logger.Info("draining", slog.Duration("deadline", drainFor))

	// A fresh context: ctx is already canceled, so deriving from it would
	// expire the deadline immediately.
	drainCtx, cancel := context.WithTimeout(context.Background(), drainFor)
	defer cancel()

	// Shutdown stops accepting and waits for in-flight connections. It does not
	// cancel in-flight request contexts, so a handler ignoring its own context
	// can still outlive this deadline.
	shutdownErr := srv.Shutdown(drainCtx)

	// Serve returns ErrServerClosed once Shutdown completes. That is the clean
	// path, not a failure.
	if err := <-srvErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	if shutdownErr != nil {
		return fmt.Errorf("drain within %s: %w", drainFor, shutdownErr)
	}

	logger.Info("stopped")

	return nil
}

// newServer is separate from run so a test can assert the deadlines are set.
// gosec covers ReadHeaderTimeout; nothing would notice the other three
// disappearing in a refactor.
func newServer(cfg config.Config, handler http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		// An empty host binds every interface, which Cloud Run requires;
		// binding 127.0.0.1 makes the container silently unreachable.
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		// net/http otherwise writes its own errors unstructured to stderr,
		// bypassing the JSON stream Cloud Logging reads.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
}
