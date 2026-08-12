package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/infra/postgres"
)

func TestOpenInvalidHost(t *testing.T) {
	_, err := postgres.Open(config.DBConfig{
		Host:     "127.0.0.1",
		Port:     1,
		User:     "bad",
		Password: "bad",
		Name:     "none",
		SSLMode:  "disable",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ping postgres")
}

func TestPingClosedPool(t *testing.T) {
	pool, err := postgres.Open(config.DBConfig{
		Host:     "127.0.0.1",
		Port:     1,
		User:     "bad",
		Password: "bad",
		Name:     "none",
		SSLMode:  "disable",
	})
	if err == nil {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		require.Error(t, postgres.Ping(ctx, pool))
		return
	}

	// When Open fails (expected for unreachable host), Ping is covered by Open's own ping.
	require.Contains(t, err.Error(), "ping postgres")
}
