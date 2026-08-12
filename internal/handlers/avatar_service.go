package handlers

import (
	"context"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/services"
)

// AvatarService is the subset of avatar operations used by HTTP handlers.
type AvatarService interface {
	MaxSize() int64
	Upload(ctx context.Context, in services.UploadInput) (*services.UploadResult, error)
	GetByID(ctx context.Context, id string) (*domain.Avatar, error)
	ListByUserID(ctx context.Context, userID string) ([]domain.Avatar, error)
	GetImage(ctx context.Context, id, size string) (*services.ImageContent, error)
	GetUserImage(ctx context.Context, userID, size string) (*services.ImageContent, error)
	Delete(ctx context.Context, avatarID, userID string) error
	DeleteUserAvatar(ctx context.Context, pathUserID, headerUserID string) error
}
