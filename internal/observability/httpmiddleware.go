package observability

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// HTTPMiddleware records Prometheus HTTP metrics and writes a JSON access log
// with a single WrapResponseWriter and one next.ServeHTTP call.
// /metrics, /livez, /readyz and /web/static are skipped.
func HTTPMiddleware(logger *slog.Logger, metricsPath string, metrics *Metrics) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipHTTPMetrics(r.URL.Path, metricsPath) {
				next.ServeHTTP(w, r)
				return
			}

			metrics.HTTPInFlightInc()
			defer metrics.HTTPInFlightDec()

			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			route := routePattern(r)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}

			metrics.ObserveHTTP(r.Method, route, status, time.Since(start).Seconds())

			attrs := []any{
				"method", r.Method,
				"route", route,
				"status", status,
				"duration_ms", time.Since(start).Milliseconds(),
				"bytes", ww.BytesWritten(),
			}
			if reqID := middleware.GetReqID(r.Context()); reqID != "" {
				attrs = append(attrs, "request_id", reqID)
			}
			if userID := r.Header.Get("X-User-ID"); userID != "" {
				attrs = append(attrs, "user_id", userID)
			}
			logger.InfoContext(r.Context(), "http request", attrs...)
		})
	}
}

func skipHTTPMetrics(path, metricsPath string) bool {
	return path == metricsPath || path == "/livez" || path == "/readyz" || strings.HasPrefix(path, "/web/static")
}

func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if p := rctx.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}
