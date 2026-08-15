package observability

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func HTTPMetrics(metricsPath string) func(http.Handler) http.Handler {
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipHTTPMetrics(r.URL.Path, metricsPath) {
				next.ServeHTTP(w, r)
				return
			}

			httpRequestsInFlight.Inc()
			defer httpRequestsInFlight.Dec()

			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			route := routePattern(r)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			httpRequestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(status)).Inc()
			httpRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
		})
	}
}

func skipHTTPMetrics(path, metricsPath string) bool {
	return path == metricsPath || strings.HasPrefix(path, "/web/static")
}

// AccessLog writes a JSON access line (method, route, status, duration) with trace_id.
// /metrics and /web/static are skipped. Logger is stored on the request context.
func AccessLog(logger *slog.Logger, metricsPath string) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(WithLogger(r.Context(), logger))
			if skipHTTPMetrics(r.URL.Path, metricsPath) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			attrs := []any{
				"method", r.Method,
				"route", routePattern(r),
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

func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if p := rctx.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}
