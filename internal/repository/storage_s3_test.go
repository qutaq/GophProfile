package repository_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/qutaq/GophProfile/internal/repository"
)

func TestS3Storage_UploadDownloadDelete(t *testing.T) {
	if !integrationEnabled() {
		t.Skip("SKIP_INTEGRATION=1")
	}

	client, bucket := openTestS3(t)
	storage := repository.NewS3Storage(client, bucket)
	ctx := context.Background()

	userID := "s3-user-" + uuid.NewString()
	avatarID := uuid.NewString()
	key := repository.OriginalKey(userID, avatarID)
	payload := []byte("fake-image-bytes")

	if err := storage.Upload(ctx, key, "image/jpeg", bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	rc, contentType, err := storage.Download(ctx, key)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("downloaded body = %q, want %q", got, payload)
	}
	if contentType != "image/jpeg" {
		t.Fatalf("contentType = %q, want image/jpeg", contentType)
	}

	thumbKey := repository.ThumbnailKey(avatarID, "100x100")
	if err := storage.Upload(ctx, thumbKey, "image/jpeg", bytes.NewReader(payload), int64(len(payload))); err != nil {
		t.Fatalf("Upload thumb: %v", err)
	}

	if err := storage.Delete(ctx, key, thumbKey); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, _, err = storage.Download(ctx, key)
	if err == nil {
		t.Fatal("expected download error after delete")
	}
}
