package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/harness/ff-proxy/v2/domain"
)

type counter interface {
	prometheus.Collector
	WithLabelValues(lvs ...string) prometheus.Counter
}

type histogram interface {
	prometheus.Collector
	WithLabelValues(lvs ...string) prometheus.Observer
}

// MetricsCache is a decorator for a Cache that uses prometheus to track
// read/write activity in the cache
type MetricsCache struct {
	next Cache

	writeDuration  histogram
	readDuration   histogram
	deleteDuration histogram
	scanDuration   histogram

	writeCount  counter
	readCount   counter
	deleteCount counter
	scanCount   counter
}

func (c MetricsCache) Scan(ctx context.Context, key string) (m map[string]string, err error) {
	start := time.Now()
	defer func() {

		trackHistogram(start, c.scanDuration)
		trackCounter(c.scanCount, key, getErrorLabel(err))
	}()
	return c.next.Scan(ctx, key)
}

// NewMetricsCache creates a MetricsCache
func NewMetricsCache(label string, reg prometheus.Registerer, next Cache) MetricsCache {
	c := MetricsCache{
		next: next,

		writeDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    fmt.Sprintf("ff_proxy_%s_cache_write_duration", label),
			Help:    "Tracks how long write operations to the cache take",
			Buckets: []float64{0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5},
		},
			[]string{},
		),
		readDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    fmt.Sprintf("ff_proxy_%s_cache_read_duration", label),
			Help:    "Tracks how long write operations to the cache take",
			Buckets: []float64{0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5},
		},
			[]string{},
		),
		deleteDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    fmt.Sprintf("ff_proxy_%s_cache_delete_duration", label),
			Help:    "Tracks how long delete operations to the cache take",
			Buckets: []float64{0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5},
		},
			[]string{},
		),
		scanDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    fmt.Sprintf("ff_proxy_%s_cache_scan_duration", label),
			Help:    "Tracks how long delete operations to the cache take",
			Buckets: []float64{0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5},
		},
			[]string{},
		),

		writeCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_proxy_%s_cache_write_count", label),
			Help: "Tracks how many writes we make to the cache",
		},
			[]string{"error"},
		),
		readCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_proxy_%s_cache_read_count", label),
			Help: "Tracks how many reads we make to the cache",
		},
			[]string{"error"},
		),
		deleteCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_proxy_%s_cache_remove_count", label),
			Help: "Tracks how many deletes we make to the cache",
		},
			[]string{"error"},
		),
		scanCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_proxy_%s_cache_scan_count", label),
			Help: "Tracks how many scans for keys we make to the cache per environment",
		},
			[]string{"error"},
		),
	}

	reg.MustRegister(
		c.writeDuration,
		c.readDuration,
		c.deleteDuration,
		c.scanDuration,
		c.writeCount,
		c.readCount,
		c.deleteCount,
		c.scanCount,
	)
	return c
}

// Set makes MetricsCache implement the Cache interface. It calls the decorated cache's Set method and
// uses a prometheus counter and histogram to track the number of calls and how long a Set operation takes.
func (c MetricsCache) Set(ctx context.Context, key string, value interface{}) (err error) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.writeDuration)
		trackCounter(c.writeCount, key, getErrorLabel(err))
	}()

	return c.next.Set(ctx, key, value)
}

// Get makes MetricsCache implement the Cache interface. It calls the decorated cache's Get method and
// uses a prometheus counter and histogram to track the number of calls and how long a Get operation takes.
func (c MetricsCache) Get(ctx context.Context, key string, v interface{}) (err error) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.readDuration)
		trackCounter(c.readCount, key, getErrorLabel(err))
	}()

	return c.next.Get(ctx, key, v)
}

// Delete makes MetricsCache implement the Cache interface. It calls the decorated cache's delete method
// and uses a prometheus counter and histogram to track the number of calls and how long each call takes
func (c MetricsCache) Delete(ctx context.Context, key string) (err error) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.deleteDuration)
		trackCounter(c.deleteCount, key, getErrorLabel(err))
	}()

	return c.next.Delete(ctx, key)
}

// Keys makes MetricsCache implement the Cache interface. It calls the decorated cache's Keys method
// and returns the results. It doesn't record any prometheus metrics.
func (c MetricsCache) Keys(ctx context.Context, key string) ([]string, error) {
	return c.next.Keys(ctx, key)
}

// HealthCheck calls the decorated cache's HealthCheck method
func (c MetricsCache) HealthCheck(ctx context.Context) error {
	return c.next.HealthCheck(ctx)
}

func trackHistogram(start time.Time, metric histogram, labels ...string) {
	metric.WithLabelValues(labels...).Observe(time.Since(start).Seconds())
}

func trackCounter(metric counter, labels ...string) {
	metric.WithLabelValues(labels...).Inc()
}

func getErrorLabel(err error) string {
	if err != nil {
		// Don't want to track NotFound as an error in prometheus metrics
		if errors.Is(err, domain.ErrCacheNotFound) {
			return "false"
		}
		return "true"
	}
	return "false"
}
