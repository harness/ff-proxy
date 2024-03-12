package cache

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/singleflight"
)

type mCache struct {
	Cache
	set  func(d map[string]interface{}, key string, value interface{}) error
	data map[string]interface{}

	latestKeyHits       int
	fullDocumentKeyHits int
}

func (c *mCache) Set(ctx context.Context, key string, value interface{}) error {
	if c.data == nil {
		c.data = make(map[string]interface{})
	}

	return c.set(c.data, key, value)
}

func (c *mCache) Get(ctx context.Context, key string, value interface{}) error {
	if strings.HasSuffix(key, "-latest") {
		c.latestKeyHits++
	} else {
		c.fullDocumentKeyHits++
	}

	data, ok := c.data[key]
	if !ok {
		return errors.New("not found")
	}

	val := reflect.ValueOf(value)
	respValue := reflect.ValueOf(data)
	if respValue.Kind() == reflect.Ptr {
		val.Elem().Set(respValue.Elem())
	} else {
		val.Elem().Set(respValue)
	}

	return nil
}

type mockLocalCache struct {
	internalCache
	data    map[string]interface{}
	getHits int
}

func (m *mockLocalCache) Set(key string, data interface{}, t time.Duration) {
	m.data[key] = data
}

func (m *mockLocalCache) Get(key string) (interface{}, bool) {
	m.getHits++

	b, ok := m.data[key]
	return b, ok
}

func TestHashCache_Set(t *testing.T) {
	fooSegment := domain.Segment{Identifier: "foo"}
	fooFeature := domain.FeatureFlag{Feature: "foo"}

	type args struct {
		key   string
		value interface{}
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
		"Given I try and set an individual segment in the HashCache": {
			args: args{
				key:   string(domain.NewSegmentKey("123", "foo")),
				value: domain.Segment{Identifier: "foo"},
			},
			mocks: mocks{
				cache: &mCache{set: func(d map[string]interface{}, key string, value interface{}) error {
					d[key] = value
					return nil
				}},
			},
			expected: expected{
				data: map[string]interface{}{
					"env-123-segment-foo": fooSegment,
				},
			},
			shouldErr: false,
		},
		"Given I try and set an individual feature in the HashCache": {
			args: args{
				key:   string(domain.NewFeatureConfigKey("123", "foo")),
				value: domain.FeatureFlag{Feature: "foo"},
			},
			mocks: mocks{
				cache: &mCache{set: func(d map[string]interface{}, key string, value interface{}) error {
					d[key] = value
					return nil
				}},
			},
			expected: expected{
				data: map[string]interface{}{
					"env-123-feature-config-foo": fooFeature,
				},
			},
			shouldErr: false,
		},
		"Given I try and set All Segments in the HashCache": {
			args: args{
				key:   string(domain.NewSegmentsKey("123")),
				value: []domain.Segment{fooSegment},
			},
			mocks: mocks{
				cache: &mCache{set: func(d map[string]interface{}, key string, value interface{}) error {
					d[key] = value
					return nil
				}},
			},
			expected: expected{
				data: map[string]interface{}{
					"env-123-segments":        []domain.Segment{fooSegment},
					"env-123-segments-latest": "90e1f23cceb0f11b9722ad1ed825fd986196e3f9550548b5f446f3b6f0efb534", // Hash of Segments
				},
			},
			shouldErr: false,
		},
		"Given I try and set All Features in the HashCache": {
			args: args{
				key:   string(domain.NewFeatureConfigsKey("123")),
				value: []domain.FeatureFlag{fooFeature},
			},
			mocks: mocks{
				cache: &mCache{set: func(d map[string]interface{}, key string, value interface{}) error {
					d[key] = value
					return nil
				}},
			},
			expected: expected{
				data: map[string]interface{}{
					"env-123-feature-configs":        []domain.FeatureFlag{fooFeature},
					"env-123-feature-configs-latest": "88fb1f47c48205b773c0c1f75cfb79816fb7736b37ceea503784c535aa5701e6", // Hash of Features
				},
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()

			hc := HashCache{
				Cache:        tc.mocks.cache,
				localCache:   nil,
				requestGroup: &singleflight.Group{},
			}

			err := hc.Set(ctx, tc.args.key, tc.args.value)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.data, tc.mocks.cache.data)
		})
	}
}

func TestHashCache_Get(t *testing.T) {
	fooSegment := domain.Segment{Identifier: "foo"}

	type args struct {
		key   string
		value domain.Segment
	}

	type mocks struct {
		cache      *mCache
		localCache *mockLocalCache
	}

	type expected struct {
		value            domain.Segment
		localCacheHits   int
		latestKeyHits    int
		fullDocumentHits int
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given I fetch a key that isn't in the local cache": {
			args: args{
				key:   string(domain.NewSegmentsKey("123")),
				value: fooSegment,
			},
			mocks: mocks{
				cache: &mCache{
					set: func(d map[string]interface{}, key string, value interface{}) error {
						d[key] = value
						return nil
					},
				},
				localCache: &mockLocalCache{
					data: make(map[string]interface{}),
				},
			},
			expected: expected{
				value:            fooSegment,
				localCacheHits:   1,
				latestKeyHits:    1,
				fullDocumentHits: 1,
			},
			shouldErr: false,
		},
		"Given I fetch a key that is in local cache": {
			args: args{
				key:   string(domain.NewSegmentsKey("123")),
				value: fooSegment,
			},
			mocks: mocks{
				cache: &mCache{
					set: func(d map[string]interface{}, key string, value interface{}) error {
						d[key] = value
						return nil
					},
				},
				localCache: &mockLocalCache{
					data: map[string]interface{}{
						// Hash of fooSegment is our key
						"6f20db82fb80d9b6dec1e1ef68cf010f2b7bb6f2963d8cf2d42961eb6acbc5ec": fooSegment,
					},
				},
			},
			expected: expected{
				value:            fooSegment,
				localCacheHits:   1,
				latestKeyHits:    1,
				fullDocumentHits: 0,
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()

			hc := HashCache{
				Cache:        tc.mocks.cache,
				localCache:   tc.mocks.localCache,
				requestGroup: &singleflight.Group{},
			}

			// Set key before we run Get test
			assert.Nil(t, hc.Set(ctx, tc.args.key, tc.args.value))

			var actual domain.Segment

			err := hc.Get(ctx, tc.args.key, &actual)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.localCacheHits, tc.mocks.localCache.getHits)
			assert.Equal(t, tc.expected.latestKeyHits, tc.mocks.cache.latestKeyHits)
			assert.Equal(t, tc.expected.fullDocumentHits, tc.mocks.cache.fullDocumentKeyHits)

			assert.Equal(t, tc.expected.value, actual)
		})
	}
}
