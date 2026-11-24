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
)

// The HashCache wraps a Cache implementation (via the MemoizeCache) and adds an in-memory caching layer for serialised data.
// It contains its own implementations for certain methods of the Cache interface that are specifically used for caching
// large objects that the Proxy stores.
//
// Three things in this file are easy to confuse, so they each have one name:
//
//	collection key: env-<id>-feature-configs, the full collection stored in Redis
//	hash key:       env-<id>-feature-configs-latest, the sha256 of that collection, written by Set
//	snapshot:       a collection's marshalled bytes, plus the hash they were stored at
//
// In latest-only mode the invariant is: each collection key retains at most one
// snapshot, and a snapshot is served only while its hash still equals the hash
// key in Redis.
type HashCache struct {
	Cache
	localCache   internalCache
	requestGroup *singleflight.Group
	latestOnly   bool
}

// snapshot is owned by the cache. The bytes are never mutated and are never
// handed to a caller, so any number of readers can decode from them without
// copying and without being able to corrupt the cached entry.
type snapshot struct {
	hash string
	data []byte
}

// HashCacheOption configures the local HashCache behavior.
type HashCacheOption func(*HashCache)

// WithLatestOnly retains one snapshot per collection key instead of one per hash.
// It is disabled by default and applies independently to primary and replica caches.
func WithLatestOnly(enabled bool) HashCacheOption {
	return func(hc *HashCache) { hc.latestOnly = enabled }
}

// NewHashCache ...
func NewHashCache(c Cache, defaultExpiration, cleanupInterval time.Duration, options ...HashCacheOption) *HashCache {
	hc := &HashCache{
		Cache:        c,
		localCache:   gocache.New(defaultExpiration, cleanupInterval),
		requestGroup: &singleflight.Group{},
	}
	for _, option := range options {
		option(hc)
	}
	return hc
}

// hashKey returns the key holding the hash of the collection stored at key.
func hashKey(collectionKey string) string {
	return collectionKey + "-latest"
}

// isCollectionKey reports whether key holds a whole collection, which is the
// only kind of key that gets a hash key written alongside it.
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

	data, err := jsoniter.Marshal(value)
	if err != nil {
		return fmt.Errorf("unable to marshall config %s %v", hashKey(key), err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	if err := hc.Cache.Set(ctx, hashKey(key), hash); err != nil {
		return err
	}

	// latest-only: write through the bytes we just marshalled so a Refresher Set
	// does not leave a stale-hash entry behind. They are already a private copy,
	// so the writer's value cannot be reached through the cache.
	if hc.latestOnly {
		hc.localCache.Set(key, snapshot{hash: hash, data: data}, 0)
	}

	return nil
}

func (hc HashCache) Get(ctx context.Context, key string, value interface{}) error {
	// latest-only only changes how collection keys are cached, because they are
	// the only keys with a hash key to validate a local snapshot against. Every
	// other key keeps the default path.
	if hc.latestOnly && isCollectionKey(key) {
		return hc.getSnapshot(ctx, key, value)
	}

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
	// Only collection keys have a hash key written alongside them, so only they
	// can ever be served locally. Anything else goes straight to Redis instead of
	// paying for a hash key lookup that always misses.
	if !isCollectionKey(key) {
		err := hc.Cache.Get(ctx, key, value)
		return value, err
	}

	var hash string
	err := hc.Cache.Get(ctx, hashKey(key), &hash)
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
	}
	return value, err
}

// getSnapshot serves a collection from its local snapshot while the snapshot's
// hash still matches Redis. One fetch is shared between concurrent callers and
// each caller decodes its own value out of the shared bytes.
func (hc HashCache) getSnapshot(ctx context.Context, key string, value interface{}) error {
	data, err, _ := hc.requestGroup.Do(key, func() (interface{}, error) {
		return hc.snapshotBytes(ctx, key, value)
	})
	if err != nil {
		return err
	}

	return jsoniter.Unmarshal(data.([]byte), value)
}

// snapshotBytes returns the marshalled collection stored at key. dst is the
// destination of whichever caller won the singleflight race, and is only used
// to decode into when the snapshot has to be refetched.
func (hc HashCache) snapshotBytes(ctx context.Context, key string, dst interface{}) ([]byte, error) {
	hash := hc.currentHash(ctx, key)

	if hash != "" {
		if s, ok := hc.localSnapshot(key); ok && s.hash == hash {
			return s.data, nil
		}
	}

	// The snapshot is obsolete, or the key has no hash to validate against at
	// all. Drop it before fetching its replacement: each replica invalidates its
	// own local copy on reads.
	hc.localCache.Delete(key)

	if err := hc.Cache.Get(ctx, key, dst); err != nil {
		return nil, err
	}
	// Re-marshal rather than cache dst itself: the bytes are ours, dst belongs to
	// the caller and may be mutated or reused as soon as we return.
	data, err := jsoniter.Marshal(dst)
	if err != nil {
		return nil, fmt.Errorf("HashCache.Get failed to snapshot key=%s for local cache: %w", key, err)
	}
	if hash != "" {
		hc.localCache.Set(key, snapshot{hash: hash, data: data}, 0)
	}
	return data, nil
}

// currentHash returns the hash Redis holds for key, or "" if it is missing or
// unreadable, in which case no local snapshot can be trusted.
func (hc HashCache) currentHash(ctx context.Context, key string) string {
	var hash string
	if err := hc.Cache.Get(ctx, hashKey(key), &hash); err != nil {
		return ""
	}
	return hash
}

func (hc HashCache) localSnapshot(key string) (snapshot, bool) {
	v, ok := hc.localCache.Get(key)
	if !ok {
		return snapshot{}, false
	}
	s, ok := v.(snapshot)
	return s, ok
}

// Delete key from local cache as well as hash entry in the redis
func (hc HashCache) Delete(ctx context.Context, key string) error {
	// No hash key exists for an individual record, so there is nothing local to
	// invalidate and no hash key to delete.
	if !isCollectionKey(key) {
		return hc.Cache.Delete(ctx, key)
	}

	if hc.latestOnly {
		hc.localCache.Delete(key)
	} else {
		var hash string
		if err := hc.Cache.Get(ctx, hashKey(key), &hash); err == nil {
			hc.localCache.Delete(hash)
		}
	}

	if err := hc.Cache.Delete(ctx, hashKey(key)); err != nil {
		return err
	}
	return hc.Cache.Delete(ctx, key)
}
