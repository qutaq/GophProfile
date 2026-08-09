package handlers_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func uploadJPEG(t *testing.T, r http.Handler, userID string) string {
	t.Helper()
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
	req.Header.Set("X-User-ID", userID)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)
	return id
}

func TestGetByUserIDAndDeleteUserAvatar(t *testing.T) {
	r := newTestRouter(1024 * 1024)
	_ = uploadJPEG(t, r, "user-77")

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-77/avatar", nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)
	got, err := io.ReadAll(getRec.Body)
	require.NoError(t, err)
	require.NotEmpty(t, got)

	missReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/nobody/avatar", nil)
	missRec := httptest.NewRecorder()
	r.ServeHTTP(missRec, missReq)
	require.Equal(t, http.StatusNotFound, missRec.Code)

	delForbidden := httptest.NewRequest(http.MethodDelete, "/api/v1/users/user-77/avatar", nil)
	delForbidden.Header.Set("X-User-ID", "intruder")
	delForbiddenRec := httptest.NewRecorder()
	r.ServeHTTP(delForbiddenRec, delForbidden)
	require.Equal(t, http.StatusForbidden, delForbiddenRec.Code)

	delMissingHeader := httptest.NewRequest(http.MethodDelete, "/api/v1/users/user-77/avatar", nil)
	delMissingRec := httptest.NewRecorder()
	r.ServeHTTP(delMissingRec, delMissingHeader)
	require.Equal(t, http.StatusBadRequest, delMissingRec.Code)

	delOK := httptest.NewRequest(http.MethodDelete, "/api/v1/users/user-77/avatar", nil)
	delOK.Header.Set("X-User-ID", "user-77")
	delOKRec := httptest.NewRecorder()
	r.ServeHTTP(delOKRec, delOK)
	require.Equal(t, http.StatusNoContent, delOKRec.Code)

	getAfter := httptest.NewRequest(http.MethodGet, "/api/v1/users/user-77/avatar", nil)
	getAfterRec := httptest.NewRecorder()
	r.ServeHTTP(getAfterRec, getAfter)
	require.Equal(t, http.StatusNotFound, getAfterRec.Code)
}
