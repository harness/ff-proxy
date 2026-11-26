package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"

	"github.com/harness/ff-proxy/v2/log"
)

// The HashCache wraps a Cache implementation (via the MemoizeCache) and adds an in-memory caching layer for serialised data.
// It contains its own implementations for certain methods of the Cache interface that are specifically used for caching
// large objects that the Proxy stores.
type HashCache struct {
	Cache
	localCache   internalCache
	requestGroup *singleflight.Group
	latestOnly   bool
	maxEntries   int
	maxBytes     int64
	metrics      *HashCacheMetrics
	sseMetrics   *SSEPayloadMetrics
	logger       log.Logger
}

type hashCacheEntry struct {
	hash  string
	value interface{}
	size  int64 // marshalled JSON size when known (Set path); 0 → LRU stub
}

// HashCacheOption configures the local HashCache behavior.
type HashCacheOption func(*HashCache)

// WithLatestOnly retains one snapshot per collection key instead of one per hash.
// It is disabled by default and applies independently to primary and replica caches.
func WithLatestOnly(enabled bool) HashCacheOption {
	return func(hc *HashCache) { hc.latestOnly = enabled }
}

// WithMaxLocalEntries caps local entries via LRU. 0 = disabled.
func WithMaxLocalEntries(n int) HashCacheOption {
	return func(hc *HashCache) {
		if n > 0 {
			hc.maxEntries = n
		}
	}
}

// WithMaxLocalBytes caps approx local payload bytes via LRU. Applied only with
// latest-only (Set path records marshal size). 0 = disabled.
func WithMaxLocalBytes(n int64) HashCacheOption {
	return func(hc *HashCache) {
		if n > 0 {
			hc.maxBytes = n
		}
	}
}

// WithMetrics enables Prometheus instrumentation for local snapshot sizes.
func WithMetrics(m *HashCacheMetrics) HashCacheOption {
	return func(hc *HashCache) { hc.metrics = m }
}

// WithLogger enables Info logs when a collection snapshot is retained locally.
func WithLogger(l log.Logger) HashCacheOption {
	return func(hc *HashCache) { hc.logger = l }
}

// WithSSEMetrics records SSE arrival→collection growth using the marshal size
// already computed in Set (avoids a second full collection encode in Refresher).
func WithSSEMetrics(m *SSEPayloadMetrics) HashCacheOption {
	return func(hc *HashCache) { hc.sseMetrics = m }
}

// NewHashCache ...
func NewHashCache(c Cache, defaultExpiration, cleanupInterval time.Duration, options ...HashCacheOption) *HashCache {
	hc := &HashCache{
		Cache:        c,
		requestGroup: &singleflight.Group{},
		logger:       log.NewNoOpLogger(),
	}
	for _, option := range options {
		option(hc)
	}
	// maxBytes needs real sizes from Set marshal; ignore without latest-only.
	maxBytes := hc.maxBytes
	if maxBytes > 0 && !hc.latestOnly {
		maxBytes = 0
	}
	if hc.maxEntries > 0 || maxBytes > 0 {
		hc.localCache = newLRULocalCache(hc.maxEntries, maxBytes, defaultExpiration, cleanupInterval, hc.recordLocalDelete)
	} else {
		local := gocache.New(defaultExpiration, cleanupInterval)
		// TTL expiry / Delete must drop gauge tracking; otherwise default multi-hash
		// get_fill leaves keyBytes growing forever while go-cache evicts entries.
		local.OnEvicted(func(k string, _ interface{}) {
			hc.recordLocalDelete(k)
		})
		hc.localCache = local
	}
	return hc
}

func (hc *HashCache) localItemCount() int {
	switch c := hc.localCache.(type) {
	case *gocache.Cache:
		return c.ItemCount()
	case *lruLocalCache:
		return c.ItemCount()
	default:
		return -1
	}
}

// recordLocalFill tracks a get_fill retain without re-encoding the collection.
func (hc *HashCache) recordLocalFill(localKey, collectionKey string) {
	hc.metrics.trackLocalEntry(localKey)
	if hc.logger == nil {
		return
	}
	hc.logger.Info("hash_cache local snapshot stored",
		"key", collectionKey,
		"local_key", localKey,
		"kind", collectionKind(collectionKey),
		"op", "get_fill",
		"size_bytes", "unknown",
		"latest_only", hc.latestOnly,
	)
}

