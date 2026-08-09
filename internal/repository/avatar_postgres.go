package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/qutaq/GophProfile/internal/domain"
)

type AvatarRepository struct {
	db *sql.DB
}

func NewAvatarRepository(db *sql.DB) *AvatarRepository {
	return &AvatarRepository{db: db}
}

func (r *AvatarRepository) Create(ctx context.Context, avatar *domain.Avatar) error {
	if avatar.ID == "" {
		avatar.ID = uuid.NewString()
	}
	if avatar.ThumbnailS3Keys == nil {
		avatar.ThumbnailS3Keys = domain.ThumbnailKeys{}
	}

	thumbs, err := json.Marshal(avatar.ThumbnailS3Keys)
	if err != nil {
		return fmt.Errorf("marshal thumbnails: %w", err)
	}

	const q = `
		INSERT INTO avatars (
			id, user_id, file_name, mime_type, size_bytes, s3_key,
			thumbnail_s3_keys, upload_status, processing_status, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`

	now := time.Now().UTC()
	if avatar.CreatedAt.IsZero() {
		avatar.CreatedAt = now
	}
	avatar.UpdatedAt = now

	_, err = r.db.ExecContext(
		ctx, q,
		avatar.ID,
		avatar.UserID,
		avatar.FileName,
		avatar.MimeType,
		avatar.SizeBytes,
		avatar.S3Key,
		thumbs,
		avatar.UploadStatus,
		avatar.ProcessingStatus,
		avatar.CreatedAt,
		avatar.UpdatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return domain.ErrConflict
		}
		return fmt.Errorf("insert avatar: %w", err)
	}
	return nil
}

func (r *AvatarRepository) GetByID(ctx context.Context, id string) (*domain.Avatar, error) {
	const q = `
		SELECT id, user_id, file_name, mime_type, size_bytes, s3_key,
		       thumbnail_s3_keys, upload_status, processing_status,
		       created_at, updated_at, deleted_at
		FROM avatars
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.scanOne(ctx, q, id)
}

// GetByUserID returns the latest non-deleted avatar for a user.
func (r *AvatarRepository) GetByUserID(ctx context.Context, userID string) (*domain.Avatar, error) {
	const q = `
		SELECT id, user_id, file_name, mime_type, size_bytes, s3_key,
		       thumbnail_s3_keys, upload_status, processing_status,
		       created_at, updated_at, deleted_at
		FROM avatars
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanOne(ctx, q, userID)
}

func (r *AvatarRepository) ListByUserID(ctx context.Context, userID string) ([]domain.Avatar, error) {
	const q = `
		SELECT id, user_id, file_name, mime_type, size_bytes, s3_key,
		       thumbnail_s3_keys, upload_status, processing_status,
		       created_at, updated_at, deleted_at
		FROM avatars
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list avatars: %w", err)
	}
	defer rows.Close()

	result := make([]domain.Avatar, 0)
	for rows.Next() {
		avatar, err := scanAvatar(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *avatar)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate avatars: %w", err)
	}
	return result, nil
}

func (r *AvatarRepository) SoftDelete(ctx context.Context, id, userID string) error {
	const q = `
		UPDATE avatars
		SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`

	res, err := r.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return fmt.Errorf("soft delete avatar: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		var deletedAt sql.NullTime
		err := r.db.QueryRowContext(
			ctx,
			`SELECT deleted_at FROM avatars WHERE id = $1 AND user_id = $2`,
			id, userID,
		).Scan(&deletedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lookup avatar for delete: %w", err)
		}
		if deletedAt.Valid {
			return domain.ErrAlreadyDeleted
		}
		return domain.ErrNotFound
	}
	return nil
}

func (r *AvatarRepository) UpdateUploadStatus(ctx context.Context, id string, status domain.UploadStatus) error {
	const q = `
		UPDATE avatars
		SET upload_status = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.execAffectingOne(ctx, q, id, status)
}

func (r *AvatarRepository) UpdateProcessingStatus(ctx context.Context, id string, status domain.ProcessingStatus) error {
	const q = `
		UPDATE avatars
		SET processing_status = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.execAffectingOne(ctx, q, id, status)
}

func (r *AvatarRepository) UpdateThumbnails(ctx context.Context, id string, keys domain.ThumbnailKeys) error {
	if keys == nil {
		keys = domain.ThumbnailKeys{}
	}
	raw, err := json.Marshal(keys)
	if err != nil {
		return fmt.Errorf("marshal thumbnails: %w", err)
	}

	const q = `
		UPDATE avatars
		SET thumbnail_s3_keys = $2, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	return r.execAffectingOne(ctx, q, id, raw)
}

func (r *AvatarRepository) execAffectingOne(ctx context.Context, q string, args ...any) error {
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("update avatar: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AvatarRepository) scanOne(ctx context.Context, q string, args ...any) (*domain.Avatar, error) {
	row := r.db.QueryRowContext(ctx, q, args...)
	avatar, err := scanAvatar(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return avatar, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanAvatar(row scannable) (*domain.Avatar, error) {
	var (
		avatar     domain.Avatar
		thumbsRaw  []byte
		deletedAt  sql.NullTime
		upload     string
		processing string
	)

	err := row.Scan(
		&avatar.ID,
		&avatar.UserID,
		&avatar.FileName,
		&avatar.MimeType,
		&avatar.SizeBytes,
		&avatar.S3Key,
		&thumbsRaw,
		&upload,
		&processing,
		&avatar.CreatedAt,
		&avatar.UpdatedAt,
		&deletedAt,
	)
	if err != nil {
		return nil, err
	}

	avatar.UploadStatus = domain.UploadStatus(upload)
	avatar.ProcessingStatus = domain.ProcessingStatus(processing)
	avatar.ThumbnailS3Keys = domain.ThumbnailKeys{}
	if len(thumbsRaw) > 0 && string(thumbsRaw) != "null" {
		if err := json.Unmarshal(thumbsRaw, &avatar.ThumbnailS3Keys); err != nil {
			return nil, fmt.Errorf("unmarshal thumbnails: %w", err)
		}
	}
	if deletedAt.Valid {
		t := deletedAt.Time.UTC()
		avatar.DeletedAt = &t
	}
	avatar.CreatedAt = avatar.CreatedAt.UTC()
	avatar.UpdatedAt = avatar.UpdatedAt.UTC()
	return &avatar, nil
}
