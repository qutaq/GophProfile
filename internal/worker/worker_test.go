package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
	"testing"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/repository"
	"github.com/qutaq/GophProfile/internal/worker"
)

type memRepo struct {
	mu     sync.Mutex
	avatar *domain.Avatar
}

func (m *memRepo) GetByID(context.Context, string) (*domain.Avatar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.avatar == nil {
		return nil, domain.ErrNotFound
	}
	cp := *m.avatar
	return &cp, nil
}

func (m *memRepo) UpdateProcessingStatus(_ context.Context, _ string, status domain.ProcessingStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.avatar.ProcessingStatus = status
	return nil
}

func (m *memRepo) UpdateThumbnails(_ context.Context, _ string, keys domain.ThumbnailKeys) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.avatar.ThumbnailS3Keys = keys
	return nil
}

type memStorage struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemStorage() *memStorage {
	return &memStorage{data: make(map[string][]byte)}
}

func (s *memStorage) Upload(_ context.Context, key, _ string, body io.Reader, _ int64) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = b
	return nil
}

func (s *memStorage) Download(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data[key]
	if !ok {
		return nil, "", io.EOF
	}
	return io.NopCloser(bytes.NewReader(b)), "image/png", nil
}

func (s *memStorage) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range keys {
		delete(s.data, k)
	}
	return nil
}

func TestWorker_HandleUploadIdempotent(t *testing.T) {
	storage := newMemStorage()
	original := solidPNG(t, 120, 120)
	avatarID := "avatar-1"
	userID := "user-1"
	key := repository.OriginalKey(userID, avatarID)
	_ = storage.Upload(context.Background(), key, "image/png", bytes.NewReader(original), int64(len(original)))

	repo := &memRepo{avatar: &domain.Avatar{
		ID:               avatarID,
		UserID:           userID,
		S3Key:            key,
		ProcessingStatus: domain.ProcessingStatusProcessing,
		ThumbnailS3Keys:  domain.ThumbnailKeys{},
	}}

	w := worker.New(repo, storage, nil)
	body, _ := json.Marshal(events.AvatarUploadEvent{
		AvatarID: avatarID,
		UserID:   userID,
		S3Key:    key,
	})

	if err := w.HandleUpload(context.Background(), body, "msg-1"); err != nil {
		t.Fatalf("HandleUpload: %v", err)
	}
	if repo.avatar.ProcessingStatus != domain.ProcessingStatusCompleted {
		t.Fatalf("status = %s", repo.avatar.ProcessingStatus)
	}
	if len(repo.avatar.ThumbnailS3Keys) != 2 {
		t.Fatalf("thumbs = %#v", repo.avatar.ThumbnailS3Keys)
	}

	// Idempotent second run.
	if err := w.HandleUpload(context.Background(), body, "msg-2"); err != nil {
		t.Fatalf("second HandleUpload: %v", err)
	}
}

func TestWorker_HandleDelete(t *testing.T) {
	storage := newMemStorage()
	_ = storage.Upload(context.Background(), "a", "image/jpeg", bytes.NewReader([]byte("1")), 1)
	_ = storage.Upload(context.Background(), "b", "image/jpeg", bytes.NewReader([]byte("2")), 1)

	w := worker.New(&memRepo{}, storage, nil)
	body, _ := json.Marshal(events.AvatarDeleteEvent{
		AvatarID: "id",
		S3Keys:   []string{"a", "b"},
	})
	if err := w.HandleDelete(context.Background(), body, "msg"); err != nil {
		t.Fatalf("HandleDelete: %v", err)
	}
	if len(storage.data) != 0 {
		t.Fatalf("expected empty storage, got %#v", storage.data)
	}
}

func solidPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
