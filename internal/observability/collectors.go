package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

const defaultStorageInterval = 15 * time.Second

var registerDBPoolOnce sync.Once

type dbPoolCollector struct {
	pool     *pgxpool.Pool
	acquired *prometheus.Desc
	idle     *prometheus.Desc
	max      *prometheus.Desc
}

func newDBPoolCollector(pool *pgxpool.Pool) *dbPoolCollector {
	return &dbPoolCollector{
		pool: pool,
		acquired: prometheus.NewDesc(
			"db_pool_acquired",
			"Number of currently acquired Postgres connections",
			nil, nil,
		),
		idle: prometheus.NewDesc(
			"db_pool_idle",
			"Number of idle Postgres connections",
			nil, nil,
		),
		max: prometheus.NewDesc(
			"db_pool_max",
			"Maximum Postgres pool size",
			nil, nil,
		),
	}
}

func (c *dbPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.acquired
	ch <- c.idle
	ch <- c.max
}

func (c *dbPoolCollector) Collect(ch chan<- prometheus.Metric) {
	stat := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(c.acquired, prometheus.GaugeValue, float64(stat.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(c.idle, prometheus.GaugeValue, float64(stat.IdleConns()))
	ch <- prometheus.MustNewConstMetric(c.max, prometheus.GaugeValue, float64(stat.MaxConns()))
}

func RegisterDBPool(pool *pgxpool.Pool) {
	if pool == nil {
		return
	}
	registerDBPoolOnce.Do(func() {
		prometheus.MustRegister(newDBPoolCollector(pool))
	})
}

func CollectStorageBytes(ctx context.Context, query func(context.Context) (int64, error), interval time.Duration, log *slog.Logger) {
	if query == nil {
		return
	}
	if log == nil {
		log = slog.Default()
	}
	if interval <= 0 {
		interval = defaultStorageInterval
	}

	update := func() {
		n, err := query(ctx)
		if err != nil {
			log.WarnContext(ctx, "collect storage bytes", "err", err)
			return
		}
		avatarsStorageBytes.Set(float64(n))
	}

	update()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			update()
		}
	}
}
