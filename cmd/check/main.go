package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/infra/postgres"
	"github.com/qutaq/GophProfile/internal/infra/rabbitmq"
	"github.com/qutaq/GophProfile/internal/infra/s3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	failed := false

	if err := checkPostgres(ctx, cfg); err != nil {
		logger.Error("postgres check failed", "err", err)
		failed = true
	} else {
		logger.Info("postgres check ok",
			"host", cfg.DB.Host,
			"port", cfg.DB.Port,
			"db", cfg.DB.Name,
		)
	}

	if err := checkS3(ctx, cfg); err != nil {
		logger.Error("minio check failed", "err", err)
		failed = true
	} else {
		logger.Info("minio check ok",
			"endpoint", cfg.S3.Endpoint,
			"bucket", cfg.S3.Bucket,
		)
	}

	if err := checkRabbitMQ(cfg); err != nil {
		logger.Error("rabbitmq check failed", "err", err)
		failed = true
	} else {
		logger.Info("rabbitmq check ok", "exchange", cfg.RabbitMQ.Exchange)
	}

	if failed {
		os.Exit(1)
	}
	fmt.Println("all dependencies are ready")
}

func checkPostgres(ctx context.Context, cfg *config.Config) error {
	db, err := postgres.Open(cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()
	return postgres.Ping(ctx, db)
}

func checkS3(ctx context.Context, cfg *config.Config) error {
	client, err := s3.NewClient(cfg.S3)
	if err != nil {
		return err
	}

	buckets, err := s3.ListBuckets(ctx, client)
	if err != nil {
		return err
	}

	found := false
	for _, name := range buckets {
		if name == cfg.S3.Bucket {
			found = true
			break
		}
	}
	if !found {
		if err := s3.EnsureBucket(ctx, client, cfg.S3.Bucket); err != nil {
			return fmt.Errorf("bucket %q not found and create failed: %w", cfg.S3.Bucket, err)
		}
	}
	return nil
}

func checkRabbitMQ(cfg *config.Config) error {
	return rabbitmq.DeclareExchange(cfg.RabbitMQ)
}
