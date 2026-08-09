package repository_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/infra/migrate"
	"github.com/qutaq/GophProfile/internal/infra/postgres"
	infras3 "github.com/qutaq/GophProfile/internal/infra/s3"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	db, err := postgres.Open(cfg.DB)
	if err != nil {
		t.Skipf("postgres unavailable, skip integration test: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		t.Skipf("postgres ping failed, skip integration test: %v", err)
	}

	if err := migrate.Up(cfg.DB.DSN()); err != nil {
		_ = db.Close()
		t.Fatalf("migrate up: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })
	return db
}

func openTestS3(t *testing.T) (*minio.Client, string) {
	t.Helper()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	client, err := infras3.NewClient(cfg.S3)
	if err != nil {
		t.Skipf("minio client create failed, skip: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.ListBuckets(ctx); err != nil {
		t.Skipf("minio unavailable, skip integration test: %v", err)
	}

	if err := infras3.EnsureBucket(ctx, client, cfg.S3.Bucket); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	return client, cfg.S3.Bucket
}

func cleanupUserAvatars(t *testing.T, db *sql.DB, userID string) {
	t.Helper()
	_, err := db.Exec(`DELETE FROM avatars WHERE user_id = $1`, userID)
	if err != nil {
		t.Fatalf("cleanup avatars: %v", err)
	}
}

func integrationEnabled() bool {
	// Always try; helpers skip when infra is down.
	// Set SKIP_INTEGRATION=1 to force-skip.
	return os.Getenv("SKIP_INTEGRATION") == ""
}
