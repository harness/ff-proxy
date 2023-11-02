package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
	jsoniter "github.com/json-iterator/go"

	"github.com/harness/ff-proxy/v2/domain"
)

// DoFn returns the item to be cached
type DoFn func(item *cache.Item) (interface{}, error)

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
		ttl:         0,
		localCache:  nil,
		marshalFn:   jsoniter.Marshal,
		unmarshalFn: jsoniter.Unmarshal,
	}

	for _, opt := range opts {
		opt(k)
	}

	k.redisClient = rc
	return k
}

// Set sets a key in the cache
func (k *KeyValCache) Set(ctx context.Context, key string, value interface{}) error {
	v, err := k.marshalFn(value)
	if err != nil {
		return fmt.Errorf("%w: KeyValCache.Set failed to marshal value", err)
	}

	if err := k.redisClient.Set(ctx, key, v, k.ttl).Err(); err != nil {
		return fmt.Errorf("%w: KeyValCache.Set failed for key: %q", err, key)
	}
	return nil
}

// Get gets a value from the cache specified by the key
func (k *KeyValCache) Get(ctx context.Context, key string, value interface{}) error {
	b, err := k.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("%w: KeyValCache.Get key %s doesn't exist in cache: %s", domain.ErrCacheNotFound, key, err)
		}
		return fmt.Errorf("%w: KeyValCache.Get failed for key: %q", err, key)
	}

	return k.unmarshalFn(b, value)
}

// Delete can be used to forcefully remove a key from the cache before it's TTL has expired
func (k *KeyValCache) Delete(ctx context.Context, key string) error {
	if err := k.redisClient.Del(ctx, key).Err(); err != nil {
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

// HealthCheck pings the underlying redis cache
func (k *KeyValCache) HealthCheck(ctx context.Context) error {
	return k.redisClient.Ping(ctx).Err()
}

// Scan returns a map of keys that match the pattern
func (k *KeyValCache) Scan(ctx context.Context, key string) (map[string]string, error) {
	scan := make(map[string]string)
	iter := k.redisClient.Scan(ctx, 0, "*"+key+"*", 0).Iterator()
	for iter.Next(ctx) {
		scan[iter.Val()] = ""
	}
	if err := iter.Err(); err != nil {
		return scan, fmt.Errorf("%w: KeyValCache.Scan failed to iterate over keset for key %q", err, key)
	}
	return scan, nil
}
