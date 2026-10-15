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

// Returns a router plus the buffer its logger writes to.
func newTestRouter(t *testing.T, ready bool) (http.Handler, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	return NewRouter(logger, func() bool { return ready }), &buf
}

func do(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), method, path, nil))

	return rec
}

func TestRoutes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		ready      bool
		wantStatus int
		wantCode   string // error envelope code, empty when a success is expected
	}{
		{name: "healthz is always 200", method: http.MethodGet, path: "/healthz", ready: true, wantStatus: http.StatusOK},
		{name: "healthz ignores readiness", method: http.MethodGet, path: "/healthz", ready: false, wantStatus: http.StatusOK},
		{name: "readyz is 200 when ready", method: http.MethodGet, path: "/readyz", ready: true, wantStatus: http.StatusOK},
		{name: "readyz is 503 when draining", method: http.MethodGet, path: "/readyz", ready: false, wantStatus: http.StatusServiceUnavailable, wantCode: "not_ready"},
		{name: "unknown route is 404", method: http.MethodGet, path: "/nope", ready: true, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "wrong method is 405", method: http.MethodPost, path: "/healthz", ready: true, wantStatus: http.StatusMethodNotAllowed, wantCode: "method_not_allowed"},
		{name: "convert is stubbed until #13", method: http.MethodPost, path: "/convert", ready: true, wantStatus: http.StatusNotImplemented, wantCode: "not_implemented"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, _ := newTestRouter(t, tt.ready)
			rec := do(t, router, tt.method, tt.path)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))

			if tt.wantCode == "" {
				var body statusResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, "ok", body.Status)

				return
			}

			var body errorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tt.wantCode, body.Error.Code)
			assert.NotEmpty(t, body.Error.Message)
		})
	}
}

// Issue #10 acceptance criterion. Paired with TestRequestLoggerLogs, without
// which this would pass against a middleware that does nothing.
func TestOperationalEndpointsAreNotLogged(t *testing.T) {
	t.Parallel()

	router, buf := newTestRouter(t, true)

	do(t, router, http.MethodGet, "/healthz")
	do(t, router, http.MethodGet, "/readyz")
	assert.Empty(t, buf.String(), "ADR-0011 excludes the operational endpoints from request logging")

	do(t, router, http.MethodPost, "/convert")
	assert.NotEmpty(t, buf.String(), "routes inside the logged group must produce a line")

	// 404 and 405 are inside the group too: probe traffic must be visible.
	for _, path := range []string{"/nope", "/healthz"} {
		buf.Reset()
		do(t, router, http.MethodDelete, path)
		assert.NotEmpty(t, buf.String(), "miss handlers must be logged: "+path)
	}
}

func TestRequestIDIsSet(t *testing.T) {
	t.Parallel()

	router, buf := newTestRouter(t, true)
	do(t, router, http.MethodPost, "/convert")

	var record map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &record))
	assert.NotEmpty(t, record["request_id"], "every logged request must be correlatable")
}
