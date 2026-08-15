package observability

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	StatusSuccess  = "success"
	StatusError    = "error"
	StatusRejected = "rejected"

	ResultCompleted = "completed"
	ResultFailed    = "failed"
	ResultSkipped   = "skipped"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "route", "status"},
	)
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being served",
		},
	)

	avatarsUploadsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_uploads_total",
			Help: "Total number of avatar uploads",
		},
		[]string{"status"},
	)
	avatarsUploadDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "avatars_upload_duration_seconds",
			Help:    "Avatar upload duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)
	avatarsDeletesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_deletes_total",
			Help: "Total number of avatar deletes",
		},
		[]string{"status"},
	)
	avatarsProcessingTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_processing_total",
			Help: "Total number of avatar processing attempts",
		},
		[]string{"result"},
	)
	avatarsProcessingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "avatars_processing_duration_seconds",
			Help:    "Avatar processing duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
	avatarsStorageBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "avatars_storage_bytes",
			Help: "Total storage used by active avatars in bytes",
		},
	)
	avatarsThumbnailsGenerated = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_thumbnails_generated_total",
			Help: "Total number of generated avatar thumbnails",
		},
		[]string{"size"},
	)

	rabbitmqPublishedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_messages_published_total",
			Help: "Total number of published RabbitMQ messages",
		},
		[]string{"routing_key"},
	)
	rabbitmqConsumedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_messages_consumed_total",
			Help: "Total number of consumed RabbitMQ messages",
		},
		[]string{"queue", "result"},
	)
	rabbitmqConsumerInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "rabbitmq_consumer_in_flight",
			Help: "Number of RabbitMQ deliveries currently being handled",
		},
	)
)

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

func NewMetricsServer(addr, path string) *http.Server {
	if path == "" {
		path = "/metrics"
	}
	mux := http.NewServeMux()
	mux.Handle(path, MetricsHandler())
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func ObserveUpload(status string, d time.Duration) {
	avatarsUploadsTotal.WithLabelValues(status).Inc()
	avatarsUploadDuration.WithLabelValues(status).Observe(d.Seconds())
}

func ObserveDelete(status string) {
	avatarsDeletesTotal.WithLabelValues(status).Inc()
}

func ObserveProcessing(result string, d time.Duration) {
	avatarsProcessingTotal.WithLabelValues(result).Inc()
	avatarsProcessingDuration.Observe(d.Seconds())
}

func ObserveThumbnail(size string) {
	avatarsThumbnailsGenerated.WithLabelValues(size).Inc()
}

func ObservePublish(routingKey string) {
	rabbitmqPublishedTotal.WithLabelValues(routingKey).Inc()
}

func ObserveConsume(queue, result string) {
	rabbitmqConsumedTotal.WithLabelValues(queue, result).Inc()
}

func ConsumerInFlightInc() { rabbitmqConsumerInFlight.Inc() }
func ConsumerInFlightDec() { rabbitmqConsumerInFlight.Dec() }
