package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func listenLocal(t *testing.T) net.Listener {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	return ln
}

// A request already in flight when the signal arrives must still complete.
func TestServeDrainsInFlightRequest(t *testing.T) {
	t.Parallel()

	ln := listenLocal(t)
	inHandler := make(chan struct{})
	release := make(chan struct{})

	srv := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(inHandler)
			<-release
			_, _ = w.Write([]byte("finished"))
		}),
	}

	ctx, signalArrives := context.WithCancel(t.Context())

	var drained atomic.Bool
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- serve(ctx, srv, ln, 5*time.Second, func() { drained.Store(true) }, discardLogger())
	}()

	type result struct {
		status int
		body   string
		err    error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String()) //nolint:noctx // deliberate blocking probe
		if err != nil {
			got <- result{err: err}

			return
		}
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)
		got <- result{status: resp.StatusCode, body: string(body)}
	}()

	<-inHandler     // the handler is mid-request
	signalArrives() // SIGTERM equivalent
	close(release)  // let the handler finish

	require.NoError(t, <-serveErr)

	r := <-got
	require.NoError(t, r.err, "an in-flight request must not be dropped")
	assert.Equal(t, http.StatusOK, r.status)
	assert.Equal(t, "finished", r.body)
	assert.True(t, drained.Load(), "readiness must flip before draining")
}

// Once the drain completes the listener is closed, so new work is refused.
func TestServeRefusesNewRequestsAfterShutdown(t *testing.T) {
	t.Parallel()

	ln := listenLocal(t)
	addr := ln.Addr().String()

	srv := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
	}

	ctx, signalArrives := context.WithCancel(t.Context())
	serveErr := make(chan error, 1)
	go func() { serveErr <- serve(ctx, srv, ln, 5*time.Second, func() {}, discardLogger()) }()

	resp, err := http.Get("http://" + addr) //nolint:noctx // probe before shutdown
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	signalArrives()
	require.NoError(t, <-serveErr)

	after, err := http.Get("http://" + addr) //nolint:noctx // probe after shutdown
	if err == nil {
		_ = after.Body.Close()
	}

	assert.Error(t, err, "the listener must be closed once the drain finishes")
}

// A handler that outlives the deadline surfaces as an error, not a silent exit.
func TestServeReportsDrainDeadlineExceeded(t *testing.T) {
	t.Parallel()

	ln := listenLocal(t)
	inHandler := make(chan struct{})
	release := make(chan struct{})
	defer close(release)

	srv := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(inHandler)
			<-release
			w.WriteHeader(http.StatusOK)
		}),
	}

	ctx, signalArrives := context.WithCancel(t.Context())
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- serve(ctx, srv, ln, 50*time.Millisecond, func() {}, discardLogger())
	}()

	go func() {
		resp, err := http.Get("http://" + ln.Addr().String()) //nolint:noctx // deliberate blocking probe
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	<-inHandler
	signalArrives()

	err := <-serveErr
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// A listener that fails mid-flight is returned, not mistaken for a clean stop.
func TestServeReturnsServeError(t *testing.T) {
	t.Parallel()

	ln := listenLocal(t)
	require.NoError(t, ln.Close()) // Serve fails immediately on a closed listener.

	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.NotFoundHandler()}

	err := serve(t.Context(), srv, ln, time.Second, func() {}, discardLogger())
	require.Error(t, err)
	assert.NotErrorIs(t, err, http.ErrServerClosed)
}
