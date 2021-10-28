package cache

import (
	"context"
	"encoding"
	"errors"
	"sync"

	"github.com/harness/ff-proxy/domain"
)

// MemCache is an in memory cache that stores a map of keys to a map of fields
// and their values
type MemCache struct {
	*sync.RWMutex
	data map[string]map[string][]byte
}

// NewMemCache createa an initialised MemCache
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
			return domain.NewCacheInternalErr(domain.CacheOpSet, key, field, err)
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

// GetAll gets all of the fiels and their values for a given key
func (m MemCache) GetAll(ctx context.Context, key string) (map[string][]byte, error) {
	m.Lock()
	defer m.Unlock()

	fields, ok := m.data[key]
	if !ok {
		return nil, domain.NewCacheNotFoundErr(domain.CacheOpGetAll, key, "all", errors.New("key doesn't exist in memCache"))
	}

	return fields, nil
}

// Get gets the value of a field for a given key
func (m MemCache) Get(ctx context.Context, key string, field string, v encoding.BinaryUnmarshaler) error {
	m.Lock()
	defer m.Unlock()

	fields, ok := m.data[key]
	if !ok {
		return domain.NewCacheNotFoundErr(domain.CacheOpGet, key, field, errors.New("key doesn't exist in memCache"))
	}

	value, ok := fields[field]
	if !ok {
		return domain.NewCacheNotFoundErr(domain.CacheOpGet, key, field, errors.New("field doesnt exist in memCahe"))
	}

	if err := v.UnmarshalBinary(value); err != nil {
		return domain.NewCacheInternalErr(domain.CacheOpGet, key, field, err)
	}
	return nil
}
