package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyValCache_Set(t *testing.T) {
	ctx := context.Background()
	k := setupTestKeyValCache()

	testCases := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "Set a valid key-value pair",
			key:   "testKey",
			value: "testValue",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := k.Set(ctx, tc.key, tc.value)
			require.NoError(t, err)
		})
	}
}

func TestKeyValCache_Get(t *testing.T) {
	ctx := context.Background()
	k := setupTestKeyValCache()

	key := "testKey"
	value := "testValue"

	_ = k.Set(ctx, key, value)

	testCases := []struct {
		name      string
		key       string
		wantValue string
		wantErr   error
	}{
		{
			name:      "Get an existing key",
			key:       key,
			wantValue: value,
			wantErr:   nil,
		},
		{
			name:      "Get a non-existing key",
			key:       "nonexistentKey",
			wantValue: "",
			wantErr:   ErrNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			err := k.Get(ctx, tc.key, &result)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantValue, result)
			}
		})
	}
}

func TestKeyValCache_Delete(t *testing.T) {
	ctx := context.Background()
	k := setupTestKeyValCache()

	key := "testKey"
	value := "testValue"

	_ = k.Set(ctx, key, value)

	testCases := []struct {
		name    string
		key     string
		wantErr error
	}{
		{
			name:    "Delete an existing key",
			key:     key,
			wantErr: nil,
		},
		{
			name:    "Delete a non-existing key",
			key:     "nonexistentKey",
			wantErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := k.Delete(ctx, tc.key)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestKeyValCache_Keys(t *testing.T) {
	ctx := context.Background()
	k := setupTestKeyValCache()

	keys := []string{"key1", "key2", "key3"}

	for _, key := range keys {
		_ = k.Set(ctx, key, "value")
	}

	testCases := []struct {
		name     string
		pattern  string
		wantKeys []string
		wantErr  error
	}{
		{
			name:     "Get all keys",
			pattern:  "*",
			wantKeys: keys,
			wantErr:  nil,
		},
		{
			name:     "Get keys with a specific pattern",
			pattern:  "key[12]",
			wantKeys: []string{"key1", "key2"},
			wantErr:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := k.Keys(ctx, tc.pattern)

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tc.wantErr))
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, tc.wantKeys, result)
			}
		})
	}
}

func TestKeyValCache_GetLatest(t *testing.T) {
	ctx := context.Background()
	k := setupTestKeyValCache()

	versions := []string{"1", "2", "3", "4", "5"}

	testCases := []struct {
		name      string
		key       string
		wantValue string
		wantErr   error
		hasKeys   bool
	}{
		{
			name:      "Get latest version",
			key:       "mykey",
			wantValue: "value:5",
			wantErr:   nil,
			hasKeys:   true,
		},
		{
			name:      "Errors on no keys",
			key:       "nopattern",
			wantValue: "",
			wantErr:   errors.New("KeyValCache.GetLatest no keys found for key pattern: \"nopattern\""),
			hasKeys:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hasKeys {
				for _, version := range versions {
					_ = k.Set(ctx, fmt.Sprintf("mykey:%s", version), fmt.Sprintf("value:%s", version))
				}
			}
			result, err := k.GetLatest(ctx, tc.key)
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantValue, result)
			}
		})
	}
}

func setupTestKeyValCache() *KeyValCache {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	k := &KeyValCache{
		ttl:        1 * time.Hour,
		localCache: nil,
	}

	k.cache = cache.New(&cache.Options{
		Redis:      redisClient,
		LocalCache: k.localCache,
		Marshal:    k.marshalFn,
		Unmarshal:  k.unmarshalFn,
	})
	k.redisClient = redisClient

	return k
}
