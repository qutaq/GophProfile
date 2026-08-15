package services_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/services"
)

type memStore struct {
	mu      sync.Mutex
	avatars map[string]*domain.Avatar
}

func newMemStore() *memStore {
	return &memStore{avatars: make(map[string]*domain.Avatar)}
}

func (m *memStore) Create(_ context.Context, avatar *domain.Avatar) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *avatar
	if cp.ThumbnailS3Keys == nil {
		cp.ThumbnailS3Keys = domain.ThumbnailKeys{}
	}
	m.avatars[avatar.ID] = &cp
	return nil
}

func (m *memStore) GetByID(_ context.Context, id string) (*domain.Avatar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.avatars[id]
	if !ok || a.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *memStore) GetByUserID(_ context.Context, userID string) (*domain.Avatar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var latest *domain.Avatar
	for _, a := range m.avatars {
		if a.UserID != userID || a.DeletedAt != nil {
			continue
		}
		if latest == nil || a.CreatedAt.After(latest.CreatedAt) {
			cp := *a
			latest = &cp
		}
	}
	if latest == nil {
		return nil, domain.ErrNotFound
	}
	return latest, nil
}

func (m *memStore) ListByUserID(_ context.Context, userID string) ([]domain.Avatar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.Avatar, 0)
	for _, a := range m.avatars {
		if a.UserID == userID && a.DeletedAt == nil {
			out = append(out, *a)
		}
	}
	return out, nil
}

func (m *memStore) SoftDelete(_ context.Context, id, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.avatars[id]
	if !ok {
		return domain.ErrNotFound
	}
	if a.UserID != userID {
		return domain.ErrNotFound
	}
	if a.DeletedAt != nil {
		return domain.ErrAlreadyDeleted
	}
	now := a.UpdatedAt
	a.DeletedAt = &now
	return nil
}

func (m *memStore) UpdateUploadStatus(_ context.Context, id string, status domain.UploadStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.avatars[id]
	if !ok || a.DeletedAt != nil {
		return domain.ErrNotFound
	}
	a.UploadStatus = status
	return nil
}

func (m *memStore) UpdateProcessingStatus(_ context.Context, id string, status domain.ProcessingStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.avatars[id]
	if !ok || a.DeletedAt != nil {
		return domain.ErrNotFound
	}
	a.ProcessingStatus = status
	return nil
}

type memStorage struct {
	mu   sync.Mutex
	data map[string][]byte
	ct   map[string]string
}

func newMemStorage() *memStorage {
	return &memStorage{
		data: make(map[string][]byte),
		ct:   make(map[string]string),
	}
}

func (s *memStorage) Upload(_ context.Context, key, contentType string, body io.Reader, _ int64) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = b
	s.ct[key] = contentType
	return nil
}

func (s *memStorage) Download(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data[key]
	if !ok {
		return nil, "", errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(b)), s.ct[key], nil
}

func (s *memStorage) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range keys {
		delete(s.data, k)
		delete(s.ct, k)
	}
	return nil
}

func jpegBytes() []byte {
	// Minimal valid-enough JPEG magic for http.DetectContentType.
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	}
}

func TestAvatarService_UploadGetDelete(t *testing.T) {
	store := newMemStore()
	storage := newMemStorage()
	svc := services.NewAvatarService(store, storage, services.NoopPublisher{}, 1024, nil)

	ctx := context.Background()
	payload := jpegBytes()

	result, err := svc.Upload(ctx, services.UploadInput{
		UserID:   "user-1",
		FileName: "a.jpg",
		MimeType: "image/jpeg",
		Content:  bytes.NewReader(payload),
		Size:     int64(len(payload)),
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if result.Status != "processing" {
		t.Fatalf("status = %q", result.Status)
	}

	img, err := svc.GetImage(ctx, result.Avatar.ID, "original")
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	defer img.Body.Close()
	got, _ := io.ReadAll(img.Body)
	if !bytes.Equal(got, payload) {
		t.Fatalf("image mismatch")
	}

	if err := svc.Delete(ctx, result.Avatar.ID, "other"); err != domain.ErrForbidden {
		t.Fatalf("Delete foreign: %v", err)
	}
	if err := svc.Delete(ctx, result.Avatar.ID, "user-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.GetByID(ctx, result.Avatar.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestAvatarService_RejectInvalidAndLarge(t *testing.T) {
	svc := services.NewAvatarService(newMemStore(), newMemStorage(), nil, 16, nil)
	ctx := context.Background()

	_, err := svc.Upload(ctx, services.UploadInput{
		UserID:   "u",
		FileName: "a.txt",
		MimeType: "text/plain",
		Content:  bytes.NewReader([]byte("hello world!!!!")),
		Size:     15,
	})
	if !errors.Is(err, domain.ErrInvalidFile) {
		t.Fatalf("want ErrInvalidFile, got %v", err)
	}

	_, err = svc.Upload(ctx, services.UploadInput{
		UserID:   "u",
		FileName: "a.jpg",
		MimeType: "image/jpeg",
		Content:  bytes.NewReader(bytes.Repeat([]byte{0xFF}, 32)),
		Size:     32,
	})
	if !errors.Is(err, domain.ErrFileTooLarge) {
		t.Fatalf("want ErrFileTooLarge, got %v", err)
	}
}
