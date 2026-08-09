package repository

import "fmt"

// OriginalKey returns S3 key for the original avatar image.
// Format: avatars/{user_id}/{avatar_id}/original
func OriginalKey(userID, avatarID string) string {
	return fmt.Sprintf("avatars/%s/%s/original", userID, avatarID)
}

// ThumbnailKey returns S3 key for a thumbnail.
// Format: thumbnails/{avatar_id}/{size}
func ThumbnailKey(avatarID, size string) string {
	return fmt.Sprintf("thumbnails/%s/%s", avatarID, size)
}
