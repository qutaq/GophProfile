package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/infra/postgres"
	"github.com/qutaq/GophProfile/internal/infra/rabbitmq"
	infras3 "github.com/qutaq/GophProfile/internal/infra/s3"
	"github.com/qutaq/GophProfile/internal/repository"
	"github.com/qutaq/GophProfile/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	db, err := postgres.Open(cfg.DB)
	if err != nil {
		logger.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	s3Client, err := infras3.NewClient(cfg.S3)
	if err != nil {
		logger.Error("create s3 client", "err", err)
		os.Exit(1)
	}

	consumer, err := rabbitmq.NewConsumer(rabbitmq.Config{
		URL:      cfg.RabbitMQ.URL,
		Exchange: cfg.RabbitMQ.Exchange,
	}, logger)
	if err != nil {
		logger.Error("rabbitmq consumer", "err", err)
		os.Exit(1)
	}
	defer consumer.Close()

	avatarRepo := repository.NewAvatarRepository(db)
	storage := repository.NewS3Storage(s3Client, cfg.S3.Bucket)
	w := worker.New(avatarRepo, storage, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return consumer.Consume(ctx, events.QueueUpload, w.HandleUpload)
	})
	g.Go(func() error {
		return consumer.Consume(ctx, events.QueueDelete, w.HandleDelete)
	})

	logger.Info("worker started",
		"app", cfg.App.Name,
		"env", cfg.App.Env,
		"exchange", cfg.RabbitMQ.Exchange,
	)

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("worker failed", "err", err)
		os.Exit(1)
	}
	logger.Info("worker stopped")
}
