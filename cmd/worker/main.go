package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/qutaq/GophProfile/internal/config"
	"github.com/qutaq/GophProfile/internal/events"
	"github.com/qutaq/GophProfile/internal/infra/postgres"
	"github.com/qutaq/GophProfile/internal/infra/rabbitmq"
	infras3 "github.com/qutaq/GophProfile/internal/infra/s3"
	"github.com/qutaq/GophProfile/internal/repository"
	"github.com/qutaq/GophProfile/internal/worker"
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

	consumer, err := rabbitmq.NewConsumer(rabbitmq.Config{
		URL:      cfg.RabbitMQ.URL,
		Exchange: cfg.RabbitMQ.Exchange,
	})
	if err != nil {
		log.Fatalf("rabbitmq consumer: %v", err)
	}
	defer consumer.Close()

	avatarRepo := repository.NewAvatarRepository(db)
	storage := repository.NewS3Storage(s3Client, cfg.S3.Bucket)
	w := worker.New(avatarRepo, storage)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := consumer.Consume(ctx, events.QueueUpload, w.HandleUpload); err != nil {
		log.Fatalf("consume upload queue: %v", err)
	}
	if err := consumer.Consume(ctx, events.QueueDelete, w.HandleDelete); err != nil {
		log.Fatalf("consume delete queue: %v", err)
	}

	log.Printf("%s worker started (env=%s exchange=%s)", cfg.App.Name, cfg.App.Env, cfg.RabbitMQ.Exchange)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	cancel()
	log.Println("worker stopped")
}
