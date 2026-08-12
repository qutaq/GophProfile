package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/pkg/imageutil"
	"github.com/qutaq/GophProfile/internal/repository"
)

type AvatarRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Avatar, error)
	UpdateProcessingStatus(ctx context.Context, id string, status domain.ProcessingStatus) error
	UpdateThumbnails(ctx context.Context, id string, keys domain.ThumbnailKeys) error
}

type ObjectStorage interface {
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Download(ctx context.Context, key string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, keys ...string) error
}

type Worker struct {
	repo    AvatarRepository
	storage ObjectStorage
}

func New(repo AvatarRepository, storage ObjectStorage) *Worker {
	return &Worker{repo: repo, storage: storage}
}

func (w *Worker) HandleUpload(ctx context.Context, body []byte, messageID string) error {
	var event events.AvatarUploadEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("unmarshal upload event: %w", err)
	}
	slog.Info("handle upload", "message_id", messageID, "avatar_id", event.AvatarID)

	avatar, err := w.repo.GetByID(ctx, event.AvatarID)
	if errors.Is(err, domain.ErrNotFound) {
		slog.Info("avatar not found, skip", "avatar_id", event.AvatarID)
		return nil
	}
	if err != nil {
		return err
	}

	if avatar.ProcessingStatus == domain.ProcessingStatusCompleted && len(avatar.ThumbnailS3Keys) > 0 {
		slog.Info("avatar already processed, skip", "avatar_id", event.AvatarID)
		return nil
	}

	rc, _, err := w.storage.Download(ctx, event.S3Key)
	if err != nil {
		_ = w.repo.UpdateProcessingStatus(ctx, event.AvatarID, domain.ProcessingStatusFailed)
		return fmt.Errorf("download original: %w", err)
	}
	defer rc.Close()

	original, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("read original: %w", err)
	}

	sizes := []struct {
		label string
		w, h  int
	}{
		{domain.ThumbnailSize100, 100, 100},
		{domain.ThumbnailSize300, 300, 300},
	}

	thumbs := domain.ThumbnailKeys{}
	for _, size := range sizes {
		data, err := imageutil.ResizeJPEG(original, size.w, size.h)
		if err != nil {
			_ = w.repo.UpdateProcessingStatus(ctx, event.AvatarID, domain.ProcessingStatusFailed)
			return fmt.Errorf("resize %s: %w", size.label, err)
		}
		key := repository.ThumbnailKey(event.AvatarID, size.label)
		if err := w.storage.Upload(ctx, key, "image/jpeg", bytes.NewReader(data), int64(len(data))); err != nil {
			_ = w.repo.UpdateProcessingStatus(ctx, event.AvatarID, domain.ProcessingStatusFailed)
			return fmt.Errorf("upload thumbnail %s: %w", size.label, err)
		}
		thumbs[size.label] = key
	}

	if err := w.repo.UpdateThumbnails(ctx, event.AvatarID, thumbs); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	if err := w.repo.UpdateProcessingStatus(ctx, event.AvatarID, domain.ProcessingStatusCompleted); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}

	slog.Info("avatar processing completed", "avatar_id", event.AvatarID)
	return nil
}

func (w *Worker) HandleDelete(ctx context.Context, body []byte, messageID string) error {
	var event events.AvatarDeleteEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("unmarshal delete event: %w", err)
	}
	slog.Info("handle delete",
		"message_id", messageID,
		"avatar_id", event.AvatarID,
		"keys", len(event.S3Keys),
	)

	if err := w.storage.Delete(ctx, event.S3Keys...); err != nil {
		return fmt.Errorf("delete s3 objects: %w", err)
	}
	return nil
}
