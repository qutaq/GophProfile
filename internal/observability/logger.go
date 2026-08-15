package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// NewLogger returns a JSON slog logger that adds service, trace_id and span_id.
func NewLogger(level, service string) *slog.Logger {
	return NewLoggerTo(os.Stderr, level, service)
}

// NewLoggerTo is like NewLogger but writes to w (useful in tests).
func NewLoggerTo(w io.Writer, level, service string) *slog.Logger {
	return newLogger(w, level, service)
}

func newLogger(w io.Writer, level, service string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}
	h := &traceHandler{inner: slog.NewJSONHandler(w, opts)}
	return slog.New(h).With("service", service)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type traceHandler struct {
	inner slog.Handler
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{inner: h.inner.WithGroup(name)}
}
