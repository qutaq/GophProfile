package services_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/services"
)

func TestAvatarService_MaxSizeDefaultAndGetByUserID(t *testing.T) {
	store := newMemStore()
	storage := newMemStorage()
	svc := services.NewAvatarService(store, storage, nil, 0)
	require.Equal(t, int64(10*1024*1024), svc.MaxSize())

	ctx := context.Background()
	payload := jpegBytes()
	_, err := svc.Upload(ctx, services.UploadInput{
		UserID:   "mime-user",
		FileName: "a.jpg",
		MimeType: "image/jpeg",
		Content:  bytes.NewReader(payload),
		Size:     int64(len(payload)),
	})
	require.NoError(t, err)

	avatar, err := svc.GetByUserID(ctx, "mime-user")
	require.NoError(t, err)
	require.Equal(t, "mime-user", avatar.UserID)

	_, err = svc.GetByUserID(ctx, "missing")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAvatarService_DetectMIMEFromExtension(t *testing.T) {
	svc := services.NewAvatarService(newMemStore(), newMemStorage(), nil, 1024)
	ctx := context.Background()

	// Bytes that DetectContentType treats as octet-stream; extension decides MIME.
	raw := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}

	_, err := svc.Upload(ctx, services.UploadInput{
		UserID:   "u",
		FileName: "pic.webp",
		MimeType: "application/octet-stream",
		Content:  bytes.NewReader(raw),
		Size:     int64(len(raw)),
	})
	require.NoError(t, err)

	_, err = svc.Upload(ctx, services.UploadInput{
		UserID:   "u",
		FileName: "pic.png",
		MimeType: "",
		Content:  bytes.NewReader(raw),
		Size:     int64(len(raw)),
	})
	require.NoError(t, err)

	_, err = svc.Upload(ctx, services.UploadInput{
		UserID:   "u",
		FileName: "pic.jpeg",
		MimeType: "application/octet-stream",
		Content:  bytes.NewReader(raw),
		Size:     int64(len(raw)),
	})
	require.NoError(t, err)

	_, err = svc.Upload(ctx, services.UploadInput{
		UserID:   "u",
		FileName: "pic.bin",
		MimeType: "application/octet-stream",
		Content:  bytes.NewReader(raw),
		Size:     int64(len(raw)),
	})
	require.ErrorIs(t, err, domain.ErrInvalidFile)
}

func TestAvatarService_DeleteUserAvatarMissingHeader(t *testing.T) {
	svc := services.NewAvatarService(newMemStore(), newMemStorage(), nil, 1024)
	err := svc.DeleteUserAvatar(context.Background(), "u", "")
	require.ErrorIs(t, err, domain.ErrMissingUserID)
}
