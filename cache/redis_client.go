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
		return fmt.Errorf("%w: failed to unmarshal value to %T for key: %q, field: %q", v, key, field, domain.ErrCacheInternal)
	}
	return nil
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
		return fmt.Errorf("redis failed to respond")
	}
	return nil
}

// Pub publishes the passed values to a topic. If the topic doesn't exist Pub
// will create it as well as publishing the values to it.
func (r *RedisCache) Pub(ctx context.Context, topic string, values map[string]interface{}) error {
	err := r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: fmt.Sprintf("stream-%s", topic),
		ID:     "*",
		Values: values,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to publish event to redis stream %q: %s", topic, err)
	}
	return nil
}

// Sub subscribes to a topic and continually listens for new messages and as new
// messages come in it passes them to the callback. Sub is a blocking function
// and will only exit if there is an error receiving on the redis stream or if
// the context is canceled.
func (r *RedisCache) Sub(ctx context.Context, topic string, onReceive func(values map[string]interface{})) error {
	stream := fmt.Sprintf("stream-%s", topic)
	id := "$"

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			xstreams, err := r.client.XRead(ctx, &redis.XReadArgs{
				Streams: []string{stream, id},
				Block:   0,
			}).Result()
			if err != nil {
				return err
			}

			for _, stream := range xstreams {
				for _, msg := range stream.Messages {
					id = msg.ID
					onReceive(msg.Values)
				}
			}
		}
	}
}
