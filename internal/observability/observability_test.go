package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/qutaq/GophProfile/internal/config"
)

func TestInitTracerDisabled(t *testing.T) {
	shutdown, err := InitTracer(context.Background(), config.ObservabilityConfig{
		Enabled:     false,
		ServiceName: "test",
	})
	require.NoError(t, err)
	require.NoError(t, shutdown(context.Background()))

	ctx, span := Tracer().Start(context.Background(), "noop")
	defer span.End()
	require.False(t, trace.SpanFromContext(ctx).SpanContext().IsValid())
}

func TestMetricsHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	NewMetrics(nil).Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "go_goroutines")
}

func TestLoggerAddsServiceAndTraceIDs(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, "info", "gophprofile-server")

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	ctx, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	logger.InfoContext(ctx, "hello", "k", "v")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
	require.Equal(t, "hello", payload["msg"])
	require.Equal(t, "gophprofile-server", payload["service"])
	require.Equal(t, "v", payload["k"])
	require.Equal(t, span.SpanContext().TraceID().String(), payload["trace_id"])
	require.Equal(t, span.SpanContext().SpanID().String(), payload["span_id"])
}

func TestLoggerOmitsTraceIDsWithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, "info", "svc")
	logger.InfoContext(context.Background(), "plain")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
	_, hasTrace := payload["trace_id"]
	_, hasSpan := payload["span_id"]
	require.False(t, hasTrace)
	require.False(t, hasSpan)
}

func TestLoggerLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, "error", "svc")
	logger.Info("nope")
	logger.Error("fail")
	require.NotContains(t, buf.String(), "nope")
	require.Contains(t, buf.String(), "fail")
}

func TestParseLevel(t *testing.T) {
	require.Equal(t, slog.LevelDebug, parseLevel("DEBUG"))
	require.Equal(t, slog.LevelWarn, parseLevel("warn"))
	require.Equal(t, slog.LevelError, parseLevel("error"))
	require.Equal(t, slog.LevelInfo, parseLevel("info"))
	require.Equal(t, slog.LevelInfo, parseLevel("unknown"))
}

func TestTraceHandlerWithAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, "info", "svc").With("a", 1).WithGroup("g")
	logger.Info("grouped", "b", 2)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
	require.Equal(t, "grouped", payload["msg"])
	require.Equal(t, float64(1), payload["a"])
	group, ok := payload["g"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(2), group["b"])
}

func TestRecordError(t *testing.T) {
	rec, cleanup := NewTestTracer()
	t.Cleanup(cleanup)

	RecordError(nil, errors.New("ignored"))

	_, span := StartSpan(context.Background(), "op")
	RecordError(span, errors.New("boom"))
	span.End()

	got := EndedSpan(rec, "op")
	require.NotNil(t, got)
	require.Equal(t, codes.Error, got.Status().Code)
	require.Equal(t, "boom", got.Status().Description)
	require.Len(t, got.Events(), 1)
}

func TestStartSpanKind(t *testing.T) {
	rec, cleanup := NewTestTracer()
	t.Cleanup(cleanup)

	_, span := StartSpanKind(context.Background(), "s3.put", trace.SpanKindClient)
	span.End()

	got := EndedSpan(rec, "s3.put")
	require.NotNil(t, got)
	require.Equal(t, trace.SpanKindClient, got.SpanKind())
}

