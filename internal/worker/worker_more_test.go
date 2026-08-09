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
	"github.com/qutaq/GophProfile/internal/worker"
)

func TestWorker_HandleUploadNotFoundAndBadJSON(t *testing.T) {
	w := worker.New(&memRepo{}, newMemStorage())

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
	w := worker.New(repo, storage)

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
	w := worker.New(&memRepo{}, &failingDeleteStorage{memStorage: *newMemStorage()})

	err := w.HandleDelete(context.Background(), []byte(`{`), "m")
	require.Error(t, err)

	body, _ := json.Marshal(events.AvatarDeleteEvent{AvatarID: "a", S3Keys: []string{"k"}})
	err = w.HandleDelete(context.Background(), body, "m2")
	require.Error(t, err)
	require.Contains(t, err.Error(), "delete s3 objects")
}

type failingDeleteStorage struct {
	memStorage
}

func (f *failingDeleteStorage) Delete(context.Context, ...string) error {
	return errors.New("boom")
}
