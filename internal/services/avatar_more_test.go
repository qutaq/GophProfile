package services_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/repository"
	"github.com/qutaq/GophProfile/internal/services"
)

func TestAvatarService_GetImageSizesAndList(t *testing.T) {
	store := newMemStore()
	storage := newMemStorage()
	svc := services.NewAvatarService(store, storage, nil, 1024*1024, nil, nil)
	ctx := context.Background()

	payload := jpegBytes()
	result, err := svc.Upload(ctx, services.UploadInput{
		UserID:   "u-size",
		FileName: "a.jpg",
		MimeType: "image/jpeg",
		Content:  bytes.NewReader(payload),
		Size:     int64(len(payload)),
	})
	require.NoError(t, err)

	_, err = svc.GetImage(ctx, result.Avatar.ID, "100x100")
	require.ErrorIs(t, err, domain.ErrThumbnailNotReady)

	_, err = svc.GetImage(ctx, result.Avatar.ID, "512x512")
	require.ErrorIs(t, err, domain.ErrInvalidSize)

	thumb := []byte{0xFF, 0xD8, 0xFF, 0xD9}
	thumbKey := repository.ThumbnailKey(result.Avatar.ID, domain.ThumbnailSize100)
	require.NoError(t, storage.Upload(ctx, thumbKey, "image/jpeg", bytes.NewReader(thumb), int64(len(thumb))))

	avatar, err := store.GetByID(ctx, result.Avatar.ID)
	require.NoError(t, err)
	avatar.ThumbnailS3Keys = domain.ThumbnailKeys{domain.ThumbnailSize100: thumbKey}
	avatar.ProcessingStatus = domain.ProcessingStatusCompleted
	store.avatars[avatar.ID] = avatar

	img, err := svc.GetImage(ctx, result.Avatar.ID, "100x100")
	require.NoError(t, err)
	defer img.Body.Close()
	got, err := io.ReadAll(img.Body)
	require.NoError(t, err)
	require.Equal(t, thumb, got)

	list, err := svc.ListByUserID(ctx, "u-size")
	require.NoError(t, err)
	require.Len(t, list, 1)

	userImg, err := svc.GetUserImage(ctx, "u-size", "original")
	require.NoError(t, err)
	defer userImg.Body.Close()
}

func TestAvatarService_DeleteUserAvatar(t *testing.T) {
	store := newMemStore()
	storage := newMemStorage()
	svc := services.NewAvatarService(store, storage, nil, 1024, nil, nil)
	ctx := context.Background()

	payload := jpegBytes()
	result, err := svc.Upload(ctx, services.UploadInput{
		UserID:   "owner",
		FileName: "a.jpg",
		MimeType: "image/jpeg",
		Content:  bytes.NewReader(payload),
		Size:     int64(len(payload)),
	})
	require.NoError(t, err)

	err = svc.DeleteUserAvatar(ctx, "owner", "other")
	require.ErrorIs(t, err, domain.ErrForbidden)

	err = svc.DeleteUserAvatar(ctx, "owner", "owner")
	require.NoError(t, err)

	_, err = svc.GetByID(ctx, result.Avatar.ID)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAvatarService_MissingUserIDAndEmptyFile(t *testing.T) {
	svc := services.NewAvatarService(newMemStore(), newMemStorage(), nil, 1024, nil, nil)

	_, err := svc.Upload(context.Background(), services.UploadInput{
		FileName: "a.jpg",
		Content:  bytes.NewReader(jpegBytes()),
	})
	require.ErrorIs(t, err, domain.ErrMissingUserID)

	err = svc.Delete(context.Background(), "id", "")
	require.ErrorIs(t, err, domain.ErrMissingUserID)

	_, err = svc.Upload(context.Background(), services.UploadInput{
		UserID:   "u",
		FileName: "a.jpg",
		MimeType: "image/jpeg",
		Content:  bytes.NewReader(nil),
	})
	require.ErrorIs(t, err, domain.ErrEmptyFile)
}

func TestNoopPublisher(t *testing.T) {
	var p services.NoopPublisher
	require.NoError(t, p.PublishUpload(context.Background(), events.AvatarUploadEvent{}))
	require.NoError(t, p.PublishDelete(context.Background(), events.AvatarDeleteEvent{}))
}
