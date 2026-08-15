package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	App           AppConfig
	HTTP          HTTPConfig
	DB            DBConfig
	S3            S3Config
	RabbitMQ      RabbitMQConfig
	Upload        UploadConfig
	Observability ObservabilityConfig
}

type AppConfig struct {
	Env  string
	Name string
}

type HTTPConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

type S3Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	UseSSL          bool
	Region          string
}

type RabbitMQConfig struct {
	URL      string
	Exchange string
}

type UploadConfig struct {
	MaxSizeBytes int64
}

type ObservabilityConfig struct {
	Enabled      bool
	ServiceName  string
	OTLPEndpoint string
	Insecure     bool
	LogLevel     string
	MetricsPath  string
	MetricsAddr  string
}

// Load reads configuration from environment variables and applies defaults.
func Load() (*Config, error) {
	readTimeout, err := getEnvDuration("HTTP_READ_TIMEOUT", 15*time.Second)
	if err != nil {
		return nil, err
	}
	writeTimeout, err := getEnvDuration("HTTP_WRITE_TIMEOUT", 15*time.Second)
	if err != nil {
		return nil, err
	}
	dbPort, err := getEnvInt("DB_PORT", 5432)
	if err != nil {
		return nil, err
	}
	useSSL, err := getEnvBool("S3_USE_SSL", false)
	if err != nil {
		return nil, err
	}
	maxSizeBytes, err := getEnvInt64("UPLOAD_MAX_SIZE_BYTES", 10*1024*1024)
	if err != nil {
		return nil, err
	}
	otelEnabled, err := getEnvBool("OTEL_ENABLED", true)
	if err != nil {
		return nil, err
	}
	otelInsecure, err := getEnvBool("OTEL_EXPORTER_OTLP_INSECURE", true)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		App: AppConfig{
			Env:  getEnv("APP_ENV", "development"),
			Name: getEnv("APP_NAME", "gophprofile"),
		},
		HTTP: HTTPConfig{
			Addr:         getEnv("HTTP_ADDR", ":8080"),
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "gophprofile"),
			Password: getEnv("DB_PASSWORD", "gophprofile"),
			Name:     getEnv("DB_NAME", "gophprofile"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		S3: S3Config{
			Endpoint:        getEnv("S3_ENDPOINT", "localhost:9000"),
			AccessKeyID:     getEnv("S3_ACCESS_KEY", "minioadmin"),
			SecretAccessKey: getEnv("S3_SECRET_KEY", "minioadmin"),
			Bucket:          getEnv("S3_BUCKET", "avatars"),
			UseSSL:          useSSL,
			Region:          getEnv("S3_REGION", "us-east-1"),
		},
		RabbitMQ: RabbitMQConfig{
			URL:      getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
			Exchange: getEnv("RABBITMQ_EXCHANGE", "avatars.exchange"),
		},
		Upload: UploadConfig{
			MaxSizeBytes: maxSizeBytes,
		},
		Observability: ObservabilityConfig{
			Enabled:      otelEnabled,
			ServiceName:  getEnv("OTEL_SERVICE_NAME", "gophprofile-server"),
			OTLPEndpoint: getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "jaeger:4317"),
			Insecure:     otelInsecure,
			LogLevel:     strings.ToLower(getEnv("LOG_LEVEL", "info")),
			MetricsPath:  getEnv("METRICS_PATH", "/metrics"),
			MetricsAddr:  getEnv("METRICS_ADDR", ":9091"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("HTTP_ADDR is required")
	}
	if c.DB.Host == "" || c.DB.Name == "" || c.DB.User == "" {
		return fmt.Errorf("DB_HOST, DB_NAME and DB_USER are required")
	}
	if c.S3.Endpoint == "" || c.S3.Bucket == "" {
		return fmt.Errorf("S3_ENDPOINT and S3_BUCKET are required")
	}
	if c.RabbitMQ.URL == "" || c.RabbitMQ.Exchange == "" {
		return fmt.Errorf("RABBITMQ_URL and RABBITMQ_EXCHANGE are required")
	}
	if c.Upload.MaxSizeBytes <= 0 {
		return fmt.Errorf("UPLOAD_MAX_SIZE_BYTES must be positive")
	}
	if c.Observability.ServiceName == "" {
		return fmt.Errorf("OTEL_SERVICE_NAME is required")
	}
	if c.Observability.MetricsPath == "" {
		return fmt.Errorf("METRICS_PATH is required")
	}
	switch c.Observability.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("LOG_LEVEL must be debug, info, warn or error")
	}
	if c.Observability.Enabled && c.Observability.OTLPEndpoint == "" {
		return fmt.Errorf("OTEL_EXPORTER_OTLP_ENDPOINT is required when OTEL_ENABLED=true")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, v, err)
	}
	return n, nil
}

func getEnvInt64(key string, fallback int64) (int64, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, v, err)
	}
	return n, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s=%q: %w", key, v, err)
	}
	return b, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s=%q: %w", key, v, err)
	}
	return d, nil
}
