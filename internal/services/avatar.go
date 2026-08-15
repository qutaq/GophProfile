package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/observability"
	"github.com/qutaq/GophProfile/internal/repository"
)

type AvatarStore interface {
	Create(ctx context.Context, avatar *domain.Avatar) error
	GetByID(ctx context.Context, id string) (*domain.Avatar, error)
	GetByUserID(ctx context.Context, userID string) (*domain.Avatar, error)
	ListByUserID(ctx context.Context, userID string) ([]domain.Avatar, error)
	SoftDelete(ctx context.Context, id, userID string) error
	UpdateUploadStatus(ctx context.Context, id string, status domain.UploadStatus) error
	UpdateProcessingStatus(ctx context.Context, id string, status domain.ProcessingStatus) error
}

type ObjectStorage interface {
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Download(ctx context.Context, key string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, keys ...string) error
}

type AvatarService struct {
	repo       AvatarStore
	storage    ObjectStorage
	publisher  EventPublisher
	maxSize    int64
	publicBase string
	log        *slog.Logger
}

func NewAvatarService(repo AvatarStore, storage ObjectStorage, publisher EventPublisher, maxSize int64, logger *slog.Logger) *AvatarService {
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	if publisher == nil {
		publisher = NoopPublisher{}
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &AvatarService{
		repo:       repo,
		storage:    storage,
		publisher:  publisher,
		maxSize:    maxSize,
		publicBase: "/api/v1/avatars",
		log:        logger.With("component", "avatar-service"),
	}
}

type UploadInput struct {
	UserID   string
	FileName string
	MimeType string
	Content  io.Reader
	Size     int64
}

type UploadResult struct {
	Avatar *domain.Avatar
	URL    string
	Status string
}

func (s *AvatarService) Upload(ctx context.Context, in UploadInput) (*UploadResult, error) {
	ctx, span := observability.StartSpan(ctx, "avatar.upload",
		attribute.String("user.id", in.UserID),
		attribute.String("file.name", in.FileName),
		attribute.Int64("file.size", in.Size),
	)
	defer span.End()

	start := time.Now()
	result, err := s.upload(ctx, in)
	status := classifyUpload(err)
	observability.ObserveUpload(status, time.Since(start))
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}
	return result, nil
}

func (s *AvatarService) upload(ctx context.Context, in UploadInput) (*UploadResult, error) {
	if strings.TrimSpace(in.UserID) == "" {
		return nil, domain.ErrMissingUserID
	}
	if in.Size > s.maxSize {
		return nil, domain.ErrFileTooLarge
	}

	data, err := io.ReadAll(io.LimitReader(in.Content, s.maxSize+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if int64(len(data)) == 0 {
		return nil, domain.ErrEmptyFile
	}
	if int64(len(data)) > s.maxSize {
		return nil, domain.ErrFileTooLarge
	}

	mimeType := detectMIME(in.FileName, in.MimeType, data)
	if !isAllowedMIME(mimeType) {
		return nil, domain.ErrInvalidFile
	}

	avatarID := uuid.NewString()
	s3Key := repository.OriginalKey(in.UserID, avatarID)
	avatar := domain.NewAvatar(in.UserID, in.FileName, mimeType, s3Key, int64(len(data)))
	avatar.ID = avatarID
	avatar.ProcessingStatus = domain.ProcessingStatusProcessing

	if err := s.repo.Create(ctx, avatar); err != nil {
		return nil, fmt.Errorf("create avatar: %w", err)
	}

	s.log.InfoContext(ctx, "uploading avatar",
		"user_id", in.UserID,
		"avatar_id", avatarID,
		"file_size", int64(len(data)),
		"mime_type", mimeType,
		"file_name", in.FileName,
	)

	if err := s.storage.Upload(ctx, s3Key, mimeType, bytes.NewReader(data), int64(len(data))); err != nil {
		_ = s.repo.UpdateUploadStatus(ctx, avatarID, domain.UploadStatusFailed)
		_ = s.repo.UpdateProcessingStatus(ctx, avatarID, domain.ProcessingStatusFailed)
		return nil, fmt.Errorf("upload to storage: %w", err)
	}

	if err := s.repo.UpdateUploadStatus(ctx, avatarID, domain.UploadStatusUploaded); err != nil {
		return nil, fmt.Errorf("update upload status: %w", err)
	}
	avatar.UploadStatus = domain.UploadStatusUploaded

	if err := s.publisher.PublishUpload(ctx, events.AvatarUploadEvent{
		AvatarID: avatarID,
		UserID:   in.UserID,
		S3Key:    s3Key,
	}); err != nil {
		s.log.ErrorContext(ctx, "publish upload event", "avatar_id", avatarID, "user_id", in.UserID, "err", err)
	}

	return &UploadResult{
		Avatar: avatar,
		URL:    s.avatarURL(avatar.ID),
		Status: string(domain.ProcessingStatusProcessing),
	}, nil
}

func (s *AvatarService) GetByID(ctx context.Context, id string) (*domain.Avatar, error) {
	ctx, span := observability.StartSpan(ctx, "avatar.metadata",
		attribute.String("avatar.id", id),
	)
	defer span.End()

	avatar, err := s.repo.GetByID(ctx, id)
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}
	return avatar, nil
}

func (s *AvatarService) GetByUserID(ctx context.Context, userID string) (*domain.Avatar, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *AvatarService) ListByUserID(ctx context.Context, userID string) ([]domain.Avatar, error) {
	ctx, span := observability.StartSpan(ctx, "avatar.list",
		attribute.String("user.id", userID),
	)
	defer span.End()

	avatars, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}
	return avatars, nil
}