func (hc *HashCache) recordLocalDelete(localKey string) {
	hc.metrics.observeDelete(localKey)
}

func hashKey(collectionKey string) string {
	return collectionKey + "-latest"
}

// isCollectionKey reports whether key is a full feature-configs/segments document
// (the only keys that get a companion -latest hash in Redis).
func isCollectionKey(key string) bool {
	return strings.HasSuffix(key, "segments") || strings.HasSuffix(key, "feature-configs")
}

// Set adds hash key entry for the given key
func (hc HashCache) Set(ctx context.Context, key string, value interface{}) error {
	// First set the key, value that we've been given
	if err := hc.Cache.Set(ctx, key, value); err != nil {
		return fmt.Errorf("HashCache.Set failed to set key=%s in cache: %s", key, err)
	}

	// If the key isn't a segments or feature-configs key then we're done.
	// If it is then we'll want to carry on and set a latest hash for these keys
	if !isCollectionKey(key) {
		return nil
	}

	latestKey := hashKey(key)
	v, err := jsoniter.Marshal(value)
	if err != nil {
		return fmt.Errorf("unable to marshall config %s %v", latestKey, err)
	}
	size := int64(len(v))
	latestHash := sha256.Sum256(v)
	latestHashString := fmt.Sprintf("%x", latestHash)

	if err := hc.Cache.Set(ctx, latestKey, latestHashString); err != nil {
		return err
	}

	// Always record marshalled collection size for SSE-driven refreshes. This is
	// the payload that ends up (or will end up) in the in-memory HashCache layer.
	hc.metrics.observeSnapshot(key, "set", size)
	hc.observeSSERefresh(ctx, key, int(size))
	if hc.logger != nil {
		hc.logger.Info("hash_cache collection set",
			"key", key,
			"kind", collectionKind(key),
			"stage", "post_json_marshal",
			"json_size_bytes", size,
			"hash", latestHashString[:12],
			"latest_only", hc.latestOnly,
		)
	}

	// latest-only: write-through an owned local snapshot so Refresher Set does not
	// leave a stale-hash entry, and callers cannot share the writer's backing array.
	if hc.latestOnly {
		owned, err := snapshotPtr(value)
		if err != nil {
			hc.localCache.Delete(key)
			hc.recordLocalDelete(key)
			return fmt.Errorf("HashCache.Set failed to snapshot key=%s for local cache: %w", key, err)
		}
		hc.localCache.Set(key, hashCacheEntry{hash: latestHashString, value: owned, size: size}, 0)
		hc.metrics.trackLocalBytes(key, size)
	}

	return nil
}

// observeSSERefresh emits arrival→collection growth using Set's existing marshal size.
func (hc *HashCache) observeSSERefresh(ctx context.Context, key string, jsonBytes int) {
	meta, ok := sseRefreshMetaFrom(ctx)
	if !ok {
		return
	}
	kind := meta.kind
	if kind == "" {
		kind = collectionKind(key)
	}
	hc.sseMetrics.observeRefresh(kind, meta.domain, meta.event, meta.arrivalBytes, jsonBytes)
	if hc.logger == nil {
		return
	}
	hc.logger.Info("sse refresh payload growth",
		"kind", kind,
		"domain", meta.domain,
		"event", meta.event,
		"sse_arrival_bytes", meta.arrivalBytes,
		"collection_json_bytes", jsonBytes,
		"growth_bytes", jsonBytes-meta.arrivalBytes,
	)
}

func (hc HashCache) Get(ctx context.Context, key string, value interface{}) error {
	data, err, _ := hc.requestGroup.Do(key, func() (interface{}, error) {
		return hc.get(ctx, key, value)
	})
	if err != nil {
		return err
	}

	val := reflect.ValueOf(value)
	respValue := reflect.ValueOf(data)
	if respValue.Kind() == reflect.Ptr {
		val.Elem().Set(respValue.Elem())
	} else {
		val.Elem().Set(respValue)
	}
	return nil
}

