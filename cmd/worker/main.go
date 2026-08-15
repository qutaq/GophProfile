package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/infra/postgres"
	"github.com/qutaq/GophProfile/internal/infra/rabbitmq"
	infras3 "github.com/qutaq/GophProfile/internal/infra/s3"
	"github.com/qutaq/GophProfile/internal/observability"
	"github.com/qutaq/GophProfile/internal/repository"
	"github.com/qutaq/GophProfile/internal/worker"
)

func main() {
	bootstrap := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		bootstrap.Error("load config", "err", err)
		os.Exit(1)
	}
	if os.Getenv("OTEL_SERVICE_NAME") == "" {
		cfg.Observability.ServiceName = "gophprofile-worker"
	}

	logger := observability.NewLogger(cfg.Observability.LogLevel, cfg.Observability.ServiceName)
	slog.SetDefault(logger)

	shutdownTracer, err := observability.InitTracer(context.Background(), cfg.Observability)
	if err != nil {
		logger.Error("init tracer", "err", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracer(ctx); err != nil {
			logger.Error("shutdown tracer", "err", err)
		}
	}()

	db, err := postgres.Open(cfg.DB)
	if err != nil {
		logger.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	observability.RegisterDBPool(db)

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
	if cfg.Observability.MetricsAddr != "" {
		metricsSrv := observability.NewMetricsServer(cfg.Observability.MetricsAddr, cfg.Observability.MetricsPath)
		g.Go(func() error {
			logger.Info("worker metrics listening", "addr", cfg.Observability.MetricsAddr)
			err := metricsSrv.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		})
		g.Go(func() error {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return metricsSrv.Shutdown(shutdownCtx)
		})
	}
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
