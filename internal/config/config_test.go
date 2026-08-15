package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	unsetenv(t,
		"APP_ENV",
		"HTTP_ADDR",
		"HTTP_READ_TIMEOUT",
		"HTTP_WRITE_TIMEOUT",
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"DB_SSLMODE",
		"S3_ENDPOINT",
		"S3_ACCESS_KEY",
		"S3_SECRET_KEY",
		"S3_BUCKET",
		"S3_USE_SSL",
		"S3_REGION",
		"RABBITMQ_URL",
		"RABBITMQ_EXCHANGE",
		"UPLOAD_MAX_SIZE_BYTES",
	)

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

func TestLoadBoolAndDuration(t *testing.T) {
	t.Setenv("S3_USE_SSL", "true")
	t.Setenv("HTTP_READ_TIMEOUT", "30s")
	unsetenv(t, "HTTP_WRITE_TIMEOUT", "DB_PORT", "UPLOAD_MAX_SIZE_BYTES")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.S3.UseSSL {
		t.Fatal("S3.UseSSL want true")
	}
	if cfg.HTTP.ReadTimeout != 30*time.Second {
		t.Fatalf("ReadTimeout = %v", cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.WriteTimeout != 15*time.Second {
		t.Fatalf("WriteTimeout = %v, want default 15s", cfg.HTTP.WriteTimeout)
	}
}

func TestLoadInvalidValuesFailEarly(t *testing.T) {
	cases := []struct {
		key   string
		value string
		want  string
	}{
		{"HTTP_WRITE_TIMEOUT", "not-a-duration", "HTTP_WRITE_TIMEOUT"},
		{"DB_PORT", "not-int", "DB_PORT"},
		{"UPLOAD_MAX_SIZE_BYTES", "not-int64", "UPLOAD_MAX_SIZE_BYTES"},
		{"S3_USE_SSL", "not-bool", "S3_USE_SSL"},
	}

	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			// Ensure unrelated parseable fields use defaults.
			unsetenv(t,
				"HTTP_READ_TIMEOUT",
				"HTTP_WRITE_TIMEOUT",
				"DB_PORT",
				"UPLOAD_MAX_SIZE_BYTES",
				"S3_USE_SSL",
			)
			t.Setenv(tc.key, tc.value)

			_, err := Load()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestValidateErrors(t *testing.T) {
	cfg := &Config{
		HTTP:     HTTPConfig{Addr: ":8080"},
		DB:       DBConfig{Host: "h", Name: "n", User: "u"},
		S3:       S3Config{Endpoint: "e", Bucket: "b"},
		RabbitMQ: RabbitMQConfig{URL: "amqp://x", Exchange: "ex"},
		Upload:   UploadConfig{MaxSizeBytes: 1},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("valid config: %v", err)
	}

	cases := []struct {
		name string
		mut  func(*Config)
	}{
		{"http", func(c *Config) { c.HTTP.Addr = "" }},
		{"db", func(c *Config) { c.DB.Host = "" }},
		{"s3", func(c *Config) { c.S3.Bucket = "" }},
		{"rabbit", func(c *Config) { c.RabbitMQ.URL = "" }},
		{"upload", func(c *Config) { c.Upload.MaxSizeBytes = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cp := *cfg
			tc.mut(&cp)
			if err := cp.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func unsetenv(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		prev, had := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unsetenv %s: %v", key, err)
		}
		k, v, ok := key, prev, had
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(k, v)
				return
			}
			_ = os.Unsetenv(k)
		})
	}
}
