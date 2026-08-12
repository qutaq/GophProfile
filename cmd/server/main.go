package main

import (
	"context"
	"log"
	"net/http"
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
		log.Fatalf("load config: %v", err)
	}

	db, err := postgres.Open(cfg.DB)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	s3Client, err := infras3.NewClient(cfg.S3)
	if err != nil {
		log.Fatalf("create s3 client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := infras3.EnsureBucket(ctx, s3Client, cfg.S3.Bucket); err != nil {
		log.Fatalf("ensure bucket: %v", err)
	}

	publisher, err := rabbitmq.NewPublisher(rabbitmq.Config{
		URL:      cfg.RabbitMQ.URL,
		Exchange: cfg.RabbitMQ.Exchange,
	})
	if err != nil {
		log.Fatalf("rabbitmq publisher: %v", err)
	}
	defer publisher.Close()

	avatarRepo := repository.NewAvatarRepository(db)
	storage := repository.NewS3Storage(s3Client, cfg.S3.Bucket)
	avatarSvc := services.NewAvatarService(avatarRepo, storage, publisher, cfg.Upload.MaxSizeBytes)

	webHandler, err := handlers.NewWebHandler(avatarSvc, "web")
	if err != nil {
		log.Fatalf("load web templates: %v", err)
	}

	router := api.NewRouter(api.Handlers{
		Avatars: handlers.NewAvatarHandler(avatarSvc),
		Health: handlers.NewHealthHandler(handlers.HealthDeps{
			DB:        db,
			S3:        s3Client,
			S3Bucket:  cfg.S3.Bucket,
			RabbitURL: cfg.RabbitMQ.URL,
		}),
		Web:    webHandler,
		WebDir: "web",
	})

	server := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	go func() {
		log.Printf("%s server starting on %s (env=%s)", cfg.App.Name, cfg.HTTP.Addr, cfg.App.Env)
		log.Printf("web UI: http://localhost%s/web/upload", cfg.HTTP.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	runCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-runCtx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("server stopped")
}
