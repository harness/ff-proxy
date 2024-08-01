package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

type mCache struct {
	cache.Cache
	data map[string]interface{}

	set func(m *mCache, key string, value interface{}) error
	get func(key string) (map[string]interface{}, error)
}

func (m *mCache) Set(ctx context.Context, key string, value interface{}) error {
	return m.set(m, key, value)
}

func TestInventoryRepo_Add(t *testing.T) {

	type args struct {
		key   string
		value map[string]string
	}

	type mocks struct {
		cache *mCache
	}

	type expected struct {
		data map[string]interface{}
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given I call set and the cache errors": {
			args: args{
				key:   "123",
				value: map[string]string{"hello": "world"},
			},
			mocks: mocks{
				cache: &mCache{
					data: map[string]interface{}{},
					set: func(m *mCache, key string, value interface{}) error {
						return errors.New("some error")
					},
				},
			},
			shouldErr: true,
			expected: expected{
				data: map[string]interface{}{},
			},
		},
		"Given I call set and the cache doesn't error": {
			args: args{
				key:   "123",
				value: map[string]string{"hello": "world"},
			},
			mocks: mocks{
				cache: &mCache{
					data: map[string]interface{}{},
					set: func(m *mCache, key string, value interface{}) error {
						m.data[key] = value
						return nil
					},
				},
			},
			shouldErr: false,
			expected: expected{
				data: map[string]interface{}{
					"key-123-inventory": map[string]string{"hello": "world"},
				},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()
			ir := NewInventoryRepo(tc.mocks.cache, log.NewNoOpLogger())

			err := ir.Add(ctx, tc.args.key, tc.args.value)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.data, tc.mocks.cache.data)
		})
	}
}

func TestInventoryRepo_Cleanup(t *testing.T) {
	var (
		key123 = "123"
		key456 = "456"

		assets123 = map[string]string{
			"env-123-feature-configs":     "[{}]",
			"env-123-feature-configs-foo": "{}",
		}

		assets456 = map[string]string{
			"env-456-feature-configs":     "[{}]",
			"env-456-feature-configs-bar": "{}",
		}
	)

	mr, err := miniredis.Run()
	assert.Nil(t, err)

	rc := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	type args struct {
		oldKey    string
		oldAssets map[string]string

		newKey    string
		newAssets map[string]string
	}

	type mocks struct {
	}

	type expected struct {
		config map[string]string
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given I cleanup key123": {
			args: args{
				oldKey:    key123,
				oldAssets: assets123,

				newKey:    key456,
				newAssets: assets456,
			},
			shouldErr: false,
			expected: expected{
				config: assets456,
			},
		},
		"Given I cleanup key456": {
			args: args{
				oldKey:    key456,
				oldAssets: assets456,

				newKey:    key123,
				newAssets: assets123,
			},
			shouldErr: false,
			expected: expected{
				config: assets123,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()
			c := cache.NewKeyValCache(rc)
			ir := NewInventoryRepo(c, log.NoOpLogger{})

			// Add both keys to setup test
			assert.Nil(t, ir.Add(ctx, tc.args.oldKey, tc.args.oldAssets))
			assert.Nil(t, ir.Add(ctx, tc.args.newKey, tc.args.newAssets))

			_, err := ir.Cleanup(ctx, tc.args.oldKey, []domain.ProxyConfig{})
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			// Assert that we've removed data for the cleanup key
			cleanupRes, err := ir.Get(ctx, tc.args.oldKey)
			assert.Nil(t, err)
			assert.Equal(t, cleanupRes, map[string]string{})

			// Assert that config for the new key still exists
			newAssets, err := ir.Get(ctx, tc.args.newKey)
			assert.Nil(t, err)
			assert.Equal(t, tc.expected.config, newAssets)
		})
	}
}
