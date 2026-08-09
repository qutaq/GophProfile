package domain

import "testing"

func TestNewAvatarDefaults(t *testing.T) {
	a := NewAvatar("user-1", "pic.jpg", "image/jpeg", "avatars/user-1/id/original", 1024)

	if a.UserID != "user-1" {
		t.Fatalf("UserID = %q", a.UserID)
	}
	if a.UploadStatus != UploadStatusUploading {
		t.Fatalf("UploadStatus = %q", a.UploadStatus)
	}
	if a.ProcessingStatus != ProcessingStatusPending {
		t.Fatalf("ProcessingStatus = %q", a.ProcessingStatus)
	}
	if a.IsDeleted() {
		t.Fatal("new avatar must not be deleted")
	}
	if a.ThumbnailS3Keys == nil {
		t.Fatal("ThumbnailS3Keys must be non-nil")
	}
}

func TestIsDeleted(t *testing.T) {
	a := NewAvatar("u", "f", "image/png", "key", 1)
	if a.IsDeleted() {
		t.Fatal("expected not deleted")
	}

	now := a.CreatedAt
	a.DeletedAt = &now
	if !a.IsDeleted() {
		t.Fatal("expected deleted")
	}
}
