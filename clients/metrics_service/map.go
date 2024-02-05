package metricsservice

import (
	"sync"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

// metricsMap is a type that stores metrics requests
// and aggregates them by environment
type metricsMap struct {
	*sync.RWMutex
	metrics     map[string]domain.MetricsRequest
	currentSize int
}

func newMetricsMap() *metricsMap {
	return &metricsMap{
		RWMutex: &sync.RWMutex{},
		metrics: make(map[string]domain.MetricsRequest),
	}
}

func (m *metricsMap) add(r domain.MetricsRequest) {
	m.Lock()
	defer m.Unlock()

	m.currentSize += r.Size

	// Store metrics to send later
	currentMetrics, ok := m.metrics[r.EnvironmentID]
	if !ok {
		m.metrics[r.EnvironmentID] = r
		return
	}

	incrSize := false

	if r.MetricsData != nil {
		incrSize = true

		if currentMetrics.MetricsData == nil {
			currentMetrics.MetricsData = &[]clientgen.MetricsData{}
		}
		newMetrics := append(*currentMetrics.MetricsData, *r.MetricsData...)
		currentMetrics.MetricsData = &newMetrics
	}

	if r.TargetData != nil {
		incrSize = true

		if currentMetrics.TargetData == nil {
			currentMetrics.TargetData = &[]clientgen.TargetData{}
		}

		newTargets := append(*currentMetrics.TargetData, *r.TargetData...)
		currentMetrics.TargetData = &newTargets
	}

	// As well as aggregating the metrics & target data we need to
	// 'merge' the size of the current aggregated object and the new one
	if incrSize {
		currentMetrics.Size += r.Size
	}

	m.metrics[r.EnvironmentID] = currentMetrics
}

func (m *metricsMap) get() map[string]domain.MetricsRequest {
	m.RLock()
	defer m.RUnlock()

	return m.metrics
}

func (m *metricsMap) flush() {
	m.Lock()
	defer m.Unlock()

	m.metrics = map[string]domain.MetricsRequest{}
	m.currentSize = 0
}

func (m *metricsMap) size() int {
	m.RLock()
	defer m.RUnlock()

	return m.currentSize
}
