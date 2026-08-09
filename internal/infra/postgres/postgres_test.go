package postgres_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
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

func TestPingNilDB(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://bad:bad@127.0.0.1:1/none?sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.Error(t, postgres.Ping(ctx, db))
}
