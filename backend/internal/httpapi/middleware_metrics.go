package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// instrument records request count and latency per route pattern. It is a
// no-op when no metrics registry was provided.
func (h *Handler) instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.metrics == nil {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(wrapped, r)
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = "unmatched"
		}
		h.metrics.IncHTTP(r.Method, route, wrapped.Status())
		h.metrics.ObserveHTTPDuration(route, time.Since(start).Seconds())
	})
}
