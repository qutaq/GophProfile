package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/observability"
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
	log     *slog.Logger
	metrics *observability.Metrics
}

func New(repo AvatarRepository, storage ObjectStorage, logger *slog.Logger, metrics *observability.Metrics) *Worker {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Worker{
		repo:    repo,
		storage: storage,
		log:     logger.With("component", "worker"),
		metrics: metrics,
	}
}

func (w *Worker) HandleUpload(ctx context.Context, body []byte, messageID string) error {
	ctx, span := observability.StartSpan(ctx, "worker.handle_upload")
	defer span.End()

	start := time.Now()
	result, err := w.handleUpload(ctx, span, body, messageID)
	if err != nil {
		observability.RecordError(span, err)
		result = observability.ResultFailed
	}
	if result == "" {
		result = observability.ResultCompleted
	}
	w.metrics.ObserveProcessing(result, time.Since(start))
	return err
}

func (w *Worker) handleUpload(ctx context.Context, span trace.Span, body []byte, messageID string) (string, error) {
	var event events.AvatarUploadEvent
	if err := json.Unmarshal(body, &event); err != nil {
		w.log.ErrorContext(ctx, "unmarshal upload event", "message_id", messageID, "err", err)
		return observability.ResultFailed, fmt.Errorf("unmarshal upload event: %w", err)
	}
	span.SetAttributes(
		attribute.String("avatar.id", event.AvatarID),
		attribute.String("user.id", event.UserID),
		attribute.String("messaging.message_id", messageID),
	)
	log := w.log.With(
		"message_id", messageID,
		"avatar_id", event.AvatarID,
		"user_id", event.UserID,
	)
	log.InfoContext(ctx, "handle upload")

	fail := func(err error) (string, error) {
		log.ErrorContext(ctx, "handle upload failed", "err", err)
		return observability.ResultFailed, err
	}

	avatar, err := w.repo.GetByID(ctx, event.AvatarID)
	if errors.Is(err, domain.ErrNotFound) {
		log.InfoContext(ctx, "avatar not found, skip")
		return observability.ResultSkipped, nil
	}
	if err != nil {
		return fail(err)
	}

	if avatar.ProcessingStatus == domain.ProcessingStatusCompleted && len(avatar.ThumbnailS3Keys) > 0 {
		log.InfoContext(ctx, "avatar already processed, skip")
		return observability.ResultSkipped, nil
	}

	rc, _, err := w.storage.Download(ctx, event.S3Key)
	if err != nil {
		_ = w.repo.UpdateProcessingStatus(ctx, event.AvatarID, domain.ProcessingStatusFailed)
		return fail(fmt.Errorf("download original: %w", err))
	}
	defer rc.Close()

	original, err := io.ReadAll(rc)
	if err != nil {
		return fail(fmt.Errorf("read original: %w", err))
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
		data, err := w.resize(ctx, original, size.w, size.h, size.label)
		if err != nil {
			_ = w.repo.UpdateProcessingStatus(ctx, event.AvatarID, domain.ProcessingStatusFailed)
			return fail(fmt.Errorf("resize %s: %w", size.label, err))
		}
		key := repository.ThumbnailKey(event.AvatarID, size.label)
		if err := w.storage.Upload(ctx, key, "image/jpeg", bytes.NewReader(data), int64(len(data))); err != nil {
			_ = w.repo.UpdateProcessingStatus(ctx, event.AvatarID, domain.ProcessingStatusFailed)
			return fail(fmt.Errorf("upload thumbnail %s: %w", size.label, err))
		}
		thumbs[size.label] = key
		w.metrics.ObserveThumbnail(size.label)
	}

	if err := w.repo.UpdateThumbnails(ctx, event.AvatarID, thumbs); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return observability.ResultSkipped, nil
		}
		return fail(err)
	}
	if err := w.repo.UpdateProcessingStatus(ctx, event.AvatarID, domain.ProcessingStatusCompleted); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return observability.ResultSkipped, nil
		}
		return fail(err)
	}

	log.InfoContext(ctx, "avatar processing completed")
	return observability.ResultCompleted, nil
}

func (w *Worker) resize(ctx context.Context, original []byte, width, height int, label string) ([]byte, error) {
	_, span := observability.StartSpan(ctx, "worker.resize",
		attribute.String("thumbnail.size", label),
		attribute.Int("thumbnail.width", width),
		attribute.Int("thumbnail.height", height),
	)
	defer span.End()

	data, err := imageutil.ResizeJPEG(original, width, height)
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}
	return data, nil
}

func (w *Worker) HandleDelete(ctx context.Context, body []byte, messageID string) error {
	ctx, span := observability.StartSpan(ctx, "worker.handle_delete")
	defer span.End()

	if err := w.handleDelete(ctx, span, body, messageID); err != nil {
		observability.RecordError(span, err)
		return err
	}
	return nil
}

func (w *Worker) handleDelete(ctx context.Context, span trace.Span, body []byte, messageID string) error {
	var event events.AvatarDeleteEvent
	if err := json.Unmarshal(body, &event); err != nil {
		w.log.ErrorContext(ctx, "unmarshal delete event", "message_id", messageID, "err", err)
		return fmt.Errorf("unmarshal delete event: %w", err)
	}
	span.SetAttributes(
		attribute.String("avatar.id", event.AvatarID),
		attribute.String("messaging.message_id", messageID),
		attribute.Int("s3.key_count", len(event.S3Keys)),
	)
	log := w.log.With(
		"message_id", messageID,
		"avatar_id", event.AvatarID,
	)
	log.InfoContext(ctx, "handle delete", "keys", len(event.S3Keys))

	if err := w.storage.Delete(ctx, event.S3Keys...); err != nil {
		err = fmt.Errorf("delete s3 objects: %w", err)
		log.ErrorContext(ctx, "handle delete failed", "err", err)
		return err
	}
	return nil
}
