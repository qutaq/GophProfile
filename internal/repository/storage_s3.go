package repository

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/qutaq/GophProfile/internal/observability"
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

func (s *S3Storage) startS3Span(ctx context.Context, name, operation, key string) (context.Context, trace.Span) {
	return observability.StartSpanKind(ctx, name, trace.SpanKindClient,
		attribute.String("s3.bucket", s.bucket),
		attribute.String("s3.key", key),
		attribute.String("s3.operation", operation),
	)
}

func (s *S3Storage) Upload(ctx context.Context, key, contentType string, body io.Reader, size int64) error {
	ctx, span := s.startS3Span(ctx, "s3.put", "put", key)
	defer span.End()

	opts := minio.PutObjectOptions{}
	if contentType != "" {
		opts.ContentType = contentType
	}

	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, opts)
	if err != nil {
		observability.RecordError(span, err)
		return fmt.Errorf("upload object %q: %w", key, err)
	}
	return nil
}

func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, string, error) {
	ctx, span := s.startS3Span(ctx, "s3.get", "get", key)
	defer span.End()

	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		observability.RecordError(span, err)
		return nil, "", fmt.Errorf("download object %q: %w", key, err)
	}

	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		observability.RecordError(span, err)
		return nil, "", fmt.Errorf("stat object %q: %w", key, err)
	}
	return obj, info.ContentType, nil
}

func (s *S3Storage) Delete(ctx context.Context, keys ...string) error {
	key := ""
	if len(keys) > 0 {
		key = strings.Join(keys, ",")
	}
	ctx, span := s.startS3Span(ctx, "s3.delete", "delete", key)
	defer span.End()
	span.SetAttributes(attribute.Int("s3.key_count", len(keys)))

	for _, k := range keys {
		if k == "" {
			continue
		}
		if err := s.client.RemoveObject(ctx, s.bucket, k, minio.RemoveObjectOptions{}); err != nil {
			observability.RecordError(span, err)
			return fmt.Errorf("delete object %q: %w", k, err)
		}
	}
	return nil
}
