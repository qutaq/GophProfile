package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qutaq/GophProfile/internal/api"
	"github.com/qutaq/GophProfile/internal/handlers"
	"github.com/qutaq/GophProfile/internal/services"
)

func newTestRouter(maxSize int64) http.Handler {
	store := newMemAvatarStore()
	storage := newMemObjectStorage()
	svc := services.NewAvatarService(store, storage, nil, maxSize, nil, nil)
	return api.NewRouter(api.Handlers{
		Avatars: handlers.NewAvatarHandler(svc),
		Health:  handlers.NewHealthHandler(handlers.HealthDeps{}),
	})
}

func TestHealthRouteExists(t *testing.T) {
	r := newTestRouter(1024)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	// Without live deps health reports degraded/503, but route must respond with JSON.
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestUploadGetMetadataDeleteFlow(t *testing.T) {
	r := newTestRouter(1024 * 1024)

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "avatar.jpg")
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, 0xFF, 0xD9}
	if _, err := part.Write(payload); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/avatars", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-User-ID", "user-42")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d body=%s", rec.Code, rec.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id, _ := created["id"].(string)
	if id == "" || created["status"] != "processing" {
		t.Fatalf("unexpected create response: %#v", created)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/"+id, nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d", getRec.Code)
	}
	got, _ := io.ReadAll(getRec.Body)
	if !bytes.Equal(got, payload) {
		t.Fatalf("image bytes mismatch")
	}

	metaReq := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/"+id+"/metadata", nil)
	metaRec := httptest.NewRecorder()
	r.ServeHTTP(metaRec, metaReq)
	if metaRec.Code != http.StatusOK {
		t.Fatalf("metadata status = %d", metaRec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-42/avatars", nil)
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d", listRec.Code)
	}

	delReq := httptest.NewRequest(http.MethodDelete, "/api/v1/avatars/"+id, nil)
	delReq.Header.Set("X-User-ID", "user-42")
	delRec := httptest.NewRecorder()
	r.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", delRec.Code)
	}

	getReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/avatars/"+id, nil)
	getRec2 := httptest.NewRecorder()
	r.ServeHTTP(getRec2, getReq2)
	if getRec2.Code != http.StatusNotFound {
		t.Fatalf("get after delete status = %d", getRec2.Code)
	}
}

func TestUploadValidationErrors(t *testing.T) {
	r := newTestRouter(32)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/avatars", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing user status = %d", rec.Code)
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("file", "note.txt")
	_, _ = part.Write([]byte("hello"))
	_ = w.Close()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/avatars", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-User-ID", "u1")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid format status = %d body=%s", rec.Code, rec.Body.String())
	}
}
