// Package httpapi is the HTTP adapter: routing, middleware and response DTOs.
// It returns an http.Handler rather than a *chi.Mux, keeping chi local to here.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter assembles the service's routes. ready reports whether to accept new
// traffic; it is a function so shutdown can flip it without rebuilding the router.
func NewRouter(logger *slog.Logger, ready func() bool) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(recoverer(logger))

	// ADR-0011: unauthenticated, and excluded from request logging and rate
	// limiting. Mounting them outside the logged group is that exclusion.
	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(ready))

	// Everything serving Clients goes here and is logged.
	r.Group(func(r chi.Router) {
		r.Use(requestLogger(logger))

		r.Post("/convert", notImplemented) // Real handler arrives with #13.
	})

	r.NotFound(notFound)
	r.MethodNotAllowed(methodNotAllowed)

	return r
}

// healthz is liveness: ADR-0011 requires an unconditional 200. "Is this process
// running" is a different question from "should it be sent traffic".
func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

// readyz is readiness, returning 503 once the service stops accepting traffic
// so the load balancer drains it before in-flight work is cut off.
func readyz(ready func() bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !ready() {
			writeJSON(w, http.StatusServiceUnavailable,
				newErrorResponse("not_ready", "service is not accepting traffic"))

			return
		}

		writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
	}
}

func notImplemented(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotImplemented,
		newErrorResponse("not_implemented", "conversion is not implemented yet"))
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusNotFound, newErrorResponse("not_found", "no such route"))
}

func methodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusMethodNotAllowed,
		newErrorResponse("method_not_allowed", "method not allowed for this route"))
}
