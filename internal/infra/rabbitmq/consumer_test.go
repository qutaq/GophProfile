package rabbitmq

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/observability"
)

func TestConsumerReadyNil(t *testing.T) {
	var c *Consumer
	if err := c.Ready(); err == nil {
		t.Fatal("expected error for nil consumer")
	}
	c = &Consumer{}
	if err := c.Ready(); err == nil {
		t.Fatal("expected error for uninitialized connection")
	}
}

func TestRetryWithBackoffSuccessAfterFailures(t *testing.T) {
	attempts := 0
	err := retryWithBackoff(context.Background(), nil, 4, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRetryWithBackoffContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := retryWithBackoff(ctx, nil, 5, func() error {
		return errors.New("always fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRetryWithBackoffExhausted(t *testing.T) {
	err := retryWithBackoff(context.Background(), nil, 2, func() error {
		return errors.New("nope")
	})
	if err == nil || err.Error() != "nope" {
		t.Fatalf("got %v", err)
	}
}

func TestRetryWithBackoffLogsTrace(t *testing.T) {
	_, cleanup := observability.NewTestTracer()
	t.Cleanup(cleanup)

	var buf bytes.Buffer
	logger := observability.NewLoggerTo(&buf, "info", "gophprofile-worker")
	ctx, span := observability.StartSpan(context.Background(), "rabbitmq.consume")
	defer span.End()

	attempts := 0
	err := retryWithBackoff(ctx, logger, 2, func() error {
		attempts++
		if attempts == 1 {
			return errors.New("transient")
		}
		return nil
	})
	require.NoError(t, err)
	require.Contains(t, buf.String(), `"msg":"retrying message handler"`)
	require.Contains(t, buf.String(), `"trace_id":"`+span.SpanContext().TraceID().String())
}
