package cache

import (
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// HashCacheMetrics tracks sizes of collections retained in the HashCache local
// in-memory layer (the snapshots that drive FFM-13229 heap growth under SSE).
type HashCacheMetrics struct {
	snapshotBytes *prometheus.HistogramVec
	storeTotal    *prometheus.CounterVec
	localEntries  prometheus.Gauge
	localBytes    prometheus.Gauge

	mu       sync.Mutex
	keyBytes map[string]int64
}

// NewHashCacheMetrics registers HashCache size metrics on reg.
func NewHashCacheMetrics(reg prometheus.Registerer) *HashCacheMetrics {
	m := &HashCacheMetrics{
		snapshotBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ff_proxy_hash_cache_snapshot_bytes",
			Help: "Marshalled size in bytes of a feature-configs/segments collection when stored in the HashCache local in-memory layer",
			// Tuned for real collections: lab ~30KiB, prod dogfood often MiB-class.
			Buckets: []float64{
				1 << 10, // 1 KiB
				4 << 10,
				16 << 10,
				64 << 10,
				256 << 10,
				1 << 20, // 1 MiB
				2 << 20,
				4 << 20,
				8 << 20,
				16 << 20,
				32 << 20,
			},
		}, []string{"kind", "op"}),
		storeTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ff_proxy_hash_cache_local_store_total",
			Help: "Count of times a collection snapshot was written into the HashCache local in-memory layer",
		}, []string{"kind", "op"}),
		localEntries: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ff_proxy_hash_cache_local_entries",
			Help: "Approximate number of entries currently retained in the HashCache local in-memory layer",
		}),
		localBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ff_proxy_hash_cache_local_bytes",
			Help: "Approximate total marshalled bytes currently retained in the HashCache local in-memory layer",
		}),
		keyBytes: make(map[string]int64),
	}
	reg.MustRegister(m.snapshotBytes, m.storeTotal, m.localEntries, m.localBytes)
	return m
}

func collectionKind(key string) string {
	switch {
	case strings.HasSuffix(key, "feature-configs"):
		return "feature-configs"
	case strings.HasSuffix(key, "segments"):
		return "segments"
	default:
		return "other"
	}
}

func (m *HashCacheMetrics) observeSnapshot(collectionKey, op string, size int64) {
	if m == nil || size < 0 {
		return
	}
	kind := collectionKind(collectionKey)
	m.snapshotBytes.WithLabelValues(kind, op).Observe(float64(size))
	m.storeTotal.WithLabelValues(kind, op).Inc()
}

// trackLocalBytes updates the approximate in-memory retained size gauges.
func (m *HashCacheMetrics) trackLocalBytes(localKey string, size int64) {
	if m == nil || size < 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, ok := m.keyBytes[localKey]; ok {
		m.localBytes.Add(float64(size - prev))
	} else {
		m.localBytes.Add(float64(size))
		m.localEntries.Inc()
	}
	m.keyBytes[localKey] = size
}

// trackLocalEntry records that a local key exists without knowing marshalled size.
// Used on get_fill so we do not re-encode MiB-class collections solely for metrics;
// Set already observes exact sizes when it marshals for hashing.
func (m *HashCacheMetrics) trackLocalEntry(localKey string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.keyBytes[localKey]; exists {
		return
	}
	m.keyBytes[localKey] = 0
	m.localEntries.Inc()
}

func (m *HashCacheMetrics) observeDelete(localKey string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, ok := m.keyBytes[localKey]; ok {
		delete(m.keyBytes, localKey)
		m.localBytes.Sub(float64(prev))
		m.localEntries.Dec()
	}
}
