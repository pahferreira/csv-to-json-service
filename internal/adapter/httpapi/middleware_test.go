package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Proves the middleware logs at all, so the exclusion test is not vacuous.
func TestRequestLoggerLogs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := requestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hi"))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPatch, "/probe", nil))

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))

	assert.Equal(t, "request", record["msg"])
	assert.Equal(t, http.MethodPatch, record["method"])
	assert.Equal(t, "/probe", record["path"])
	assert.EqualValues(t, http.StatusTeapot, record["status"])
	assert.EqualValues(t, 2, record["bytes"])
	assert.Contains(t, record, "duration")
}

// Guards the seeded 200: a body written without WriteHeader is an implicit 200.
func TestRequestLoggerDefaultsToStatus200(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := requestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("no explicit WriteHeader"))
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/probe", nil))

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))

	assert.EqualValues(t, http.StatusOK, record["status"])
}

func TestRecovererTurnsPanicInto500(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := recoverer(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("row 42 contained: secret@example.com")
	}))

	rec := httptest.NewRecorder()
	require.NotPanics(t, func() {
		h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/probe", nil))
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var body errorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "internal_error", body.Error.Code)

	// ADR-0008: a panic value may carry row data, so it must not reach the logs.
	logged := buf.String()
	assert.Contains(t, logged, "panic recovered")
	assert.Contains(t, logged, "stack")
	assert.NotContains(t, logged, "secret@example.com",
		"the recovered value must never be logged")
}

// net/http expects to receive ErrAbortHandler, so it must not be swallowed.
func TestRecovererPropagatesErrAbortHandler(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	h := recoverer(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	assert.PanicsWithError(t, http.ErrAbortHandler.Error(), func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/probe", nil))
	})
	assert.Empty(t, buf.String(), "an intentional abort is not a crash")
}
