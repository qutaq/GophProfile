package services_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/observability"
	"github.com/qutaq/GophProfile/internal/services"
)

func TestAvatarService_UploadAndDeleteSpans(t *testing.T) {
	rec, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	metrics := observability.NewTestMetrics()
	svc := services.NewAvatarService(newMemStore(), newMemStorage(), nil, 1024*1024, nil, metrics)
	ctx := context.Background()
	payload := jpegBytes()

	result, err := svc.Upload(ctx, services.UploadInput{
		UserID:   "user-1",
		FileName: "a.jpg",
		MimeType: "image/jpeg",
		Content:  bytes.NewReader(payload),
		Size:     int64(len(payload)),
	})
	require.NoError(t, err)

	upload := observability.EndedSpan(rec, "avatar.upload")
	require.NotNil(t, upload)
	require.Equal(t, "user-1", observability.SpanAttr(upload, "user.id"))
	require.Equal(t, "a.jpg", observability.SpanAttr(upload, "file.name"))

	_, err = svc.GetByID(ctx, result.Avatar.ID)
	require.NoError(t, err)
	require.NotNil(t, observability.EndedSpan(rec, "avatar.metadata"))

	require.NoError(t, svc.Delete(ctx, result.Avatar.ID, "user-1"))
	require.NotNil(t, observability.EndedSpan(rec, "avatar.delete"))

	got := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(got, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Contains(t, got.Body.String(), `avatars_deletes_total{status="success"}`)
}

func TestAvatarService_GetByIDRecordsError(t *testing.T) {
	rec, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	svc := services.NewAvatarService(newMemStore(), newMemStorage(), nil, 1024, nil, nil)
	_, err := svc.GetByID(context.Background(), "missing")
	require.ErrorIs(t, err, domain.ErrNotFound)

	span := observability.EndedSpan(rec, "avatar.metadata")
	require.NotNil(t, span)
	require.Equal(t, codes.Error, span.Status().Code)
}

func TestAvatarService_UploadRejectsMissingUser(t *testing.T) {
	rec, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	metrics := observability.NewTestMetrics()
	svc := services.NewAvatarService(newMemStore(), newMemStorage(), nil, 1024, nil, metrics)
	_, err := svc.Upload(context.Background(), services.UploadInput{FileName: "a.jpg"})
	require.True(t, errors.Is(err, domain.ErrMissingUserID))

	span := observability.EndedSpan(rec, "avatar.upload")
	require.NotNil(t, span)
	require.Equal(t, codes.Error, span.Status().Code)

	got := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(got, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Contains(t, got.Body.String(), `avatars_uploads_total{status="rejected"}`)
}

func TestAvatarService_UploadLogsTraceUserAndAvatar(t *testing.T) {
	rec, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	var buf bytes.Buffer
	logger := observability.NewLoggerTo(&buf, "info", "gophprofile-server")
	svc := services.NewAvatarService(newMemStore(), newMemStorage(), nil, 1024*1024, logger, nil)

	ctx, span := observability.StartSpan(context.Background(), "http")
	defer span.End()

	payload := jpegBytes()
	result, err := svc.Upload(ctx, services.UploadInput{
		UserID:   "user-1",
		FileName: "a.jpg",
		MimeType: "image/jpeg",
		Content:  bytes.NewReader(payload),
		Size:     int64(len(payload)),
	})
	require.NoError(t, err)

	require.Contains(t, buf.String(), `"msg":"uploading avatar"`)
	require.Contains(t, buf.String(), `"user_id":"user-1"`)
	require.Contains(t, buf.String(), `"avatar_id":"`+result.Avatar.ID)
	require.Contains(t, buf.String(), `"trace_id":"`+span.SpanContext().TraceID().String())
	require.NotNil(t, observability.EndedSpan(rec, "avatar.upload"))
}
