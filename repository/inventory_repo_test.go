package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
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
		value map[string]int64
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
				value: map[string]int64{"hello": 1},
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
				value: map[string]int64{"hello": 1},
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
					"key-123-inventory": map[string]int64{"hello": 1},
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

		assetsc22b78a0Map = map[string]int64{
			"env-c22b78a0-4bbe-46dc-bc12-a0206c0d4ad7-feature-config-flagOne": 1,
			"env-c22b78a0-4bbe-46dc-bc12-a0206c0d4ad7-feature-configs":        0,
			"env-c22b78a0-4bbe-46dc-bc12-a0206c0d4ad7-segments":               0,
			"env-c22b78a0-4bbe-46dc-bc12-a0206c0d4ad7-segment-segmentOne":     1,
			"env-c22b78a0-4bbe-46dc-bc12-a0206c0d4ad7-api-configs":            0,
			"auth-key-123": 0,
		}

		assetsc22b78a0 = []domain.ProxyConfig{
			{
				Environments: []domain.Environments{
					{
						ID:      uuid.MustParse("c22b78a0-4bbe-46dc-bc12-a0206c0d4ad7"),
						APIKeys: []string{"123"},
						FeatureConfigs: []domain.FeatureFlag{
							{
								Feature: "flagOne",
								Version: int64Ptr(1),
							},
						},
						Segments: []domain.Segment{
							{
								Name:       "segmentOne",
								Identifier: "segmentOne",
								Version:    int64Ptr(1),
							},
						},
					},
				},
			},
		}

		assetsd5c39e52Map = map[string]int64{
			"auth-key-456": 0,
			"env-d5c39e52-0f94-4a4b-a053-9ad842ffd692-feature-config-flagTwo": 2,
			"env-d5c39e52-0f94-4a4b-a053-9ad842ffd692-feature-configs":        0,
			"env-d5c39e52-0f94-4a4b-a053-9ad842ffd692-segments":               0,
			"env-d5c39e52-0f94-4a4b-a053-9ad842ffd692-segment-segmentTwo":     2,
			"env-d5c39e52-0f94-4a4b-a053-9ad842ffd692-api-configs":            0,
		}

		assetsd5c39e52 = []domain.ProxyConfig{
			{
				Environments: []domain.Environments{
					{
						ID:      uuid.MustParse("d5c39e52-0f94-4a4b-a053-9ad842ffd692"),
						APIKeys: []string{"456"},
						FeatureConfigs: []domain.FeatureFlag{
							{
								Feature: "flagTwo",
								Version: int64Ptr(2),
							},
						},
						Segments: []domain.Segment{
							{
								Name:       "segmentTwo",
								Identifier: "segmentTwo",
								Version:    int64Ptr(2),
							},
						},
					},
				},
			},
		}
	)

	mr, err := miniredis.Run()
	assert.Nil(t, err)

	rc := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	type args struct {
		oldKey    string
		oldAssets map[string]int64

		newKey    string
		newAssets []domain.ProxyConfig
	}

	type mocks struct {
	}

	type expected struct {
		config map[string]int64
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
				oldAssets: assetsc22b78a0Map,

				newKey:    key456,
				newAssets: assetsd5c39e52,
			},
			shouldErr: false,
			expected: expected{
				config: assetsd5c39e52Map,
			},
		},
		"Given I cleanup key456": {
			args: args{
				oldKey:    key456,
				oldAssets: assetsd5c39e52Map,

				newKey:    key123,
				newAssets: assetsc22b78a0,
			},
			shouldErr: false,
			expected: expected{
				config: assetsc22b78a0Map,
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

			_, err := ir.Cleanup(ctx, tc.args.newKey, tc.args.newAssets)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			var nilMap map[string]int64

			// Assert that we've removed data for the cleanup key
			cleanupRes, err := ir.Get(ctx, tc.args.oldKey)
			assert.Nil(t, err)
			assert.Equal(t, cleanupRes, nilMap)

			// Assert that config for the new key still exists
			newAssets, err := ir.Get(ctx, tc.args.newKey)
			assert.Nil(t, err)
			assert.Equal(t, tc.expected.config, newAssets)
		})
	}
}

func TestInventoryRepo_BuildNotificatons(t *testing.T) {

	type args struct {
		assets domain.Assets
	}

	type expected struct {
		notifications []domain.SSEMessage
	}

	testCases := map[string]struct {
		args     args
		expected expected
	}{
		"Given I have assets with no underscores": {
			args: args{
				assets: domain.Assets{
					Deleted: map[string]int64{
						"env-1234-feature-config-foobar": 1,
					},
					Created: map[string]int64{
						"env-1234-feature-config-helloworld": 2,
					},
					Patched: map[string]int64{
						"env-1234-segment-foobar": 3,
					},
				},
			},
			expected: expected{
				notifications: []domain.SSEMessage{
					{
						Event:       "delete",
						Domain:      "flag",
						Identifier:  "foobar",
						Version:     1,
						Environment: "1234",
					},
					{
						Event:       "create",
						Domain:      "flag",
						Identifier:  "helloworld",
						Version:     2,
						Environment: "1234",
					},
					{
						Event:       "patch",
						Domain:      "target-segment",
						Identifier:  "foobar",
						Version:     3,
						Environment: "1234",
					},
				},
			},
		},
		"Given I have assets with underscores": {
			args: args{
				assets: domain.Assets{
					Deleted: map[string]int64{
						"env-1234-feature-config-PIE_ENABLE_THIS_THING": 1,
					},
					Created: map[string]int64{
						"env-1234-feature-config-_CDS__ENABLED___FLAG": 2,
					},
					Patched: map[string]int64{
						"env-1234-segment-_SOME_SPECIAL_SEGMENT__": 3,
					},
				},
			},
			expected: expected{
				notifications: []domain.SSEMessage{
					{
						Event:       "delete",
						Domain:      "flag",
						Identifier:  "PIE_ENABLE_THIS_THING",
						Version:     1,
						Environment: "1234",
					},
					{
						Event:       "create",
						Domain:      "flag",
						Identifier:  "_CDS__ENABLED___FLAG",
						Version:     2,
						Environment: "1234",
					},
					{
						Event:       "patch",
						Domain:      "target-segment",
						Identifier:  "_SOME_SPECIAL_SEGMENT__",
						Version:     3,
						Environment: "1234",
					},
				},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			i := InventoryRepo{}
			actual := i.BuildNotifications(tc.args.assets)
			assert.Equal(t, tc.expected.notifications, actual)
		})
	}
}
