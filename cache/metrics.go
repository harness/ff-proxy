package cache

import (
	"context"
	"encoding"
	"errors"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/domain"
	"github.com/prometheus/client_golang/prometheus"
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

	writeCount  counter
	readCount   counter
	deleteCount counter
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

		writeCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_proxy_%s_cache_write_count", label),
			Help: "Tracks how many writes we make to the cache",
		},
			[]string{"key", "operation", "error"},
		),
		readCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_proxy_%s_cache_read_count", label),
			Help: "Tracks how many reads we make to the cache",
		},
			[]string{"key", "operation", "error"},
		),
		deleteCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("ff_proxy_%s_cache_remove_count", label),
			Help: "Tracks how many deletes we make to the cache",
		},
			[]string{"key", "operation"},
		),
	}

	reg.MustRegister(
		c.writeDuration,
		c.readDuration,
		c.deleteDuration,
		c.writeCount,
		c.readCount,
		c.deleteCount,
	)
	return c
}

// Set makes MetricsCache implement the Cache interface. It calls the decorated cache's Set method and
// uses a prometheus counter and histogram to track the number of calls and how long a Set operation takes.
func (c MetricsCache) Set(ctx context.Context, key string, field string, value encoding.BinaryMarshaler) (err error) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.writeDuration)
		trackCounter(c.writeCount, key, "Set", getErrorLabel(err))
	}()

	return c.next.Set(ctx, key, field, value)
}

// SetByte makes MetricsCache implement the Cache interface. It calls the decorated cache's SetByte method and
// uses a prometheus counter and histogram to track the number of calls and how long a SetByte operation takes.
func (c MetricsCache) SetByte(ctx context.Context, key string, field string, value []byte) (err error) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.writeDuration)
		trackCounter(c.writeCount, key, "SetByte", getErrorLabel(err))
	}()

	return c.next.SetByte(ctx, key, field, value)
}

// Get makes MetricsCache implement the Cache interface. It calls the decorated cache's Get method and
// uses a prometheus counter and histogram to track the number of calls and how long a Get operation takes.
func (c MetricsCache) Get(ctx context.Context, key string, field string, v encoding.BinaryUnmarshaler) (err error) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.readDuration)
		trackCounter(c.readCount, key, "Get", getErrorLabel(err))
	}()

	return c.next.Get(ctx, key, field, v)
}

// GetByte makes MetricsCache implement the Cache interface. It calls the decorated cache's GetByte method and
// uses a prometheus counter and histogram to track the number of calls and how long a GetByte operation takes.
func (c MetricsCache) GetByte(ctx context.Context, key string, field string) (b []byte, err error) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.readDuration)
		trackCounter(c.readCount, key, "GetByte", getErrorLabel(err))
	}()

	return c.next.GetByte(ctx, key, field)
}

// GetAll makes MetricsCache implement the Cache interface. It calls the decorated cache's GetAll method and
// uses a prometheus counter and histogram to track the number of calls and how long a GetAll operation takes.
func (c MetricsCache) GetAll(ctx context.Context, key string) (m map[string][]byte, err error) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.readDuration)
		trackCounter(c.readCount, key, "GetAll", getErrorLabel(err))
	}()

	return c.next.GetAll(ctx, key)
}

// RemoveAll makes MetricsCache implement the Cache interface. It calls the decorated cache's RemoveAll method and
// uses a prometheus counter and histogram to track the number of calls and how long a RemoveAll operation takes.
func (c MetricsCache) RemoveAll(ctx context.Context, key string) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.deleteDuration)
		trackCounter(c.deleteCount, key, "RemoveAll")
	}()

	c.next.RemoveAll(ctx, key)
}

// Remove makes MetricsCache implement the Cache interface. It calls the decorated cache's Remove method and
// uses a prometheus counter and histogram to track the number of calls and how long a Remove operation takes.
func (c MetricsCache) Remove(ctx context.Context, key string, field string) {
	start := time.Now()
	defer func() {
		trackHistogram(start, c.deleteDuration)
		trackCounter(c.deleteCount, key, "Remove")
	}()

	c.next.Remove(ctx, key, field)
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
