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
)

// HashCache ...
type HashCache struct {
	Cache
	localCache internalCache
}

// NewHashCache ...
func NewHashCache(c Cache, defaultExpiration, cleanupInterval time.Duration) *HashCache {
	return &HashCache{
		Cache:      c,
		localCache: gocache.New(defaultExpiration, cleanupInterval),
	}
}

// Set adds hash key entry for the given key
func (hc HashCache) Set(ctx context.Context, key string, value interface{}) error {
	// First set the key, value that we've been given
	if err := hc.Cache.Set(ctx, key, value); err != nil {
		return fmt.Errorf("HashCache.Set failed to set key=%s in cache: %s", key, err)
	}

	// If the key isn't a segments or feature-configs key then we're done.
	// If it is then we'll want to carry on and set a latest hash for these keys
	if !strings.HasSuffix(key, "segments") && !strings.HasSuffix(key, "feature-configs") {
		return nil
	}

	latestKey := fmt.Sprintf("%s-latest", key)
	v, err := jsoniter.Marshal(value)
	if err != nil {
		return fmt.Errorf("unable to marshall config %s %v", latestKey, err)
	}
	latestHash := sha256.Sum256(v)
	latestHashString := fmt.Sprintf("%x", latestHash)

	return hc.Cache.Set(ctx, latestKey, latestHashString)
}

// Get checks the local cache for the key and returns it if there.
func (hc HashCache) Get(ctx context.Context, key string, value interface{}) error {
	latestKey := fmt.Sprintf("%s-latest", key)
	var hash string
	err := hc.Cache.Get(ctx, latestKey, &hash)
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
	err = hc.Cache.Get(ctx, key, value)
	if err != nil {
		return err
	}
	// set the value in local
	if hash != "" {
		hc.localCache.Set(hash, value, 0)
	}
	return err
}

// Delete key from local cache as well as hash entry in the redis
func (hc HashCache) Delete(ctx context.Context, key string) error {

	latestKey := fmt.Sprintf("%s-latest", key)
	var hash string
	err := hc.Cache.Get(ctx, latestKey, &hash)
	if err == nil {
		//delete the latest hash entry in redis
		hc.localCache.Delete(hash)
	}
	err = hc.Cache.Delete(ctx, latestKey)
	if err != nil {
		return err
	}
	return hc.Cache.Delete(ctx, key)
}
