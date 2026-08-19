package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/qutaq/GophProfile/internal/api"
	"github.com/qutaq/GophProfile/internal/handlers"
	"github.com/qutaq/GophProfile/internal/services"
)

func projectWebDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(root, "web")
}

func TestWebUploadPage(t *testing.T) {
	store := newMemAvatarStore()
	storage := newMemObjectStorage()
	svc := services.NewAvatarService(store, storage, nil, 1024*1024, nil, nil)
	webDir := projectWebDir(t)
	web, err := handlers.NewWebHandler(svc, webDir)
	if err != nil {
		t.Fatalf("NewWebHandler: %v", err)
	}

	r := api.NewRouter(api.Handlers{
		Avatars: handlers.NewAvatarHandler(svc),
		Health:  handlers.NewHealthHandler(handlers.HealthDeps{}),
		Web:     web,
		WebDir:  webDir,
	})

	req := httptest.NewRequest(http.MethodGet, "/web/upload", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "GophProfile") || !strings.Contains(body, "dropzone") {
		t.Fatalf("unexpected body: %s", body)
	}
}
