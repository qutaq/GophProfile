package worker_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"

	"github.com/qutaq/GophProfile/internal/domain"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/observability"
	"github.com/qutaq/GophProfile/internal/repository"
	"github.com/qutaq/GophProfile/internal/worker"
)

func TestWorker_HandleUploadSpans(t *testing.T) {
	rec, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	storage := newMemStorage()
	original := solidPNG(t, 120, 120)
	avatarID := "avatar-trace"
	userID := "user-trace"
	key := repository.OriginalKey(userID, avatarID)
	require.NoError(t, storage.Upload(context.Background(), key, "image/png", bytes.NewReader(original), int64(len(original))))

	repo := &memRepo{avatar: &domain.Avatar{
		ID:               avatarID,
		UserID:           userID,
		S3Key:            key,
		ProcessingStatus: domain.ProcessingStatusProcessing,
		ThumbnailS3Keys:  domain.ThumbnailKeys{},
	}}
	metrics := observability.NewTestMetrics()
	w := worker.New(repo, storage, nil, metrics)
	body, err := json.Marshal(events.AvatarUploadEvent{
		AvatarID: avatarID,
		UserID:   userID,
		S3Key:    key,
	})
	require.NoError(t, err)
	require.NoError(t, w.HandleUpload(context.Background(), body, "msg-1"))

	upload := observability.EndedSpan(rec, "worker.handle_upload")
	require.NotNil(t, upload)
	require.Equal(t, avatarID, observability.SpanAttr(upload, "avatar.id"))
	require.Equal(t, 2, observability.CountSpans(rec, "worker.resize"))

	recMetrics := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recMetrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	got := recMetrics.Body.String()
	require.Contains(t, got, `avatars_processing_total{result="completed"}`)
	require.Contains(t, got, `avatars_thumbnails_generated_total{size="100x100"}`)
	require.Contains(t, got, `avatars_thumbnails_generated_total{size="300x300"}`)
}

func TestWorker_HandleUploadBadJSONRecordsError(t *testing.T) {
	rec, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	w := worker.New(&memRepo{}, newMemStorage(), nil, nil)
	err := w.HandleUpload(context.Background(), []byte(`{`), "m1")
	require.Error(t, err)

	span := observability.EndedSpan(rec, "worker.handle_upload")
	require.NotNil(t, span)
	require.Equal(t, codes.Error, span.Status().Code)
}

func TestWorker_HandleDeleteSpan(t *testing.T) {
	rec, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	w := worker.New(&memRepo{}, newMemStorage(), nil, nil)
	body, err := json.Marshal(events.AvatarDeleteEvent{AvatarID: "id", S3Keys: nil})
	require.NoError(t, err)
	require.NoError(t, w.HandleDelete(context.Background(), body, "msg"))

	span := observability.EndedSpan(rec, "worker.handle_delete")
	require.NotNil(t, span)
	require.Equal(t, "id", observability.SpanAttr(span, "avatar.id"))
}

func TestWorker_HandleUploadLogsTrace(t *testing.T) {
	_, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	storage := newMemStorage()
	original := solidPNG(t, 120, 120)
	avatarID := "avatar-log"
	userID := "user-log"
	key := repository.OriginalKey(userID, avatarID)
	require.NoError(t, storage.Upload(context.Background(), key, "image/png", bytes.NewReader(original), int64(len(original))))

	var buf bytes.Buffer
	logger := observability.NewLoggerTo(&buf, "info", "gophprofile-worker")
	w := worker.New(&memRepo{avatar: &domain.Avatar{
		ID:               avatarID,
		UserID:           userID,
		S3Key:            key,
		ProcessingStatus: domain.ProcessingStatusProcessing,
		ThumbnailS3Keys:  domain.ThumbnailKeys{},
	}}, storage, logger, nil)

	ctx, span := observability.StartSpan(context.Background(), "rabbitmq.consume")
	defer span.End()

	body, err := json.Marshal(events.AvatarUploadEvent{
		AvatarID: avatarID,
		UserID:   userID,
		S3Key:    key,
	})
	require.NoError(t, err)
	require.NoError(t, w.HandleUpload(ctx, body, "msg-log"))

	got := buf.String()
	require.Contains(t, got, `"msg":"handle upload"`)
	require.Contains(t, got, `"avatar_id":"avatar-log"`)
	require.Contains(t, got, `"user_id":"user-log"`)
	require.Contains(t, got, `"trace_id":"`+span.SpanContext().TraceID().String())
}