// Get checks the local cache for the key and returns it if there.
func (hc HashCache) get(ctx context.Context, key string, value interface{}) (interface{}, error) {
	if hc.latestOnly {
		return hc.getLatest(ctx, key, value)
	}

	latestKey := hashKey(key)

	var hash string
	err := hc.Cache.Get(ctx, latestKey, &hash)
	if err == nil {
		if data, ok := hc.localCache.Get(hash); ok {
			return data, nil
		}
	}

	// fetch from redis
	err = hc.Cache.Get(ctx, key, value)
	if err != nil {
		return value, err
	}
	// set the value in local
	if hash != "" {
		hc.localCache.Set(hash, value, 0)
		// Do not re-marshal the collection for metrics — that doubles encode cost on
		// the SSE/read hot path. Entry count still tracks multi-hash churn; exact
		// bytes come from Set's existing marshal (see observeSnapshot / latest-only).
		hc.recordLocalFill(hash, key)
	}
	return value, err
}

func (hc HashCache) getLatest(ctx context.Context, key string, value interface{}) (interface{}, error) {
	output := reflect.ValueOf(value)
	if output.Kind() != reflect.Ptr || output.IsNil() {
		return nil, fmt.Errorf("HashCache.Get requires a non-nil pointer for key=%s", key)
	}

	var hash string
	if err := hc.Cache.Get(ctx, hashKey(key), &hash); err == nil && hash != "" {
		if data, ok := hc.localCache.Get(key); ok {
			entry, ok := data.(hashCacheEntry)
			if !ok {
				hc.localCache.Delete(key)
				hc.recordLocalDelete(key)
			} else if entry.hash == hash {
				// Return a clone so Refresher/SDK callers cannot mutate the cached snapshot.
				return cloneSnapshot(entry.value)
			}
		}
	}

	// Drop an obsolete snapshot before fetching its replacement, including when
	// the hash is missing. Each replica invalidates its own local copy on reads.
	hc.localCache.Delete(key)
	hc.recordLocalDelete(key)
	// Own the decode destination so reusing a caller's output cannot mutate the
	// cached pointer or reuse a previous snapshot's slice backing array.
	data := reflect.New(output.Type().Elem()).Interface()
	if err := hc.Cache.Get(ctx, key, data); err != nil {
		return nil, err
	}
	if hash != "" {
		hc.localCache.Set(key, hashCacheEntry{hash: hash, value: data}, 0)
		hc.recordLocalFill(key, key)
		return cloneSnapshot(data)
	}
	return data, nil
}

// Delete key from local cache as well as hash entry in the redis
func (hc HashCache) Delete(ctx context.Context, key string) error {
	latestKey := hashKey(key)
	if hc.latestOnly {
		hc.localCache.Delete(key)
		hc.recordLocalDelete(key)
	} else {
		var hash string
		if err := hc.Cache.Get(ctx, latestKey, &hash); err == nil {
			hc.localCache.Delete(hash)
			hc.recordLocalDelete(hash)
		}
	}
	err := hc.Cache.Delete(ctx, latestKey)
	if err != nil {
		return err
	}
	return hc.Cache.Delete(ctx, key)
}

// snapshotPtr JSON-round-trips value into a fresh *T for local retention.
// If value is already *T, result is still *T (not **T).
func snapshotPtr(value interface{}) (interface{}, error) {
	if value == nil {
		return nil, fmt.Errorf("nil snapshot value")
	}
	b, err := jsoniter.Marshal(value)
	if err != nil {
		return nil, err
	}
	t := reflect.TypeOf(value)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	ptr := reflect.New(t).Interface()
	if err := jsoniter.Unmarshal(b, ptr); err != nil {
		return nil, err
	}
	return ptr, nil
}

// cloneSnapshot returns a deep copy of a cached pointer snapshot (*T → new *T).
func cloneSnapshot(value interface{}) (interface{}, error) {
	v := reflect.ValueOf(value)
	if !v.IsValid() || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return value, nil
	}
	b, err := jsoniter.Marshal(value)
	if err != nil {
		return nil, err
	}
	out := reflect.New(v.Type().Elem()).Interface()
	if err := jsoniter.Unmarshal(b, out); err != nil {
		return nil, err
	}
	return out, nil
}
