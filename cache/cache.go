package cache

import "context"

// Cache is the interface for a key value cache
type Cache interface {
	// Set sets a value in the cache for a given key and field
	Set(ctx context.Context, key string, value interface{}) error

	// Get gets the value of a field for a given key
	Get(ctx context.Context, key string, value interface{}) error

	// GetRawBytes gets a value from the cache specified by the key and return raw bytes
	GetRawBytes(ctx context.Context, key string) ([]byte, error)

	// Delete removes a key from the cache
	Delete(ctx context.Context, key string) error

	// Keys returns a list of keys that match the pattern
	Keys(ctx context.Context, key string) ([]string, error)

	// HealthCheck checks cache health
	HealthCheck(ctx context.Context) error

	// Scan all the keys for given key
	Scan(ctx context.Context, key string) (map[string]string, error)
}
