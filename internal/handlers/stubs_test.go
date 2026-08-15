package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/qutaq/GophProfile/internal/domain"
)

type memAvatarStore struct {
	mu      sync.Mutex
	avatars map[string]*domain.Avatar
}

func newMemAvatarStore() *memAvatarStore {
	return &memAvatarStore{avatars: make(map[string]*domain.Avatar)}
}

func (m *memAvatarStore) Create(_ context.Context, avatar *domain.Avatar) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *avatar
	if cp.ThumbnailS3Keys == nil {
		cp.ThumbnailS3Keys = domain.ThumbnailKeys{}
	}
	m.avatars[avatar.ID] = &cp
	return nil
}

func (m *memAvatarStore) GetByID(_ context.Context, id string) (*domain.Avatar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.avatars[id]
	if !ok || a.DeletedAt != nil {
		return nil, domain.ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *memAvatarStore) GetByUserID(_ context.Context, userID string) (*domain.Avatar, error) {
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

func (m *memAvatarStore) ListByUserID(_ context.Context, userID string) ([]domain.Avatar, error) {
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

func (m *memAvatarStore) SoftDelete(_ context.Context, id, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.avatars[id]
	if !ok || a.UserID != userID {
		return domain.ErrNotFound
	}
	if a.DeletedAt != nil {
		return domain.ErrAlreadyDeleted
	}
	now := time.Now().UTC()
	a.DeletedAt = &now
	return nil
}

func (m *memAvatarStore) UpdateUploadStatus(_ context.Context, id string, status domain.UploadStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.avatars[id]
	if !ok || a.DeletedAt != nil {
		return domain.ErrNotFound
	}
	a.UploadStatus = status
	return nil
}

func (m *memAvatarStore) UpdateProcessingStatus(_ context.Context, id string, status domain.ProcessingStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.avatars[id]
	if !ok || a.DeletedAt != nil {
		return domain.ErrNotFound
	}
	a.ProcessingStatus = status
	return nil
}

type memObjectStorage struct {
	mu   sync.Mutex
	data map[string][]byte
	ct   map[string]string
}

func newMemObjectStorage() *memObjectStorage {
	return &memObjectStorage{
		data: make(map[string][]byte),
		ct:   make(map[string]string),
	}
}

func (s *memObjectStorage) Upload(_ context.Context, key, contentType string, body io.Reader, _ int64) error {
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

func (s *memObjectStorage) Download(_ context.Context, key string) (io.ReadCloser, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data[key]
	if !ok {
		return nil, "", errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(b)), s.ct[key], nil
}

func (s *memObjectStorage) Delete(_ context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range keys {
		delete(s.data, k)
		delete(s.ct, k)
	}
	return nil
}
