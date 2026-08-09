package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/qutaq/GophProfile/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	log.Printf("%s worker starting (env=%s)", cfg.App.Name, cfg.App.Env)
	log.Printf("deps: db=%s:%d/%s s3=%s/%s rabbitmq=%s exchange=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.Name,
		cfg.S3.Endpoint, cfg.S3.Bucket,
		cfg.RabbitMQ.URL, cfg.RabbitMQ.Exchange,
	)

	// Stage 0 stub: wait for shutdown signal.
	// RabbitMQ consumer and image processing will be added in later stages.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("worker stopped")
}
