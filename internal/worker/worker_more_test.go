package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/observability"
	"github.com/qutaq/GophProfile/internal/worker"
)

func TestWorker_HandleUploadNotFoundAndBadJSON(t *testing.T) {
	w := worker.New(&memRepo{}, newMemStorage(), nil, nil)

	err := w.HandleUpload(context.Background(), []byte(`{`), "m1")
	require.Error(t, err)

	body, _ := json.Marshal(events.AvatarUploadEvent{
		AvatarID: "missing",
		UserID:   "u",
		S3Key:    "k",
	})
	require.NoError(t, w.HandleUpload(context.Background(), body, "m2"))
}

func TestWorker_HandleUploadDownloadAndResizeErrors(t *testing.T) {
	storage := newMemStorage()
	repo := &memRepo{avatar: &domain.Avatar{
		ID:               "a1",
		UserID:           "u1",
		S3Key:            "missing-key",
		ProcessingStatus: domain.ProcessingStatusProcessing,
		ThumbnailS3Keys:  domain.ThumbnailKeys{},
	}}
	w := worker.New(repo, storage, nil, nil)

	body, _ := json.Marshal(events.AvatarUploadEvent{
		AvatarID: "a1",
		UserID:   "u1",
		S3Key:    "missing-key",
	})
	err := w.HandleUpload(context.Background(), body, "m")
	require.Error(t, err)
	require.Equal(t, domain.ProcessingStatusFailed, repo.avatar.ProcessingStatus)

	bad := []byte("not-an-image")
	require.NoError(t, storage.Upload(context.Background(), "orig", "image/jpeg", bytes.NewReader(bad), int64(len(bad))))
	repo.avatar.ProcessingStatus = domain.ProcessingStatusProcessing
	body, _ = json.Marshal(events.AvatarUploadEvent{
		AvatarID: "a1",
		UserID:   "u1",
		S3Key:    "orig",
	})
	err = w.HandleUpload(context.Background(), body, "m2")
	require.Error(t, err)
	require.Contains(t, err.Error(), "resize")
}

func TestWorker_HandleDeleteBadJSONAndStorageError(t *testing.T) {
	w := worker.New(&memRepo{}, &failingDeleteStorage{memStorage: *newMemStorage()}, nil, nil)

	err := w.HandleDelete(context.Background(), []byte(`{`), "m")
	require.Error(t, err)

	body, _ := json.Marshal(events.AvatarDeleteEvent{AvatarID: "a", S3Keys: []string{"k"}})
	err = w.HandleDelete(context.Background(), body, "m2")
	require.Error(t, err)
	require.Contains(t, err.Error(), "delete s3 objects")
}

func TestWorker_HandleUploadFailedLogsEventFields(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.NewLoggerTo(&buf, "info", "gophprofile-worker")
	storage := newMemStorage()
	repo := &memRepo{avatar: &domain.Avatar{
		ID:               "a1",
		UserID:           "u1",
		S3Key:            "missing-key",
		ProcessingStatus: domain.ProcessingStatusProcessing,
		ThumbnailS3Keys:  domain.ThumbnailKeys{},
	}}
	w := worker.New(repo, storage, logger, nil)

	body, err := json.Marshal(events.AvatarUploadEvent{
		AvatarID: "a1",
		UserID:   "u1",
		S3Key:    "missing-key",
	})
	require.NoError(t, err)
	require.Error(t, w.HandleUpload(context.Background(), body, "msg-fail"))

	got := buf.String()
	require.Contains(t, got, `"msg":"handle upload failed"`)
	require.Contains(t, got, `"avatar_id":"a1"`)
	require.Contains(t, got, `"user_id":"u1"`)
	require.Contains(t, got, `"message_id":"msg-fail"`)
}

func TestWorker_HandleDeleteFailedLogsEventFields(t *testing.T) {
	var buf bytes.Buffer
	logger := observability.NewLoggerTo(&buf, "info", "gophprofile-worker")
	w := worker.New(&memRepo{}, &failingDeleteStorage{memStorage: *newMemStorage()}, logger, nil)

	body, err := json.Marshal(events.AvatarDeleteEvent{AvatarID: "avatar-del", S3Keys: []string{"k"}})
	require.NoError(t, err)
	require.Error(t, w.HandleDelete(context.Background(), body, "msg-del"))

	got := buf.String()
	require.Contains(t, got, `"msg":"handle delete failed"`)
	require.Contains(t, got, `"avatar_id":"avatar-del"`)
	require.Contains(t, got, `"message_id":"msg-del"`)
}

type failingDeleteStorage struct {
	memStorage
}

func (f *failingDeleteStorage) Delete(context.Context, ...string) error {
	return errors.New("boom")
}
