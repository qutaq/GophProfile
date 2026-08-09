package handlers_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/api"
	"github.com/qutaq/GophProfile/internal/handlers"
	"github.com/qutaq/GophProfile/internal/services"
)

func TestWebUpload(t *testing.T) {
	store := newMemAvatarStore()
	storage := newMemObjectStorage()
	svc := services.NewAvatarService(store, storage, nil, 1024*1024)
	webDir := projectWebDir(t)
	web, err := handlers.NewWebHandler(svc, webDir)
	require.NoError(t, err)

	r := api.NewRouter(api.Handlers{
		Avatars: handlers.NewAvatarHandler(svc),
		Health:  handlers.NewHealthHandler(handlers.HealthDeps{}),
		Web:     web,
		WebDir:  webDir,
	})

	// Missing user id.
	req := httptest.NewRequest(http.MethodPost, "/web/upload", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Successful upload via form user_id.
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	require.NoError(t, w.WriteField("user_id", "web-user"))
	part, err := w.CreateFormFile("file", "avatar.jpg")
	require.NoError(t, err)
	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0xFF, 0xD9}
	_, err = part.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req = httptest.NewRequest(http.MethodPost, "/web/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, "web-user", created["user_id"])
	require.NotEmpty(t, created["id"])

	// Missing file.
	body = &bytes.Buffer{}
	w = multipart.NewWriter(body)
	require.NoError(t, w.WriteField("user_id", "web-user"))
	require.NoError(t, w.Close())
	req = httptest.NewRequest(http.MethodPost, "/web/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNewWebHandlerInvalidDir(t *testing.T) {
	svc := services.NewAvatarService(newMemAvatarStore(), newMemObjectStorage(), nil, 1024)
	_, err := handlers.NewWebHandler(svc, t.TempDir())
	require.Error(t, err)
}
