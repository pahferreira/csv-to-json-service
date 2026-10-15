package main

import (
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pahferreira/csv-to-json-service/internal/platform/config"
)

// Guards against a refactor silently dropping a deadline. gosec only checks
// ReadHeaderTimeout, and the zero value of the others is "no timeout".
func TestNewServerSetsEveryDeadline(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := newServer(config.Config{Port: 8080}, http.NotFoundHandler(), logger)

	assert.Equal(t, ":8080", srv.Addr, "an empty host binds every interface")
	assert.NotZero(t, srv.ReadHeaderTimeout, "slowloris defense, per ADR-0011")
	assert.NotZero(t, srv.ReadTimeout)
	assert.NotZero(t, srv.WriteTimeout, "covers Conversion, not just the write")
	assert.NotZero(t, srv.IdleTimeout, "zero here silently inherits ReadTimeout")
	assert.NotNil(t, srv.ErrorLog, "net/http errors must reach the JSON stream")
	assert.NotNil(t, srv.Handler)
}
