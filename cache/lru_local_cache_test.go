package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/stretchr/testify/require"
)

func TestLRULocalCache_MaxEntriesEvictsLRU(t *testing.T) {
	c := newLRULocalCache(2, 0, time.Minute, 0, nil)
	c.Set("a", []byte("aa"), 0)
	c.Set("b", []byte("bb"), 0)
	_, ok := c.Get("a")
	require.True(t, ok)
	c.Set("c", []byte("cc"), 0)

	require.Equal(t, 2, c.ItemCount())
	_, ok = c.Get("b")
	require.False(t, ok, "b should be evicted as LRU")
	_, ok = c.Get("a")
	require.True(t, ok)
	_, ok = c.Get("c")
	require.True(t, ok)
}

func TestLRULocalCache_MaxBytesEvictsUntilUnderBudget(t *testing.T) {
	c := newLRULocalCache(0, 300, time.Minute, 0, nil)
	for i := 0; i < 5; i++ {
		c.Set(fmt.Sprintf("k%d", i), make([]byte, 100), 0)
	}
	require.LessOrEqual(t, c.Bytes(), int64(300))
	require.LessOrEqual(t, c.ItemCount(), 2)
	_, ok := c.Get("k4")
	require.True(t, ok)
	_, ok = c.Get("k0")
	require.False(t, ok)
}

func TestLRULocalCache_OversizedEntryNotCached(t *testing.T) {
	c := newLRULocalCache(10, 50, time.Minute, 0, nil)
	require.False(t, c.setRetained("big", make([]byte, 200), 0))
	require.Equal(t, 0, c.ItemCount())
	_, ok := c.Get("big")
	require.False(t, ok)

	require.True(t, c.setRetained("small", make([]byte, 10), 0))
	require.Equal(t, 1, c.ItemCount())
}

func TestLRULocalCache_TTLExpiresOnGet(t *testing.T) {
	c := newLRULocalCache(10, 0, 20*time.Millisecond, 0, nil)
	c.Set("a", []byte("x"), 0)
	_, ok := c.Get("a")
	require.True(t, ok)
	time.Sleep(40 * time.Millisecond)
	_, ok = c.Get("a")
	require.False(t, ok)
	require.Equal(t, 0, c.ItemCount())
}

func TestLRULocalCache_DeleteRemovesEntryAndBytes(t *testing.T) {
	c := newLRULocalCache(10, 0, time.Minute, 0, nil)
	c.Set("a", make([]byte, 100), 0)
	require.Equal(t, 1, c.ItemCount())
	require.Greater(t, c.Bytes(), int64(0))
	c.Delete("a")
	require.Equal(t, 0, c.ItemCount())
	require.Equal(t, int64(0), c.Bytes())
	_, ok := c.Get("a")
	require.False(t, ok)
}

func TestLRULocalCache_ReplaceSameKeyTrimsBytes(t *testing.T) {
	c := newLRULocalCache(0, 200, time.Minute, 0, nil)
	c.Set("a", make([]byte, 50), 0)
	before := c.Bytes()
	c.Set("a", make([]byte, 80), 0)
	require.Equal(t, 1, c.ItemCount())
	require.NotEqual(t, before, c.Bytes())
	require.LessOrEqual(t, c.Bytes(), int64(200))
}

func TestLRULocalCache_DeleteExpiredJanitor(t *testing.T) {
	c := newLRULocalCache(10, 0, 30*time.Millisecond, 20*time.Millisecond, nil)
	defer c.StopJanitor()
	c.Set("a", []byte("x"), 0)
	c.Set("b", []byte("y"), 0)
	require.Equal(t, 2, c.ItemCount())
	time.Sleep(80 * time.Millisecond)
	require.Equal(t, 0, c.ItemCount(), "janitor should drop expired entries without a Get")
}

func TestHashCache_WithMaxLocalEntries_CapsGrowth(t *testing.T) {
	ctx := context.Background()
	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true), WithMaxLocalEntries(2))
	for i := 0; i < 10; i++ {
		key := string(domain.NewFeatureConfigsKey(fmt.Sprintf("env%d", i)))
		flags := []domain.FeatureFlag{{Feature: fmt.Sprintf("f%d", i)}}
		require.NoError(t, hc.Set(ctx, key, flags))
		var got []domain.FeatureFlag
		require.NoError(t, hc.Get(ctx, key, &got))
	}
	require.LessOrEqual(t, hc.localItemCount(), 2)
}

func TestHashCache_WithMaxLocalBytes_CapsGrowth(t *testing.T) {
	ctx := context.Background()
	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true), WithMaxLocalBytes(400))
	for i := 0; i < 5; i++ {
		key := string(domain.NewFeatureConfigsKey(fmt.Sprintf("env%d", i)))
		flags := []domain.FeatureFlag{{Feature: fmt.Sprintf("flag-payload-%d", i)}}
		require.NoError(t, hc.Set(ctx, key, flags))
	}
	require.LessOrEqual(t, hc.localItemCount(), 2)
}

func TestHashCache_MaxBytesIgnoredWithoutLatestOnly(t *testing.T) {
	ctx := context.Background()
	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, time.Minute, WithMaxLocalBytes(1))
	_, isLRU := hc.localCache.(*lruLocalCache)
	require.False(t, isLRU, "maxBytes alone without latest-only must not enable LRU")

	hc2 := NewHashCache(backend, time.Minute, time.Minute, WithMaxLocalEntries(2), WithMaxLocalBytes(1))
	lru, ok := hc2.localCache.(*lruLocalCache)
	require.True(t, ok)
	require.Equal(t, int64(0), lru.maxBytes, "byte cap must be zeroed without latest-only")

	key := string(domain.NewFeatureConfigsKey("env1"))
	flags := []domain.FeatureFlag{{Feature: "f1"}}
	require.NoError(t, hc2.Set(ctx, key, flags))
	var got []domain.FeatureFlag
	require.NoError(t, hc2.Get(ctx, key, &got))
	require.Equal(t, 1, hc2.localItemCount())
}

func TestHashCache_LRU_DeleteClearsLocal(t *testing.T) {
	ctx := context.Background()
	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true), WithMaxLocalEntries(10))
	key := string(domain.NewFeatureConfigsKey("env-del"))
	flags := []domain.FeatureFlag{{Feature: "f"}}
	require.NoError(t, hc.Set(ctx, key, flags))
	require.Equal(t, 1, hc.localItemCount())
	require.NoError(t, hc.Delete(ctx, key))
	require.Equal(t, 0, hc.localItemCount())
}
