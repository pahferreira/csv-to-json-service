package httpapi

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// statusRecorder captures what was sent; http.ResponseWriter cannot be read back.
type statusRecorder struct {
	http.ResponseWriter

	status int
	bytes  int
}

// Seeds 200: a body written without WriteHeader is an implicit 200, and a zero
// default would log every success as status=0.
func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n

	return n, err
}

// requestLogger logs one line per request. It is attached to a route group
// rather than the root, so ADR-0011's excluded endpoints are excluded by where
// they are mounted instead of by a path check hidden in here.
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := newStatusRecorder(w)

			next.ServeHTTP(rec, r)

			logger.InfoContext(r.Context(), "request",
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int("bytes", rec.bytes),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// recoverer turns a panic into a 500. It sits at the root so the operational
// endpoints are covered too, which puts it outside requestLogger.
func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}

				// Documented way to abandon a response; not a crash.
				if recovered == http.ErrAbortHandler { //nolint:errorlint // sentinel compared as any, not as error
					panic(recovered)
				}

				// The recovered value is not logged: it is an arbitrary any and
				// ADR-0008 forbids cell values reaching logs.
				logger.ErrorContext(r.Context(), "panic recovered",
					slog.String("request_id", middleware.GetReqID(r.Context())),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)

				writeJSON(w, http.StatusInternalServerError,
					newErrorResponse("internal_error", "internal server error"))
			}()

			next.ServeHTTP(w, r)
		})
	}
}
