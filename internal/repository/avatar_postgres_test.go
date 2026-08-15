package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/repository"
)

func TestAvatarRepository_CRUDAndSoftDelete(t *testing.T) {
	if !integrationEnabled() {
		t.Skip("SKIP_INTEGRATION=1")
	}

	db := openTestDB(t)
	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()
	userID := "test-user-" + uuid.NewString()
	cleanupUserAvatars(t, db, userID)

	avatar := domain.NewAvatar(
		userID,
		"avatar.jpg",
		"image/jpeg",
		repository.OriginalKey(userID, "pending"),
		2048,
	)
	avatar.ID = uuid.NewString()
	avatar.S3Key = repository.OriginalKey(userID, avatar.ID)

	if err := repo.Create(ctx, avatar); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, avatar.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != userID || got.FileName != "avatar.jpg" || got.SizeBytes != 2048 {
		t.Fatalf("unexpected avatar: %+v", got)
	}
	if got.UploadStatus != domain.UploadStatusUploading {
		t.Fatalf("upload status = %q", got.UploadStatus)
	}

	if err := repo.UpdateUploadStatus(ctx, avatar.ID, domain.UploadStatusUploaded); err != nil {
		t.Fatalf("UpdateUploadStatus: %v", err)
	}
	if err := repo.UpdateProcessingStatus(ctx, avatar.ID, domain.ProcessingStatusProcessing); err != nil {
		t.Fatalf("UpdateProcessingStatus: %v", err)
	}

	thumbs := domain.ThumbnailKeys{
		domain.ThumbnailSize100: repository.ThumbnailKey(avatar.ID, domain.ThumbnailSize100),
		domain.ThumbnailSize300: repository.ThumbnailKey(avatar.ID, domain.ThumbnailSize300),
	}
	if err := repo.UpdateThumbnails(ctx, avatar.ID, thumbs); err != nil {
		t.Fatalf("UpdateThumbnails: %v", err)
	}
	if err := repo.UpdateProcessingStatus(ctx, avatar.ID, domain.ProcessingStatusCompleted); err != nil {
		t.Fatalf("UpdateProcessingStatus completed: %v", err)
	}

	got, err = repo.GetByID(ctx, avatar.ID)
	if err != nil {
		t.Fatalf("GetByID after updates: %v", err)
	}
	if got.UploadStatus != domain.UploadStatusUploaded {
		t.Fatalf("upload status = %q", got.UploadStatus)
	}
	if got.ProcessingStatus != domain.ProcessingStatusCompleted {
		t.Fatalf("processing status = %q", got.ProcessingStatus)
	}
	if got.ThumbnailS3Keys[domain.ThumbnailSize100] != thumbs[domain.ThumbnailSize100] {
		t.Fatalf("thumbnails = %#v", got.ThumbnailS3Keys)
	}

	second := domain.NewAvatar(userID, "second.png", "image/png", "key-2", 100)
	second.ID = uuid.NewString()
	second.S3Key = repository.OriginalKey(userID, second.ID)
	second.CreatedAt = time.Now().UTC().Add(time.Second)
	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("Create second: %v", err)
	}

	latest, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if latest.ID != second.ID {
		t.Fatalf("GetByUserID got %s, want latest %s", latest.ID, second.ID)
	}

	list, err := repo.ListByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUserID: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListByUserID len = %d, want 2", len(list))
	}

	if err := repo.SoftDelete(ctx, avatar.ID, userID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	_, err = repo.GetByID(ctx, avatar.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("GetByID after delete: %v, want ErrNotFound", err)
	}

	list, err = repo.ListByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUserID after delete: %v", err)
	}
	if len(list) != 1 || list[0].ID != second.ID {
		t.Fatalf("ListByUserID after delete = %+v", list)
	}

	err = repo.SoftDelete(ctx, avatar.ID, userID)
	if !errors.Is(err, domain.ErrAlreadyDeleted) {
		t.Fatalf("second SoftDelete: %v, want ErrAlreadyDeleted", err)
	}

	total, err := repo.SumActiveSizeBytes(ctx)
	if err != nil {
		t.Fatalf("SumActiveSizeBytes: %v", err)
	}
	if total < 100 {
		t.Fatalf("SumActiveSizeBytes = %d, want at least remaining avatar", total)
	}
}
