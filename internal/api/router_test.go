package api_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/api"
	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/handlers"
	"github.com/qutaq/GophProfile/internal/observability"
	"github.com/qutaq/GophProfile/internal/services"
)

type stubStore struct{}

func (stubStore) Create(context.Context, *domain.Avatar) error { return nil }
func (stubStore) GetByID(context.Context, string) (*domain.Avatar, error) {
	return nil, domain.ErrNotFound
}
func (stubStore) GetByUserID(context.Context, string) (*domain.Avatar, error) {
	return nil, domain.ErrNotFound
}
func (stubStore) ListByUserID(context.Context, string) ([]domain.Avatar, error) {
	return nil, nil
}
func (stubStore) SoftDelete(context.Context, string, string) error { return domain.ErrNotFound }
func (stubStore) UpdateUploadStatus(context.Context, string, domain.UploadStatus) error {
	return nil
}
func (stubStore) UpdateProcessingStatus(context.Context, string, domain.ProcessingStatus) error {
	return nil
}

type stubStorage struct{}

func (stubStorage) Upload(context.Context, string, string, io.Reader, int64) error { return nil }
func (stubStorage) Download(context.Context, string) (io.ReadCloser, string, error) {
	return nil, "", errors.New("not found")
}
func (stubStorage) Delete(context.Context, ...string) error { return nil }

func newTestHandlers(t *testing.T, webDir string) api.Handlers {
	t.Helper()
	svc := services.NewAvatarService(stubStore{}, stubStorage{}, nil, 1024, nil, nil)
	h := api.Handlers{
		Avatars: handlers.NewAvatarHandler(svc),
		Health:  handlers.NewHealthHandler(handlers.HealthDeps{}),
		Metrics: observability.NewMetrics(nil),
	}
	if webDir != "" {
		web, err := handlers.NewWebHandler(svc, webDir)
		require.NoError(t, err)
		h.Web = web
		h.WebDir = webDir
	}
	return h
}

func prepareWebDir(t *testing.T) string {
	t.Helper()
	webDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(webDir, "templates"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(webDir, "static"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(webDir, "templates", "upload.html"),
		[]byte(`{{define "upload.html"}}ok{{end}}`),
		0o644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(webDir, "templates", "gallery.html"),
		[]byte(`{{define "gallery.html"}}gallery{{end}}`),
		0o644,
	))
	require.NoError(t, os.WriteFile(filepath.Join(webDir, "static", "app.css"), []byte("body{}"), 0o644))
	return webDir
}

func TestNewRouterWithoutWeb(t *testing.T) {
	r := api.NewRouter(newTestHandlers(t, ""))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "go_goroutines")

	req = httptest.NewRequest(http.MethodGet, "/web/upload", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestNewRouterWithWebRedirectsAndStatic(t *testing.T) {
	webDir := prepareWebDir(t)
	r := api.NewRouter(newTestHandlers(t, webDir))

	for _, path := range []string{"/", "/web"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusFound, rec.Code, path)
		require.Equal(t, "/web/upload", rec.Header().Get("Location"), path)
	}

	req := httptest.NewRequest(http.MethodGet, "/web/static/app.css", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "body{}", rec.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/web/upload", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ok", rec.Body.String())
}

func TestNewRouterDefaultWebDir(t *testing.T) {
	webDir := prepareWebDir(t)
	svc := services.NewAvatarService(stubStore{}, stubStorage{}, nil, 1024, nil, nil)
	web, err := handlers.NewWebHandler(svc, webDir)
	require.NoError(t, err)

	r := api.NewRouter(api.Handlers{
		Avatars: handlers.NewAvatarHandler(svc),
		Health:  handlers.NewHealthHandler(handlers.HealthDeps{}),
		Web:     web,
		// WebDir empty → defaults to "web" for static files.
	})

	req := httptest.NewRequest(http.MethodGet, "/web/upload", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAPIRoutesMounted(t *testing.T) {
	r := api.NewRouter(newTestHandlers(t, ""))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/avatars", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "[]")
}

func TestRouterTracesAPIAndSkipsHealthMetrics(t *testing.T) {
	recorder, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	r := api.NewRouter(newTestHandlers(t, ""))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/avatars", nil)
	req.Header.Set("X-User-ID", "u1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	httpSpan := observability.EndedSpan(recorder, "GET /api/v1/users/{user_id}/avatars")
	require.NotNil(t, httpSpan)
	require.Equal(t, "u1", observability.SpanAttr(httpSpan, "user.id"))
	require.Equal(t, "/api/v1/users/{user_id}/avatars", observability.SpanAttr(httpSpan, "http.route"))

	listSpan := observability.EndedSpan(recorder, "avatar.list")
	require.NotNil(t, listSpan)
	require.Equal(t, httpSpan.SpanContext().TraceID(), listSpan.SpanContext().TraceID())

	before := len(recorder.Ended())
	for _, path := range []string{"/health", "/metrics"} {
		req = httptest.NewRequest(http.MethodGet, path, nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
	}
	require.Equal(t, before, len(recorder.Ended()), "health and metrics must not create spans")
}

func TestRouterHTTPMetrics(t *testing.T) {
	webDir := prepareWebDir(t)
	r := api.NewRouter(newTestHandlers(t, webDir))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/avatars", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/web/static/app.css", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `http_requests_total{method="GET",route="/api/v1/users/{user_id}/avatars",status="200"}`)
	require.Contains(t, body, "http_request_duration_seconds")
	require.Contains(t, body, "http_requests_in_flight")
	require.NotContains(t, body, `route="/metrics"`)
	require.NotContains(t, body, `route="/web/static`)
}

func TestRouterAccessLogJSON(t *testing.T) {
	_, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	var buf bytes.Buffer
	logger := observability.NewLoggerTo(&buf, "info", "gophprofile-server")
	h := newTestHandlers(t, "")
	h.Logger = logger
	r := api.NewRouter(h)

	ctx, span := observability.StartSpan(context.Background(), "test")
	defer span.End()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/u1/avatars", nil)
	req = req.WithContext(ctx)
	req.Header.Set("X-User-ID", "u1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	require.Contains(t, buf.String(), `"msg":"http request"`)
	require.Contains(t, buf.String(), `"route":"/api/v1/users/{user_id}/avatars"`)
	require.Contains(t, buf.String(), `"user_id":"u1"`)
	require.Contains(t, buf.String(), `"trace_id":"`)
}
