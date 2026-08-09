package repository

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

// ObjectStorage stores and retrieves avatar binary objects.
type ObjectStorage interface {
	Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Download(ctx context.Context, key string) (io.ReadCloser, string, error)
	Delete(ctx context.Context, keys ...string) error
}

type S3Storage struct {
	client *minio.Client
	bucket string
}

func NewS3Storage(client *minio.Client, bucket string) *S3Storage {
	return &S3Storage{client: client, bucket: bucket}
}

func (s *S3Storage) Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	opts := minio.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}

	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, opts)
	if err != nil {
		return fmt.Errorf("upload object %q: %w", key, err)
	}
	return nil
}

func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", fmt.Errorf("download object %q: %w", key, err)
	}

	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, "", fmt.Errorf("stat object %q: %w", key, err)
	}
	return obj, info.ContentType, nil
}

func (s *S3Storage) Delete(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("delete object %q: %w", key, err)
		}
	}
	return nil
}