func scrapeMetrics(t *testing.T, m *Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestObserveBusinessMetrics(t *testing.T) {
	m := NewTestMetrics()
	m.ObserveUpload(StatusSuccess, 10*time.Millisecond)
	m.ObserveDelete(StatusRejected)
	m.ObserveProcessing(ResultCompleted, 20*time.Millisecond)
	m.ObserveThumbnail("100x100")
	m.ObservePublish("avatar.uploaded")
	m.ObserveConsume("avatars.upload", StatusSuccess)
	m.ConsumerInFlightInc()
	m.ConsumerInFlightDec()

	body := scrapeMetrics(t, m)
	require.Contains(t, body, `avatars_uploads_total{status="success"}`)
	require.Contains(t, body, "avatars_upload_duration_seconds")
	require.Contains(t, body, `avatars_deletes_total{status="rejected"}`)
	require.Contains(t, body, `avatars_processing_total{result="completed"}`)
	require.Contains(t, body, "avatars_processing_duration_seconds")
	require.Contains(t, body, `avatars_thumbnails_generated_total{size="100x100"}`)
	require.Contains(t, body, `rabbitmq_messages_published_total{routing_key="avatar.uploaded"}`)
	require.Contains(t, body, `rabbitmq_messages_consumed_total{queue="avatars.upload",result="success"}`)
	require.Contains(t, body, "rabbitmq_consumer_in_flight")

	other := scrapeMetrics(t, NewTestMetrics())
	require.NotContains(t, other, `avatars_uploads_total{status="success"}`)
}

func TestNilMetricsMethodsDoNotPanic(t *testing.T) {
	var m *Metrics
	m.ObserveUpload(StatusSuccess, time.Millisecond)
	m.ObserveDelete(StatusRejected)
	m.ObserveProcessing(ResultCompleted, time.Millisecond)
	m.ObserveThumbnail("100x100")
	m.ObservePublish("avatar.uploaded")
	m.ObserveConsume("avatars.upload", StatusSuccess)
	m.ConsumerInFlightInc()
	m.ConsumerInFlightDec()
	m.HTTPInFlightInc()
	m.HTTPInFlightDec()
	m.ObserveHTTP(http.MethodGet, "/health", http.StatusOK, 0.01)
	m.RegisterDBPool(nil)
	m.CollectStorageBytes(context.Background(), nil, 0, nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestSkipHTTPMetrics(t *testing.T) {
	require.True(t, skipHTTPMetrics("/metrics", "/metrics"))
	require.True(t, skipHTTPMetrics("/web/static/app.css", "/metrics"))
	require.False(t, skipHTTPMetrics("/health", "/metrics"))
	require.False(t, skipHTTPMetrics("/api/v1/avatars", "/metrics"))
}

func TestCollectStorageBytes(t *testing.T) {
	m := NewTestMetrics()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.CollectStorageBytes(ctx, func(context.Context) (int64, error) {
			return 42, nil
		}, 15*time.Millisecond, slog.New(slog.DiscardHandler))
		close(done)
	}()
	time.Sleep(40 * time.Millisecond)
	cancel()
	<-done

	require.Contains(t, scrapeMetrics(t, m), "avatars_storage_bytes 42")
}

func TestCollectStorageBytesQueryError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	NewTestMetrics().CollectStorageBytes(ctx, func(context.Context) (int64, error) {
		return 0, errors.New("db down")
	}, 5*time.Millisecond, slog.New(slog.DiscardHandler))
}

func TestNewMetricsServer(t *testing.T) {
	srv := NewMetricsServer("127.0.0.1:0", "/metrics", NewMetrics(nil))
	require.NotNil(t, srv)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "go_goroutines")
}

func TestHTTPMiddlewareIncludesRouteStatusAndTrace(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, "info", "gophprofile-server")

	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	r := chi.NewRouter()
	r.Use(HTTPMiddleware(logger, "/metrics", nil))
	r.Get("/api/v1/users/{user_id}/avatars", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})

	ctx, span := tp.Tracer("test").Start(context.Background(), "http")
	defer span.End()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/avatars", nil)
	req = req.WithContext(ctx)
	req.Header.Set("X-User-ID", "u1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	payload := lastJSONLine(t, buf.Bytes())
	require.Equal(t, "http request", payload["msg"])
	require.Equal(t, "GET", payload["method"])
	require.Equal(t, "/api/v1/users/{user_id}/avatars", payload["route"])
	require.Equal(t, float64(http.StatusCreated), payload["status"])
	require.Equal(t, "u1", payload["user_id"])
	require.Equal(t, "gophprofile-server", payload["service"])
	require.Equal(t, span.SpanContext().TraceID().String(), payload["trace_id"])
	require.Equal(t, span.SpanContext().SpanID().String(), payload["span_id"])
	_, hasDuration := payload["duration_ms"]
	require.True(t, hasDuration)
}

func TestHTTPMiddlewareSkipsMetricsAndStatic(t *testing.T) {
	var buf bytes.Buffer
	logger := newLogger(&buf, "info", "svc")

	r := chi.NewRouter()
	r.Use(HTTPMiddleware(logger, "/metrics", nil))
	r.Get("/metrics", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/web/static/app.css", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	for _, path := range []string{"/metrics", "/web/static/app.css"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	require.Empty(t, buf.String())

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	payload := lastJSONLine(t, buf.Bytes())
	require.Equal(t, "http request", payload["msg"])
	require.Equal(t, "/health", payload["route"])
}

func TestHTTPMiddlewareNilLoggerDoesNotPanic(t *testing.T) {
	r := chi.NewRouter()
	r.Use(HTTPMiddleware(nil, "", nil))
	r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func lastJSONLine(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	raw = bytes.TrimSpace(raw)
	require.NotEmpty(t, raw)
	line := raw
	if i := bytes.LastIndexByte(raw, '\n'); i >= 0 {
		line = raw[i+1:]
	}
	var payload map[string]any
	require.NoError(t, json.Unmarshal(line, &payload))
	return payload
}
