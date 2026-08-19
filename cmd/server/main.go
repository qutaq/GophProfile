package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/qutaq/GophProfile/internal/api"
	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/handlers"
	"github.com/qutaq/GophProfile/internal/infra/postgres"
	"github.com/qutaq/GophProfile/internal/infra/rabbitmq"
	infras3 "github.com/qutaq/GophProfile/internal/infra/s3"
	"github.com/qutaq/GophProfile/internal/observability"
	"github.com/qutaq/GophProfile/internal/repository"
	"github.com/qutaq/GophProfile/internal/services"
)

func main() {
	bootstrap := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		bootstrap.Error("load config", "err", err)
		os.Exit(1)
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

	metrics := observability.NewMetrics(nil)
	metrics.RegisterDBPool(db)

	s3Client, err := infras3.NewClient(cfg.S3)
	if err != nil {
		logger.Error("create s3 client", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := infras3.EnsureBucket(ctx, s3Client, cfg.S3.Bucket); err != nil {
		logger.Error("ensure bucket", "err", err)
		os.Exit(1)
	}

	publisher, err := rabbitmq.NewPublisher(rabbitmq.Config{
		URL:      cfg.RabbitMQ.URL,
		Exchange: cfg.RabbitMQ.Exchange,
	}, metrics)
	if err != nil {
		logger.Error("rabbitmq publisher", "err", err)
		os.Exit(1)
	}
	defer publisher.Close()

	avatarRepo := repository.NewAvatarRepository(db)
	storage := repository.NewS3Storage(s3Client, cfg.S3.Bucket)
	avatarSvc := services.NewAvatarService(avatarRepo, storage, publisher, cfg.Upload.MaxSizeBytes, logger, metrics)

	webHandler, err := handlers.NewWebHandler(avatarSvc, "web")
	if err != nil {
		logger.Error("load web templates", "err", err)
		os.Exit(1)
	}

	healthHandler := handlers.NewHealthHandler(handlers.HealthDeps{
		DB:        db,
		S3:        s3Client,
		S3Bucket:  cfg.S3.Bucket,
		RabbitURL: cfg.RabbitMQ.URL,
	})
	defer healthHandler.Close()

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go metrics.CollectStorageBytes(runCtx, avatarRepo.SumActiveSizeBytes, 15*time.Second, logger)

	router := api.NewRouter(api.Handlers{
		Avatars:     handlers.NewAvatarHandler(avatarSvc),
		Health:      healthHandler,
		Web:         webHandler,
		WebDir:      "web",
		MetricsPath: cfg.Observability.MetricsPath,
		Logger:      logger,
		Metrics:     metrics,
	})

	server := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	go func() {
		logger.Info("server starting",
			"app", cfg.App.Name,
			"addr", cfg.HTTP.Addr,
			"env", cfg.App.Env,
		)
		logger.Info("web UI available", "url", "http://localhost"+cfg.HTTP.Addr+"/web/upload")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-runCtx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
	logger.Info("server stopped")
}
