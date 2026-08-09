package domain

import (
	"time"
)

type UploadStatus string

const (
	UploadStatusUploading UploadStatus = "uploading"
	UploadStatusUploaded  UploadStatus = "uploaded"
	UploadStatusFailed    UploadStatus = "failed"
)

type ProcessingStatus string

const (
	ProcessingStatusPending    ProcessingStatus = "pending"
	ProcessingStatusProcessing ProcessingStatus = "processing"
	ProcessingStatusCompleted  ProcessingStatus = "completed"
	ProcessingStatusFailed     ProcessingStatus = "failed"
)

const (
	ThumbnailSize100 = "100x100"
	ThumbnailSize300 = "300x300"
)

// ThumbnailKeys maps thumbnail size label to S3 object key.
type ThumbnailKeys map[string]string

// Avatar is the domain model for a user avatar.
type Avatar struct {
	ID               string
	UserID           string
	FileName         string
	MimeType         string
	SizeBytes        int64
	S3Key            string
	ThumbnailS3Keys  ThumbnailKeys
	UploadStatus     UploadStatus
	ProcessingStatus ProcessingStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
}

func (a *Avatar) IsDeleted() bool {
	return a != nil && a.DeletedAt != nil
}

func NewAvatar(userID, fileName, mimeType, s3Key string, sizeBytes int64) *Avatar {
	now := time.Now().UTC()
	return &Avatar{
		UserID:           userID,
		FileName:         fileName,
		MimeType:         mimeType,
		SizeBytes:        sizeBytes,
		S3Key:            s3Key,
		ThumbnailS3Keys:  ThumbnailKeys{},
		UploadStatus:     UploadStatusUploading,
		ProcessingStatus: ProcessingStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
