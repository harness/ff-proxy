package cache

import (
	"context"
	"encoding"
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"

	"github.com/harness/ff-proxy/domain"
)

// MemCache is an in memory cache that stores a map of keys to a map of fields
// and their values
type MemCache struct {
	*sync.RWMutex
	data map[string]map[string][]byte
}

// NewMemCache creates an initialised MemCache
func NewMemCache() MemCache {
	return MemCache{&sync.RWMutex{}, map[string]map[string][]byte{}}
}

// Set sets a value in the cache for a given key and field
func (m MemCache) Set(ctx context.Context, key string, field string, value encoding.BinaryMarshaler) error {
	m.Lock()
	defer m.Unlock()

	if v, ok := m.data[key]; ok {
		b, err := value.MarshalBinary()
		if err != nil {
			return fmt.Errorf("%w: failed to set value for key: %q, field: %q", domain.ErrCacheInternal, key, field)
		}
		v[field] = b
		return nil
	}

	b, err := value.MarshalBinary()
	if err != nil {
		return err
	}
	m.data[key] = map[string][]byte{
		field: b,
	}
	return nil
}

// GetAll gets all of the fields and their values for a given key
func (m MemCache) GetAll(ctx context.Context, key string) (map[string][]byte, error) {
	m.RLock()
	defer m.RUnlock()

	fields, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("%w: key: %s doesn't exist in memcache", domain.ErrCacheNotFound, key)
	}

	return fields, nil
}

// Get gets the value of a field for a given key
func (m MemCache) Get(ctx context.Context, key string, field string, v encoding.BinaryUnmarshaler) error {
	m.RLock()
	defer m.RUnlock()

	fields, ok := m.data[key]
	if !ok {
		return fmt.Errorf("%w: key %q doesn't exist in memcache", domain.ErrCacheNotFound, key)
	}

	value, ok := fields[field]
	if !ok {
		return fmt.Errorf("%w: field %q doesn't exist in memcache for key: %q", domain.ErrCacheNotFound, field, key)
	}

	if err := v.UnmarshalBinary(value); err != nil {
		return fmt.Errorf("%w: failed to unmarshal value to %T for key: %q, field: %q", v, key, field, domain.ErrCacheInternal)
	}
	return nil
}

// RemoveAll removes all of the fields and their values for a given key
func (m MemCache) RemoveAll(ctx context.Context, key string) {
	m.Lock()
	defer m.Unlock()

	delete(m.data, key)
}

// Remove removes the field for a given key
func (m MemCache) Remove(ctx context.Context, key string, field string) {
	m.Lock()
	defer m.Unlock()
	// get map from key
	fields, ok := m.data[key]
	if !ok {
		log.Debugf("key %s not found in cache", key)
	}

	delete(fields, field)
}