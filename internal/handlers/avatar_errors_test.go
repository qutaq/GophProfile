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

func TestDeleteForbiddenAndNotFound(t *testing.T) {
	r := newTestRouter(1024 * 1024)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "avatar.jpg")
	require.NoError(t, err)
	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0xFF, 0xD9}
	_, err = part.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/avatars", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-User-ID", "owner")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id, _ := created["id"].(string)

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/avatars/"+id, nil)
	delReq.Header.Set("X-User-ID", "intruder")
	delRec := httptest.NewRecorder()
	r.ServeHTTP(delRec, delReq)
	require.Equal(t, http.StatusForbidden, delRec.Code)

	missReq := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/00000000-0000-0000-0000-000000000000", nil)
	missRec := httptest.NewRecorder()
	r.ServeHTTP(missRec, missReq)
	require.Equal(t, http.StatusNotFound, missRec.Code)

	sizeReq := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/"+id+"?size=999x999", nil)
	sizeRec := httptest.NewRecorder()
	r.ServeHTTP(sizeRec, sizeReq)
	require.Equal(t, http.StatusBadRequest, sizeRec.Code)

	thumbReq := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/"+id+"?size=100x100", nil)
	thumbRec := httptest.NewRecorder()
	r.ServeHTTP(thumbRec, thumbReq)
	require.Equal(t, http.StatusNotFound, thumbRec.Code)
}

func TestWebGalleryPage(t *testing.T) {
	store := newMemAvatarStore()
	storage := newMemObjectStorage()
	svc := services.NewAvatarService(store, storage, nil, 1024*1024, nil, nil)
	webDir := projectWebDir(t)
	web, err := handlers.NewWebHandler(svc, webDir)
	require.NoError(t, err)

	r := api.NewRouter(api.Handlers{
		Avatars: handlers.NewAvatarHandler(svc),
		Health:  handlers.NewHealthHandler(handlers.HealthDeps{}),
		Web:     web,
		WebDir:  webDir,
	})

	req := httptest.NewRequest(http.MethodGet, "/web/gallery/demo-user", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "GophProfile")
	require.Contains(t, rec.Body.String(), "demo-user")
}
