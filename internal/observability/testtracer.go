package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
)

// NewTestTracer installs an in-memory TracerProvider and W3C propagator.
// Tests must call the returned cleanup (typically via t.Cleanup).
func NewTestTracer() (*tracetest.SpanRecorder, func()) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return recorder, func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(noop.NewTracerProvider())
	}
}

func EndedSpan(rec *tracetest.SpanRecorder, name string) sdktrace.ReadOnlySpan {
	for _, s := range rec.Ended() {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

func SpanAttr(span sdktrace.ReadOnlySpan, key string) string {
	if span == nil {
		return ""
	}
	for _, a := range span.Attributes() {
		if string(a.Key) == key {
			return a.Value.AsString()
		}
	}
	return ""
}

func CountSpans(rec *tracetest.SpanRecorder, name string) int {
	n := 0
	for _, s := range rec.Ended() {
		if s.Name() == name {
			n++
		}
	}
	return n
}
