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
	"github.com/qutaq/GophProfile/internal/repository"
	"github.com/qutaq/GophProfile/internal/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	db, err := postgres.Open(cfg.DB)
	if err != nil {
		slog.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	s3Client, err := infras3.NewClient(cfg.S3)
	if err != nil {
		slog.Error("create s3 client", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := infras3.EnsureBucket(ctx, s3Client, cfg.S3.Bucket); err != nil {
		slog.Error("ensure bucket", "err", err)
		os.Exit(1)
	}

	publisher, err := rabbitmq.NewPublisher(rabbitmq.Config{
		URL:      cfg.RabbitMQ.URL,
		Exchange: cfg.RabbitMQ.Exchange,
	})
	if err != nil {
		slog.Error("rabbitmq publisher", "err", err)
		os.Exit(1)
	}
	defer publisher.Close()

	avatarRepo := repository.NewAvatarRepository(db)
	storage := repository.NewS3Storage(s3Client, cfg.S3.Bucket)
	avatarSvc := services.NewAvatarService(avatarRepo, storage, publisher, cfg.Upload.MaxSizeBytes)

	webHandler, err := handlers.NewWebHandler(avatarSvc, "web")
	if err != nil {
		slog.Error("load web templates", "err", err)
		os.Exit(1)
	}

	healthHandler := handlers.NewHealthHandler(handlers.HealthDeps{
		DB:        db,
		S3:        s3Client,
		S3Bucket:  cfg.S3.Bucket,
		RabbitURL: cfg.RabbitMQ.URL,
	})
	defer healthHandler.Close()

	router := api.NewRouter(api.Handlers{
		Avatars: handlers.NewAvatarHandler(avatarSvc),
		Health:  healthHandler,
		Web:     webHandler,
		WebDir:  "web",
	})

	server := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	go func() {
		slog.Info("server starting",
			"app", cfg.App.Name,
			"addr", cfg.HTTP.Addr,
			"env", cfg.App.Env,
		)
		slog.Info("web UI available", "url", "http://localhost"+cfg.HTTP.Addr+"/web/upload")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-runCtx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
	slog.Info("server stopped")
}
