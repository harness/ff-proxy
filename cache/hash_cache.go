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

// HashCacher ...
type HashCacher struct {
	Cache
	localCache *gocache.Cache
}

// NewHashCacher ...
func NewHashCacher(c Cache, defaultExpiration, cleanupInterval time.Duration) *HashCacher {
	return &HashCacher{
		Cache:      c,
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

	if err := hc.Cache.Set(ctx, latestHashKey, latestHashString); err != nil {
		return latestHashKey, err
	}
	return latestHashKey, nil
}

// Get checks the local cache for the key and returns it if there.
func (hc HashCacher) Get(ctx context.Context, key string, value interface{}) error {
	latestKey := fmt.Sprintf("%s-latest", key)
	var hash string

	//hc.metrics.hashInc(latestKey)
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
	hc.localCache.Set(hash, value, 0)
	return err
}
