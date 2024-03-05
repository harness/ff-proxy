package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"time"

	jsoniter "github.com/json-iterator/go"
	gocache "github.com/patrickmn/go-cache"
)

// local interface for redisCache...
type HashCache interface {
	Set(ctx context.Context, key string, value interface{}) error
	// Get gets the value of a field for a given key
	Get(ctx context.Context, key string, value interface{}) error
	// Delete removes a key from the cache
	Delete(ctx context.Context, key string) error
	// GetRawBytes gets a value from the cache specified by the key and return raw bytes
	GetRawBytes(ctx context.Context, key string) ([]byte, error)
}

// HashCacher ...
type HashCacher struct {
	HashCache
	localCache *gocache.Cache
}

// NewHashRepo ...
func NewHashRepo(c HashCache, defaultExpiration, cleanupInterval time.Duration) *HashCacher {
	return &HashCacher{
		HashCache:  c,
		localCache: gocache.New(defaultExpiration, cleanupInterval),
	}
}

// AddHashKey adds hash key entry for the given key
func (hc HashCacher) AddHashKey(ctx context.Context, key string, value interface{}) (string, error) {
	latestHashKey := string(key) + "-latest"
	v, err := jsoniter.Marshal(value)
	if err != nil {
		return latestHashKey, fmt.Errorf("unable to marshall config %s %v", latestHashKey, err)
	}
	latestHash := sha256.Sum256(v)
	latestHashString := fmt.Sprintf("%x", latestHash)

	if err := hc.HashCache.Set(ctx, latestHashKey, latestHashString); err != nil {
		return latestHashKey, err
	}
	return latestHashKey, nil
}

// Get checks the local cache for the key and returns it if there.
func (hc HashCacher) Get(ctx context.Context, key string, value interface{}) error {
	latestKey := fmt.Sprintf("%s-latest", key)
	var hash string

	hash, err := hc.GetHash(ctx, latestKey)
	// m.metrics.hashInc(latestKey) todo
	if err == nil {
		data, ok := hc.localCache.Get(hash)
		if ok {
			// this is assigning value of the data to the value interface.
			val := reflect.ValueOf(value)
			respValue := reflect.ValueOf(data)
			if respValue.Kind() == reflect.Ptr {
				val.Elem().Set(respValue.Elem())
			} else {
				val.Elem().Set(respValue)
			}
			return nil
		}
	}
	// fetch from redis
	err = hc.HashCache.Get(ctx, key, value)
	if err != nil {
		return err
	}
	// set the value in local
	hc.localCache.Set(hash, value, 0)
	return err
}

func (hc HashCacher) GetHash(ctx context.Context, key string) (string, error) {
	bytes, err := hc.GetRawBytes(ctx, key)
	if err != nil {
		return "", err
	}
	return string(bytes[:]), nil
}
