package rabbitmq

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryWithBackoffSuccessAfterFailures(t *testing.T) {
	attempts := 0
	err := retryWithBackoff(context.Background(), 4, func() error {
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

	err := retryWithBackoff(ctx, 5, func() error {
		return errors.New("always fail")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRetryWithBackoffExhausted(t *testing.T) {
	err := retryWithBackoff(context.Background(), 2, func() error {
		return errors.New("nope")
	})
	if err == nil || err.Error() != "nope" {
		t.Fatalf("got %v", err)
	}
}
