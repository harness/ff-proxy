package cache

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

func TestHashCacheMetrics_SetRecordsLocalSnapshotSize(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewHashCacheMetrics(reg)

	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true), WithMetrics(metrics))

	key := string(domain.NewFeatureConfigsKey("env-1"))
	flags := []domain.FeatureFlag{{Feature: "flag-a"}, {Feature: "flag-b"}}
	require.NoError(t, hc.Set(context.Background(), key, flags))

	assert.Equal(t, 1, testutil.CollectAndCount(metrics.snapshotBytes))
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.storeTotal.WithLabelValues("feature-configs", "set")))
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.localEntries))
	assert.Greater(t, testutil.ToFloat64(metrics.localBytes), float64(0))
}

func TestHashCacheMetrics_DeclinedLRUSetSkipsLocalGauges(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewHashCacheMetrics(reg)

	backend, _ := newHashCacheTestBackend(t)
	// Budget smaller than any marshalled feature-configs payload → LRU declines.
	hc := NewHashCache(backend, time.Minute, 0,
		WithLatestOnly(true),
		WithMaxLocalBytes(1),
		WithMetrics(metrics),
	)

	key := string(domain.NewFeatureConfigsKey("env-1"))
	flags := []domain.FeatureFlag{{Feature: "flag-a"}, {Feature: "flag-b"}}
	require.NoError(t, hc.Set(context.Background(), key, flags))

	assert.Equal(t, 0, hc.localItemCount())
	assert.Equal(t, float64(0), testutil.ToFloat64(metrics.localEntries))
	assert.Equal(t, float64(0), testutil.ToFloat64(metrics.localBytes))
	// Store still succeeded in Redis; histogram/counters still observe the write.
	assert.Equal(t, float64(1), testutil.ToFloat64(metrics.storeTotal.WithLabelValues("feature-configs", "set")))
}

func TestHashCacheMetrics_DeleteClearsLocalBytes(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewHashCacheMetrics(reg)

	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true), WithMetrics(metrics))

	key := string(domain.NewSegmentsKey("env-1"))
	require.NoError(t, hc.Set(context.Background(), key, []domain.Segment{{Identifier: "s1"}}))
	require.Greater(t, testutil.ToFloat64(metrics.localBytes), float64(0))

	require.NoError(t, hc.Delete(context.Background(), key))
	assert.Equal(t, float64(0), testutil.ToFloat64(metrics.localEntries))
	assert.Equal(t, float64(0), testutil.ToFloat64(metrics.localBytes))
}

func TestHashCacheMetrics_TTLEvictionClearsLocalBytes(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewHashCacheMetrics(reg)

	backend, _ := newHashCacheTestBackend(t)
	// Short TTL + cleanup so OnEvicted fires without an explicit Delete.
	hc := NewHashCache(backend, 50*time.Millisecond, 20*time.Millisecond, WithMetrics(metrics))

	key := string(domain.NewFeatureConfigsKey("env-1"))
	require.NoError(t, hc.Set(context.Background(), key, []domain.FeatureFlag{{Feature: "flag-a"}}))

	var out []domain.FeatureFlag
	require.NoError(t, hc.Get(context.Background(), key, &out))
	require.Greater(t, testutil.ToFloat64(metrics.localEntries), float64(0))

	require.Eventually(t, func() bool {
		return testutil.ToFloat64(metrics.localEntries) == 0 && testutil.ToFloat64(metrics.localBytes) == 0
	}, 2*time.Second, 25*time.Millisecond)
}

func TestHashCache_SSERefreshUsesSetMarshalSize(t *testing.T) {
	reg := prometheus.NewRegistry()
	hashMetrics := NewHashCacheMetrics(reg)
	sseMetrics := NewSSEPayloadMetrics(reg)

	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithMetrics(hashMetrics), WithSSEMetrics(sseMetrics), WithLogger(log.NewNoOpLogger()))

	key := string(domain.NewFeatureConfigsKey("env-1"))
	flags := []domain.FeatureFlag{{Feature: "flag-a"}, {Feature: "flag-b"}}
	msg := domain.SSEMessage{
		Domain: domain.MsgDomainFeature, Event: domain.EventPatch, RawSize: 100,
	}
	ctx := ContextWithSSERefresh(context.Background(), msg, "feature-configs")
	require.NoError(t, hc.Set(ctx, key, flags))

	assert.Equal(t, float64(1), testutil.ToFloat64(sseMetrics.refreshTotal.WithLabelValues("feature-configs", domain.MsgDomainFeature, domain.EventPatch)))
	assert.Equal(t, 1, testutil.CollectAndCount(sseMetrics.collectionBytes))
	assert.Equal(t, 1, testutil.CollectAndCount(sseMetrics.growthBytes))
}

func TestHashCache_SSERefreshInfersKindWhenEmpty(t *testing.T) {
	reg := prometheus.NewRegistry()
	sseMetrics := NewSSEPayloadMetrics(reg)
	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithSSEMetrics(sseMetrics))

	key := string(domain.NewFeatureConfigsKey("env-2"))
	msg := domain.SSEMessage{Domain: domain.MsgDomainFeature, Event: domain.EventCreate, RawSize: 50}
	// Empty kind forces collectionKind(key) path in observeSSERefresh.
	ctx := ContextWithSSERefresh(context.Background(), msg, "")
	require.NoError(t, hc.Set(ctx, key, []domain.FeatureFlag{{Feature: "x"}}))
	assert.Equal(t, float64(1), testutil.ToFloat64(sseMetrics.refreshTotal.WithLabelValues("feature-configs", domain.MsgDomainFeature, domain.EventCreate)))
}

func TestHashCacheMetrics_Helpers(t *testing.T) {
	assert.Equal(t, "other", collectionKind("env-1/something-else"))
	assert.Equal(t, "feature-configs", collectionKind("env-1/feature-configs"))
	assert.Equal(t, "segments", collectionKind("env-1/segments"))

	reg := prometheus.NewRegistry()
	m := NewHashCacheMetrics(reg)
	m.trackLocalBytes("k1", 100)
	m.trackLocalBytes("k1", 150) // update existing
	m.trackLocalEntry("k2")
	m.trackLocalEntry("k2") // no-op
	assert.Equal(t, float64(2), testutil.ToFloat64(m.localEntries))
	assert.Equal(t, float64(150), testutil.ToFloat64(m.localBytes))

	m.observeDelete("k1")
	m.observeDelete("missing")
	assert.Equal(t, float64(1), testutil.ToFloat64(m.localEntries))
	assert.Equal(t, float64(0), testutil.ToFloat64(m.localBytes))
}
