package events

const (
	RoutingKeyUploaded = "avatar.uploaded"
	RoutingKeyDeleted  = "avatar.deleted"

	QueueUpload = "avatars.upload"
	QueueDelete = "avatars.delete"
)

type AvatarUploadEvent struct {
	AvatarID string `json:"avatar_id"`
	UserID   string `json:"user_id"`
	S3Key    string `json:"s3_key"`
}

type ProcessingOp struct {
	Type string `json:"type"`
	Size string `json:"size,omitempty"`
}

type AvatarProcessEvent struct {
	AvatarID   string         `json:"avatar_id"`
	Operations []ProcessingOp `json:"operations"`
}

type AvatarDeleteEvent struct {
	AvatarID string   `json:"avatar_id"`
	S3Keys   []string `json:"s3_keys"`
}
