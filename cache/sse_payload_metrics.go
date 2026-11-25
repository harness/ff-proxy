package cache

import (
	"github.com/prometheus/client_golang/prometheus"
)

// SSEPayloadMetrics tracks SSE wire size vs the marshalled collection JSON built
// for HashCache — filterable in Grafana by kind/domain/event (no env label: cardinality).
type SSEPayloadMetrics struct {
	arrivalBytes    *prometheus.HistogramVec
	collectionBytes *prometheus.HistogramVec
	growthBytes     *prometheus.HistogramVec
	refreshTotal    *prometheus.CounterVec
}

// NewSSEPayloadMetrics registers SSE→cache payload size metrics on reg.
func NewSSEPayloadMetrics(reg prometheus.Registerer) *SSEPayloadMetrics {
	m := &SSEPayloadMetrics{
		arrivalBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ff_proxy_sse_arrival_bytes",
			Help: "Raw SSE event payload size in bytes when the notification arrives (before fetch/refresh)",
			Buckets: []float64{
				64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384,
			},
		}, []string{"domain", "event"}),
		collectionBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ff_proxy_sse_collection_json_bytes",
			Help: "Marshalled feature-configs/segments collection JSON size in bytes after SSE-driven refresh (what HashCache stores)",
			Buckets: []float64{
				1 << 10,
				4 << 10,
				16 << 10,
				64 << 10,
				256 << 10,
				1 << 20,
				2 << 20,
				4 << 20,
				8 << 20,
				16 << 20,
				32 << 20,
			},
		}, []string{"kind", "domain", "event"}),
		growthBytes: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ff_proxy_sse_payload_growth_bytes",
			Help: "collection_json_bytes minus sse_arrival_bytes — inflation from SSE notification to cached JSON payload",
			Buckets: []float64{
				1 << 10,
				4 << 10,
				16 << 10,
				64 << 10,
				256 << 10,
				1 << 20,
				2 << 20,
				4 << 20,
				8 << 20,
				16 << 20,
				32 << 20,
			},
		}, []string{"kind", "domain", "event"}),
		refreshTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ff_proxy_sse_payload_refresh_total",
			Help: "Count of SSE refreshes that built a collection JSON payload for HashCache",
		}, []string{"kind", "domain", "event"}),
	}
	reg.MustRegister(m.arrivalBytes, m.collectionBytes, m.growthBytes, m.refreshTotal)
	return m
}

func (m *SSEPayloadMetrics) observeArrival(domain, event string, size int) {
	if m == nil || size < 0 {
		return
	}
	m.arrivalBytes.WithLabelValues(labelOrUnknown(domain), labelOrUnknown(event)).Observe(float64(size))
}

func (m *SSEPayloadMetrics) observeRefresh(kind, domain, event string, arrivalBytes, collectionBytes int) {
	if m == nil || collectionBytes < 0 {
		return
	}
	d := labelOrUnknown(domain)
	e := labelOrUnknown(event)
	k := labelOrUnknown(kind)
	m.collectionBytes.WithLabelValues(k, d, e).Observe(float64(collectionBytes))
	m.refreshTotal.WithLabelValues(k, d, e).Inc()
	growth := collectionBytes - arrivalBytes
	if growth < 0 {
		growth = 0
	}
	m.growthBytes.WithLabelValues(k, d, e).Observe(float64(growth))
}

func labelOrUnknown(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}
