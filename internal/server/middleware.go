package server

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"
)

// chain wraps h with the given middlewares, applying the first listed as the
// outermost layer. So chain(h, logging, recoverer) runs logging → recoverer → h.
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusRecorder wraps http.ResponseWriter to capture the status code for access
// logging.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader records the status code before delegating to the wrapped writer.
func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// withLogging logs one structured line per request at the boundary (§9),
// including method, path, status, and duration. It sits outside recoverer so a
// recovered panic is still logged as a 500.
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.deps.Logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// recoverer is the top-level guard that turns a panic in any handler into a
// logged 500 instead of crashing the process, so one bad request cannot take
// down the server (NFR-R3; the single allowed recover per coding-standards §3).
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.deps.Logger.Error("recovered from panic",
					"path", r.URL.Path,
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				s.serverError(w, fmt.Errorf("panic recovered: %v", rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
