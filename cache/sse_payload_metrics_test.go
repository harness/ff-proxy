package cache

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/domain"
)

func TestSSEPayloadMetrics_ObserveArrivalAndRefresh(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewSSEPayloadMetrics(reg)

	m.observeArrival(domain.MsgDomainFeature, domain.EventPatch, 120)
	m.observeArrival("", "", -1) // ignored
	m.observeRefresh("feature-configs", domain.MsgDomainFeature, domain.EventPatch, 120, 50_000)
	m.observeRefresh("segments", "", "", 200, 100) // clamps negative growth; empty labels → unknown

	assert.Equal(t, float64(1), testutil.ToFloat64(m.refreshTotal.WithLabelValues("feature-configs", domain.MsgDomainFeature, domain.EventPatch)))
	assert.Equal(t, float64(1), testutil.ToFloat64(m.refreshTotal.WithLabelValues("segments", "unknown", "unknown")))
	assert.Equal(t, 1, testutil.CollectAndCount(m.arrivalBytes))
	assert.Equal(t, 2, testutil.CollectAndCount(m.collectionBytes))
	assert.Equal(t, 2, testutil.CollectAndCount(m.growthBytes))
}
