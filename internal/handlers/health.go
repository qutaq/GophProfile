package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	amqp "github.com/rabbitmq/amqp091-go"
)

type HealthDeps struct {
	DB        *sql.DB
	S3        *minio.Client
	S3Bucket  string
	RabbitURL string
}

type HealthHandler struct {
	deps HealthDeps
}

func NewHealthHandler(deps HealthDeps) *HealthHandler {
	return &HealthHandler{deps: deps}
}

type componentStatus struct {
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	Latency string `json:"latency,omitempty"`
}

type healthResponse struct {
	Status     string                     `json:"status"`
	Components map[string]componentStatus `json:"components"`
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	components := map[string]componentStatus{
		"postgres": h.checkPostgres(ctx),
		"minio":    h.checkMinio(ctx),
		"rabbitmq": h.checkRabbitMQ(),
	}

	overall := "ok"
	httpStatus := http.StatusOK
	for _, c := range components {
		if c.Status != "ok" {
			overall = "degraded"
			httpStatus = http.StatusServiceUnavailable
			break
		}
	}

	writeJSON(w, httpStatus, healthResponse{
		Status:     overall,
		Components: components,
	})
}

func (h *HealthHandler) checkPostgres(ctx context.Context) componentStatus {
	if h.deps.DB == nil {
		return componentStatus{Status: "error", Error: "db is not configured"}
	}
	start := time.Now()
	if err := h.deps.DB.PingContext(ctx); err != nil {
		return componentStatus{Status: "error", Error: err.Error()}
	}
	return componentStatus{Status: "ok", Latency: time.Since(start).String()}
}

func (h *HealthHandler) checkMinio(ctx context.Context) componentStatus {
	if h.deps.S3 == nil {
		return componentStatus{Status: "error", Error: "s3 is not configured"}
	}
	start := time.Now()
	exists, err := h.deps.S3.BucketExists(ctx, h.deps.S3Bucket)
	if err != nil {
		return componentStatus{Status: "error", Error: err.Error()}
	}
	if !exists {
		return componentStatus{Status: "error", Error: "bucket not found: " + h.deps.S3Bucket}
	}
	return componentStatus{Status: "ok", Latency: time.Since(start).String()}
}

func (h *HealthHandler) checkRabbitMQ() componentStatus {
	if h.deps.RabbitURL == "" {
		return componentStatus{Status: "error", Error: "rabbitmq url is empty"}
	}
	start := time.Now()
	conn, err := amqp.Dial(h.deps.RabbitURL)
	if err != nil {
		return componentStatus{Status: "error", Error: err.Error()}
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return componentStatus{Status: "error", Error: err.Error()}
	}
	_ = ch.Close()
	return componentStatus{Status: "ok", Latency: time.Since(start).String()}
}
