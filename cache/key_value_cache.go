package cache

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
)

var (
	// ErrNotFound is the error returned when a record isn't found
	ErrNotFound = errors.New("NotFound")
)

// CacheDoFn returns the item to be cached
type CacheDoFn func(item *cache.Item) (interface{}, error)

// Options defines optional parameters for configuring a KeyValCache
type Options func(k *KeyValCache)

// WithMarshalFunc lets you set how data is marshaled into the cache
func WithMarshalFunc(marshalFunc cache.MarshalFunc) Options {
	return func(k *KeyValCache) {
		k.marshalFn = marshalFunc
	}
}

// WithUnmarshalFunc lets you set how data is unmarshaled from the cache
func WithUnmarshalFunc(unmarshalFunc cache.UnmarshalFunc) Options {
	return func(k *KeyValCache) {
		k.unmarshalFn = unmarshalFunc
	}
}

// WithTTL sets the TTL for keys in the cache
func WithTTL(ttl time.Duration) Options {
	return func(k *KeyValCache) {
		k.ttl = ttl
	}
}

// WithLocalCache lets you configure the LocalCache e.g.
// NewKeyValCache("localhost:6379", WithLocalCache(cache.NewTinyLFU(5000, 1 * time.Hour))
func WithLocalCache(lc cache.LocalCache) Options {
	return func(k *KeyValCache) {
		k.localCache = lc
	}
}

// KeyValCache is a cache that stores KeyValue pairs
type KeyValCache struct {
	cache       *cache.Cache
	marshalFn   cache.MarshalFunc
	unmarshalFn cache.UnmarshalFunc
	ttl         time.Duration
	localCache  cache.LocalCache
	redisClient redis.UniversalClient
}

// NewKeyValCache instantiates and returns a KeyValCache
func NewKeyValCache(rc redis.UniversalClient, opts ...Options) *KeyValCache {
	k := &KeyValCache{
		// Set sane defaults, these can be overridden using the Option funcs
		ttl:        1 * time.Hour,
		localCache: nil,
	}
	for _, opt := range opts {
		opt(k)
	}

	k.cache = cache.New(&cache.Options{
		Redis:      rc,
		LocalCache: k.localCache,
		Marshal:    k.marshalFn,
		Unmarshal:  k.unmarshalFn,
	})
	k.redisClient = rc

	return k
}

// Set sets a key in the cache
func (k *KeyValCache) Set(ctx context.Context, key string, value interface{}) error {
	err := k.cache.Set(&cache.Item{
		Ctx:            ctx,
		Key:            key,
		Value:          value,
		TTL:            k.ttl,
		Do:             nil,
		SkipLocalCache: true,
	})
	if err != nil {
		return fmt.Errorf("%w: KeyValCache.Set failed for key: %q", err, key)
	}
	return nil
}

// Get gets a value from the cache specified by the key
func (k *KeyValCache) Get(ctx context.Context, key string, value interface{}) error {
	err := k.cache.Get(ctx, key, value)
	if err != nil {
		if err == cache.ErrCacheMiss {
			return fmt.Errorf("%w: KeyValCache.Get key %s doesn't exist in cache: %s", ErrNotFound, key, err)
		}
		return fmt.Errorf("%w: KeyValCache.Get failed for key: %q", err, key)
	}
	return nil
}

// GetOnce gets the value from the cache, if the item does not exist in the cache it executes the doFn, caches the result
// and returns the value. Only one doFn execution is in-flight for a given key at a time so if a duplicate comes in the
// caller waits for the original request to complete and gets the result from the cache
func (k *KeyValCache) GetOnce(ctx context.Context, key string, value interface{}, doFn CacheDoFn) error {
	err := k.cache.Once(&cache.Item{
		Ctx:            ctx,
		Key:            key,
		Value:          value,
		Do:             doFn,
		SkipLocalCache: true,
	})
	if err != nil {
		if err == cache.ErrCacheMiss {
			return fmt.Errorf("%w: key %s doesn't exist in cache: %s", ErrNotFound, key, err)
		}
		return fmt.Errorf("%w: KeyValCache.GetOnce failed for key %q", err, key)
	}
	return nil
}

// Delete can be used to forcefully remove a key from the cache before it's TTL has expired
func (k *KeyValCache) Delete(ctx context.Context, key string) error {
	if err := k.cache.Delete(ctx, key); err != nil {
		return fmt.Errorf("%w: KeyValCache.Delete failed for key: %s", err, key)
	}
	return nil
}

// Keys returns a list of keys that match the pattern
func (k *KeyValCache) Keys(ctx context.Context, key string) ([]string, error) {
	cmd := k.redisClient.Keys(ctx, key)
	if cmd.Err() != nil {
		return nil, fmt.Errorf("%w: KeyValCache.Keys failed for key pattern %q", cmd.Err(), key)
	}
	return cmd.Result()
}

// GetLatest returns the latest of keys that match the pattern
// TODO Investigate using ZSET and ZRevRangeByScore for this
func (k *KeyValCache) GetLatest(ctx context.Context, key string) (string, error) {
	keys, err := k.redisClient.Keys(ctx, fmt.Sprintf("%s:*", key)).Result()
	if err != nil {
		return "", fmt.Errorf("%w: KeyValCache.GetLatest failed for key pattern %q", err, key)
	}

	if len(keys) == 0 {
		return "", fmt.Errorf("KeyValCache.GetLatest no keys found for key pattern: %q", key)
	}

	sort.Slice(keys, func(i, j int) bool {
		iVersion, _ := strconv.Atoi(strings.Split(keys[i], ":")[1])
		jVersion, _ := strconv.Atoi(strings.Split(keys[j], ":")[1])
		return iVersion > jVersion
	})

	latestVersionKey := keys[0]
	return k.redisClient.Get(ctx, latestVersionKey).Result()
}
