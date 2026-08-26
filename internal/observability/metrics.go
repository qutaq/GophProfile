package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
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

// Metrics holds Prometheus collectors registered on a caller-supplied Registerer.
// Methods are no-ops on a nil receiver so tests can omit metrics.
type Metrics struct {
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer

	httpRequestsTotal          *prometheus.CounterVec
	httpRequestDuration        *prometheus.HistogramVec
	httpRequestsInFlight       prometheus.Gauge
	avatarsUploadsTotal        *prometheus.CounterVec
	avatarsUploadDuration      *prometheus.HistogramVec
	avatarsDeletesTotal        *prometheus.CounterVec
	avatarsProcessingTotal     *prometheus.CounterVec
	avatarsProcessingDuration  prometheus.Histogram
	avatarsStorageBytes        prometheus.Gauge
	avatarsThumbnailsGenerated *prometheus.CounterVec
	rabbitmqPublishedTotal     *prometheus.CounterVec
	rabbitmqConsumedTotal      *prometheus.CounterVec
	rabbitmqConsumerInFlight   prometheus.Gauge
}

// NewRegistry returns a registry with Go and process collectors.
func NewRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}

// NewMetrics registers application metrics on reg.
// If reg is nil, a new registry with runtime collectors is created.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = NewRegistry()
	}
	factory := promauto.With(reg)
	m := &Metrics{registerer: reg}
	if g, ok := reg.(prometheus.Gatherer); ok {
		m.gatherer = g
	} else {
		m.gatherer = prometheus.DefaultGatherer
	}

	m.httpRequestsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "route", "status"},
	)
	m.httpRequestDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
	m.httpRequestsInFlight = factory.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being served",
		},
	)
	m.avatarsUploadsTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_uploads_total",
			Help: "Total number of avatar uploads",
		},
		[]string{"status"},
	)
	m.avatarsUploadDuration = factory.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "avatars_upload_duration_seconds",
			Help:    "Avatar upload duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"status"},
	)
	m.avatarsDeletesTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_deletes_total",
			Help: "Total number of avatar deletes",
		},
		[]string{"status"},
	)
	m.avatarsProcessingTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_processing_total",
			Help: "Total number of avatar processing attempts",
		},
		[]string{"result"},
	)
	m.avatarsProcessingDuration = factory.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "avatars_processing_duration_seconds",
			Help:    "Avatar processing duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
	m.avatarsStorageBytes = factory.NewGauge(
		prometheus.GaugeOpts{
			Name: "avatars_storage_bytes",
			Help: "Total storage used by active avatars in bytes",
		},
	)
	m.avatarsThumbnailsGenerated = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "avatars_thumbnails_generated_total",
			Help: "Total number of generated avatar thumbnails",
		},
		[]string{"size"},
	)
	m.rabbitmqPublishedTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_messages_published_total",
			Help: "Total number of published RabbitMQ messages",
		},
		[]string{"routing_key"},
	)
	m.rabbitmqConsumedTotal = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rabbitmq_messages_consumed_total",
			Help: "Total number of consumed RabbitMQ messages",
		},
		[]string{"queue", "result"},
	)
	m.rabbitmqConsumerInFlight = factory.NewGauge(
		prometheus.GaugeOpts{
			Name: "rabbitmq_consumer_in_flight",
			Help: "Number of RabbitMQ deliveries currently being handled",
		},
	)
	return m
}

// NewTestMetrics returns metrics on a fresh registry without runtime collectors.
func NewTestMetrics() *Metrics {
	return NewMetrics(prometheus.NewRegistry())
}

func (m *Metrics) Handler() http.Handler {
	gatherer := prometheus.Gatherer(prometheus.NewRegistry())
	if m != nil && m.gatherer != nil {
		gatherer = m.gatherer
	}
	return promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})
}

func NewMetricsServer(addr, path string, m *Metrics, ready func() error) *http.Server {
	if path == "" {
		path = "/metrics"
	}
	mux := http.NewServeMux()
	mux.Handle(path, m.Handler())
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if ready != nil {
			if err := ready(); err != nil {
				http.Error(w, err.Error(), http.StatusServiceUnavailable)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
	})
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func (m *Metrics) HTTPInFlightInc() {
	if m == nil {
		return
	}
	m.httpRequestsInFlight.Inc()
}

func (m *Metrics) HTTPInFlightDec() {
	if m == nil {
		return
	}
	m.httpRequestsInFlight.Dec()
}

func (m *Metrics) ObserveHTTP(method, route string, status int, seconds float64) {
	if m == nil {
		return
	}
	m.httpRequestsTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.httpRequestDuration.WithLabelValues(method, route).Observe(seconds)
}

func (m *Metrics) ObserveUpload(status string, d time.Duration) {
	if m == nil {
		return
	}
	m.avatarsUploadsTotal.WithLabelValues(status).Inc()
	m.avatarsUploadDuration.WithLabelValues(status).Observe(d.Seconds())
}

func (m *Metrics) ObserveDelete(status string) {
	if m == nil {
		return
	}
	m.avatarsDeletesTotal.WithLabelValues(status).Inc()
}

func (m *Metrics) ObserveProcessing(result string, d time.Duration) {
	if m == nil {
		return
	}
	m.avatarsProcessingTotal.WithLabelValues(result).Inc()
	m.avatarsProcessingDuration.Observe(d.Seconds())
}

func (m *Metrics) ObserveThumbnail(size string) {
	if m == nil {
		return
	}
	m.avatarsThumbnailsGenerated.WithLabelValues(size).Inc()
}

func (m *Metrics) ObservePublish(routingKey string) {
	if m == nil {
		return
	}
	m.rabbitmqPublishedTotal.WithLabelValues(routingKey).Inc()
}

func (m *Metrics) ObserveConsume(queue, result string) {
	if m == nil {
		return
	}
	m.rabbitmqConsumedTotal.WithLabelValues(queue, result).Inc()
}

func (m *Metrics) ConsumerInFlightInc() {
	if m == nil {
		return
	}
	m.rabbitmqConsumerInFlight.Inc()
}

func (m *Metrics) ConsumerInFlightDec() {
	if m == nil {
		return
	}
	m.rabbitmqConsumerInFlight.Dec()
}
