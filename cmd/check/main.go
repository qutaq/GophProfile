package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/infra/postgres"
	"github.com/qutaq/GophProfile/internal/infra/rabbitmq"
	"github.com/qutaq/GophProfile/internal/infra/s3"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	failed := false

	if err := checkPostgres(ctx, cfg); err != nil {
		log.Printf("postgres: FAIL (%v)", err)
		failed = true
	} else {
		log.Printf("postgres: OK (%s:%d/%s)", cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
	}

	if err := checkS3(ctx, cfg); err != nil {
		log.Printf("minio: FAIL (%v)", err)
		failed = true
	} else {
		log.Printf("minio: OK (endpoint=%s bucket=%s)", cfg.S3.Endpoint, cfg.S3.Bucket)
	}

	if err := checkRabbitMQ(cfg); err != nil {
		log.Printf("rabbitmq: FAIL (%v)", err)
		failed = true
	} else {
		log.Printf("rabbitmq: OK (exchange=%s)", cfg.RabbitMQ.Exchange)
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
