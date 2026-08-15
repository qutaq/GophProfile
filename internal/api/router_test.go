package api_test

import (
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
	svc := services.NewAvatarService(stubStore{}, stubStorage{}, nil, 1024, nil)
	h := api.Handlers{
		Avatars: handlers.NewAvatarHandler(svc),
		Health:  handlers.NewHealthHandler(handlers.HealthDeps{}),
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
	svc := services.NewAvatarService(stubStore{}, stubStorage{}, nil, 1024, nil)
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
