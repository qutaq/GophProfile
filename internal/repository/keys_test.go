package repository

import "testing"

func TestOriginalKey(t *testing.T) {
	got := OriginalKey("user-1", "avatar-1")
	want := "avatars/user-1/avatar-1/original"
	if got != want {
		t.Fatalf("OriginalKey() = %q, want %q", got, want)
	}
}

func TestThumbnailKey(t *testing.T) {
	got := ThumbnailKey("avatar-1", "100x100")
	want := "thumbnails/avatar-1/100x100"
	if got != want {
		t.Fatalf("ThumbnailKey() = %q, want %q", got, want)
	}
}