type ImageContent struct {
	Body        io.ReadCloser
	ContentType string
	FileName    string
}

func (s *AvatarService) GetImage(ctx context.Context, id, size string) (*ImageContent, error) {
	ctx, span := observability.StartSpan(ctx, "avatar.get",
		attribute.String("avatar.id", id),
		attribute.String("image.size", size),
	)
	defer span.End()

	avatar, err := s.repo.GetByID(ctx, id)
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}
	img, err := s.openImage(ctx, avatar, size)
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}
	return img, nil
}

func (s *AvatarService) GetUserImage(ctx context.Context, userID, size string) (*ImageContent, error) {
	ctx, span := observability.StartSpan(ctx, "avatar.get",
		attribute.String("user.id", userID),
		attribute.String("image.size", size),
	)
	defer span.End()

	avatar, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}
	img, err := s.openImage(ctx, avatar, size)
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}
	return img, nil
}

func (s *AvatarService) openImage(ctx context.Context, avatar *domain.Avatar, size string) (*ImageContent, error) {
	key := avatar.S3Key
	contentType := avatar.MimeType

	size = strings.TrimSpace(size)
	if size == "" || size == "original" {
		// keep original
	} else if size == domain.ThumbnailSize100 || size == domain.ThumbnailSize300 {
		thumbKey, ok := avatar.ThumbnailS3Keys[size]
		if !ok || thumbKey == "" {
			return nil, domain.ErrThumbnailNotReady
		}
		key = thumbKey
		contentType = "image/jpeg"
	} else {
		return nil, domain.ErrInvalidSize
	}

	rc, detectedType, err := s.storage.Download(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("download avatar: %w", err)
	}
	if detectedType != "" {
		contentType = detectedType
	}
	return &ImageContent{
		Body:        rc,
		ContentType: contentType,
		FileName:    avatar.FileName,
	}, nil
}

func (s *AvatarService) Delete(ctx context.Context, avatarID, userID string) error {
	ctx, span := observability.StartSpan(ctx, "avatar.delete",
		attribute.String("avatar.id", avatarID),
		attribute.String("user.id", userID),
	)
	defer span.End()

	err := s.delete(ctx, avatarID, userID)
	observability.ObserveDelete(classifyDelete(err))
	if err != nil {
		observability.RecordError(span, err)
		return err
	}
	return nil
}

func (s *AvatarService) delete(ctx context.Context, avatarID, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return domain.ErrMissingUserID
	}

	avatar, err := s.repo.GetByID(ctx, avatarID)
	if err != nil {
		return err
	}
	if avatar.UserID != userID {
		return domain.ErrForbidden
	}

	keys := collectS3Keys(avatar)
	if err := s.repo.SoftDelete(ctx, avatarID, userID); err != nil {
		return err
	}

	s.log.InfoContext(ctx, "deleting avatar", "avatar_id", avatarID, "user_id", userID)

	if err := s.publisher.PublishDelete(ctx, events.AvatarDeleteEvent{
		AvatarID: avatarID,
		S3Keys:   keys,
	}); err != nil {
		s.log.ErrorContext(ctx, "publish delete event", "avatar_id", avatarID, "user_id", userID, "err", err)
	}
	return nil
}

func (s *AvatarService) DeleteUserAvatar(ctx context.Context, pathUserID, headerUserID string) error {
	if strings.TrimSpace(headerUserID) == "" {
		observability.ObserveDelete(observability.StatusRejected)
		return domain.ErrMissingUserID
	}
	if pathUserID != headerUserID {
		observability.ObserveDelete(observability.StatusRejected)
		return domain.ErrForbidden
	}

	avatar, err := s.repo.GetByUserID(ctx, pathUserID)
	if err != nil {
		observability.ObserveDelete(classifyDelete(err))
		return err
	}
	return s.Delete(ctx, avatar.ID, headerUserID)
}

func collectS3Keys(avatar *domain.Avatar) []string {
	keys := make([]string, 0, 1+len(avatar.ThumbnailS3Keys))
	if avatar.S3Key != "" {
		keys = append(keys, avatar.S3Key)
	}
	for _, k := range avatar.ThumbnailS3Keys {
		if k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

func (s *AvatarService) avatarURL(id string) string {
	return fmt.Sprintf("%s/%s", s.publicBase, id)
}

func (s *AvatarService) MaxSize() int64 {
	return s.maxSize
}

func detectMIME(fileName, declared string, data []byte) string {
	if declared != "" && declared != "application/octet-stream" {
		return strings.ToLower(declared)
	}
	detected := http.DetectContentType(data)
	if detected != "application/octet-stream" {
		return detected
	}
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return detected
	}
}

func isAllowedMIME(mimeType string) bool {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func classifyUpload(err error) string {
	if err == nil {
		return observability.StatusSuccess
	}
	if errors.Is(err, domain.ErrMissingUserID) ||
		errors.Is(err, domain.ErrFileTooLarge) ||
		errors.Is(err, domain.ErrEmptyFile) ||
		errors.Is(err, domain.ErrInvalidFile) {
		return observability.StatusRejected
	}
	return observability.StatusError
}

func classifyDelete(err error) string {
	if err == nil {
		return observability.StatusSuccess
	}
	if errors.Is(err, domain.ErrMissingUserID) ||
		errors.Is(err, domain.ErrForbidden) ||
		errors.Is(err, domain.ErrNotFound) ||
		errors.Is(err, domain.ErrAlreadyDeleted) {
		return observability.StatusRejected
	}
	return observability.StatusError
}
