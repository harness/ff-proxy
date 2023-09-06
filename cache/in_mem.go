package cache

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/harness/ff-proxy/v2/domain"
	jsoniter "github.com/json-iterator/go"
)

// MemCache is an in memory cache that stores a map of keys to a map of fields
// and their values
type MemCache struct {
	*sync.RWMutex
	data map[string][]byte
}

// NewMemCache creates an initialised MemCache
func NewMemCache() MemCache {
	return MemCache{&sync.RWMutex{}, map[string][]byte{}}
}

// Set sets a value in the cache for a given key and field
func (m MemCache) Set(ctx context.Context, key string, value interface{}) error {
	m.Lock()
	defer m.Unlock()

	b, err := jsoniter.Marshal(value)
	if err != nil {
		return err
	}

	m.data[key] = b
	return nil
}

// Get gets the value of a field for a given key
func (m MemCache) Get(ctx context.Context, key string, v interface{}) error {
	m.RLock()
	defer m.RUnlock()

	b, ok := m.data[key]
	if !ok {
		return fmt.Errorf("%w: key %q doesn't exist in memcache", domain.ErrCacheNotFound, key)
	}

	if err := jsoniter.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%v: failed to unmarshal value to %T for key: %q", domain.ErrCacheInternal, v, key)
	}
	return nil
}

// Delete removes all of the fields and their values for a given key
func (m MemCache) Delete(ctx context.Context, key string) error {
	m.Lock()
	defer m.Unlock()

	delete(m.data, key)
	return nil
}

// Keys returns a list of keys that match the pattern
func (m MemCache) Keys(ctx context.Context, key string) ([]string, error) {
	m.Lock()
	defer m.Unlock()

	keys := reflect.ValueOf(m.data).MapKeys()

	results := make([]string, 0, len(keys))
	for _, k := range keys {
		s := k.String()
		if strings.HasPrefix(s, key) {
			results = append(results, s)
		}
	}

	return results, nil
}

// HealthCheck checks cache health
// we don't have any connection to check here so just return no errors
func (m MemCache) HealthCheck(ctx context.Context) error {
	return nil
}
