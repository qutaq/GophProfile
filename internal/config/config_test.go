package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_SSLMODE", "")
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("S3_ACCESS_KEY", "")
	t.Setenv("S3_SECRET_KEY", "")
	t.Setenv("S3_BUCKET", "")
	t.Setenv("S3_USE_SSL", "")
	t.Setenv("S3_REGION", "")
	t.Setenv("RABBITMQ_URL", "")
	t.Setenv("RABBITMQ_EXCHANGE", "")
	t.Setenv("UPLOAD_MAX_SIZE_BYTES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("HTTP.Addr = %q, want :8080", cfg.HTTP.Addr)
	}
	if cfg.DB.Name != "gophprofile" {
		t.Errorf("DB.Name = %q, want gophprofile", cfg.DB.Name)
	}
	if cfg.S3.Bucket != "avatars" {
		t.Errorf("S3.Bucket = %q, want avatars", cfg.S3.Bucket)
	}
	if cfg.RabbitMQ.Exchange != "avatars.exchange" {
		t.Errorf("RabbitMQ.Exchange = %q, want avatars.exchange", cfg.RabbitMQ.Exchange)
	}
	if cfg.Upload.MaxSizeBytes != 10*1024*1024 {
		t.Errorf("Upload.MaxSizeBytes = %d, want 10485760", cfg.Upload.MaxSizeBytes)
	}

	dsn := cfg.DB.DSN()
	wantDSN := "postgres://gophprofile:gophprofile@localhost:5432/gophprofile?sslmode=disable"
	if dsn != wantDSN {
		t.Errorf("DSN() = %q, want %q", dsn, wantDSN)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "user")
	t.Setenv("DB_PASSWORD", "pass")
	t.Setenv("DB_NAME", "avatars")
	t.Setenv("S3_ENDPOINT", "minio:9000")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("RABBITMQ_URL", "amqp://user:pass@rabbit:5672/")
	t.Setenv("RABBITMQ_EXCHANGE", "custom.exchange")
	t.Setenv("UPLOAD_MAX_SIZE_BYTES", "2048")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.HTTP.Addr != ":9090" {
		t.Errorf("HTTP.Addr = %q, want :9090", cfg.HTTP.Addr)
	}
	if cfg.DB.Port != 5433 {
		t.Errorf("DB.Port = %d, want 5433", cfg.DB.Port)
	}
	if cfg.Upload.MaxSizeBytes != 2048 {
		t.Errorf("Upload.MaxSizeBytes = %d, want 2048", cfg.Upload.MaxSizeBytes)
	}
	if cfg.RabbitMQ.Exchange != "custom.exchange" {
		t.Errorf("RabbitMQ.Exchange = %q, want custom.exchange", cfg.RabbitMQ.Exchange)
	}
}
