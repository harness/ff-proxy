package cache

import (
	"context"
	"encoding"
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/harness/ff-proxy/domain"
)

// RedisCache provides a redis implementation of our cache.Cache interface
type RedisCache struct {
	client redis.UniversalClient
}

// NewRedisCache creates an initialised RedisCache
func NewRedisCache(client redis.UniversalClient) *RedisCache {
	return &RedisCache{client: client}
}

// Set sets a value in the cache for a given key and field
func (r *RedisCache) Set(ctx context.Context, key string, field string, value encoding.BinaryMarshaler) error {
	if err := r.client.HSet(ctx, key, field, value).Err(); err != nil {
		return err
	}
	return nil
}

// SetByte sets a value in the cache for a given key and field
func (r *RedisCache) SetByte(ctx context.Context, key string, field string, value []byte) error {
	if err := r.client.HSet(ctx, key, field, value).Err(); err != nil {
		return err
	}
	return nil
}

// SetKV sets a key and value
func (r *RedisCache) SetKV(ctx context.Context, key string, value string) error {
	return r.client.Set(ctx, key, value, 0).Err()
}

// GetAll gets all of the fields and their values for a given key
func (r *RedisCache) GetAll(ctx context.Context, key string) (map[string][]byte, error) {
	m, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("%w: key %s doesn't exist in redis", domain.ErrCacheNotFound, key)
		}
		return nil, err
	}

	result := map[string][]byte{}
	for k, v := range m {
		result[k] = []byte(v)
	}
	return result, nil
}

// Get gets the value of a field for a given key
func (r *RedisCache) Get(ctx context.Context, key string, field string, v encoding.BinaryUnmarshaler) error {
	b, err := r.client.HGet(ctx, key, field).Bytes()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("%w: field %s doesn't exist in redis for key: %s", domain.ErrCacheNotFound, field, key)
		}
		return err
	}

	if err := v.UnmarshalBinary(b); err != nil {
		return fmt.Errorf("%v: failed to unmarshal value to %T for key: %q, field: %q", v, key, field, domain.ErrCacheInternal)
	}
	return nil
}

// GetByte gets the value of a field for a given key
func (r *RedisCache) GetByte(ctx context.Context, key string, field string) ([]byte, error) {
	b, err := r.client.HGet(ctx, key, field).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("%w: field %s doesn't exist in redis for key: %s", domain.ErrCacheNotFound, field, key)
		}
		return nil, err
	}
	return b, nil
}

// GetKV gets the value for a given key
func (r *RedisCache) GetKV(ctx context.Context, key string) (string, error) {
	res, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
	}
	return res, nil
}

// RemoveAll removes all of the fields and their values for a given key
func (r *RedisCache) RemoveAll(ctx context.Context, key string) {
	r.client.Del(ctx, key)
}

// Remove removes the field for a given key
func (r *RedisCache) Remove(ctx context.Context, key string, field string) {
	r.client.HDel(ctx, key, field)
}

// HealthCheck checks cache health
func (r *RedisCache) HealthCheck(ctx context.Context) error {
	res := r.client.Ping(ctx)
	if res.Err() != nil {
		return fmt.Errorf("redis failed to respond err: %s", res.Err())
	}
	return nil
}
