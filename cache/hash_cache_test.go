package cache

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/harness/ff-proxy/v2/domain"
	gocache "github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/singleflight"
)

type mCache struct {
	Cache
	set  func(d map[string]interface{}, key string, value interface{}) error
	data map[string]interface{}

	latestKeyHits       int
	fullDocumentKeyHits int
}

func newHashCacheTestBackend(t *testing.T) (*KeyValCache, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return NewKeyValCache(client), server
}

func TestHashCache_SnapshotRetention(t *testing.T) {
	for _, mode := range []struct {
		name    string
		options []HashCacheOption
		latest  bool
	}{
		{name: "default"},
		{name: "explicitly disabled", options: []HashCacheOption{WithLatestOnly(false)}},
		{name: "latest only", options: []HashCacheOption{WithLatestOnly(true)}, latest: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			ctx := context.Background()
			backend, server := newHashCacheTestBackend(t)
			writer := NewHashCache(backend, time.Minute, 0, mode.options...)
			// A replica has its own local cache and never receives Set calls.
			replica := NewHashCache(backend, time.Minute, 0, mode.options...)
			const versions = 250
			for version := 0; version < versions; version++ {
				for _, env := range []string{"one", "two"} {
					flagsKey := string(domain.NewFeatureConfigsKey(env))
					segmentsKey := string(domain.NewSegmentsKey(env))
					flags := []domain.FeatureFlag{{Feature: fmt.Sprintf("flag-%s-%d", env, version)}}
					segments := []domain.Segment{{Identifier: fmt.Sprintf("segment-%s-%d", env, version)}}
					require.NoError(t, writer.Set(ctx, flagsKey, flags))
					require.NoError(t, writer.Set(ctx, segmentsKey, segments))
					for _, reader := range []*HashCache{writer, replica} {
						var gotFlags []domain.FeatureFlag
						var gotSegments []domain.Segment
						require.NoError(t, reader.Get(ctx, flagsKey, &gotFlags))
						require.NoError(t, reader.Get(ctx, segmentsKey, &gotSegments))
						require.Equal(t, flags, gotFlags)
						require.Equal(t, segments, gotSegments)
						// Unchanged hashes still avoid fetching the full collection.
						commands := server.CommandCount()
						var hit []domain.FeatureFlag
						require.NoError(t, reader.Get(ctx, flagsKey, &hit))
						require.Equal(t, flags, hit)
						require.Equal(t, commands+1, server.CommandCount())
					}
				}
				if mode.latest {
					require.Equal(t, 4, writer.localCache.(*gocache.Cache).ItemCount())
					require.Equal(t, 4, replica.localCache.(*gocache.Cache).ItemCount())
				}
			}
			if !mode.latest {
				require.Equal(t, versions*4, writer.localCache.(*gocache.Cache).ItemCount())
				require.Equal(t, versions*4, replica.localCache.(*gocache.Cache).ItemCount())
			}
		})
	}
}

func TestHashCache_LatestOnlyInvalidation(t *testing.T) {
	for _, externalDelete := range []bool{false, true} {
		t.Run(fmt.Sprintf("external-delete=%t", externalDelete), func(t *testing.T) {
			ctx := context.Background()
			backend, server := newHashCacheTestBackend(t)
			writer := NewHashCache(backend, time.Minute, 0)
			reader := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true))
			key := string(domain.NewFeatureConfigsKey("one"))
			var got []string
			require.NoError(t, writer.Set(ctx, key, []string{"first"}))
			require.NoError(t, reader.Get(ctx, key, &got))

			deleter := reader
			if externalDelete {
				deleter = writer
			}
			require.NoError(t, deleter.Delete(ctx, key))
			require.ErrorIs(t, reader.Get(ctx, key, &got), domain.ErrCacheNotFound)
			require.Zero(t, reader.localCache.(*gocache.Cache).ItemCount())
			require.NoError(t, writer.Set(ctx, key, []string{"recreated"}))
			require.NoError(t, reader.Get(ctx, key, &got))
			require.Equal(t, []string{"recreated"}, got)

			// A missing hash must bypass and evict the previous local snapshot.
			require.NoError(t, backend.Delete(ctx, key+"-latest"))
			require.NoError(t, backend.Set(ctx, key, []string{"without hash"}))
			require.NoError(t, reader.Get(ctx, key, &got))
			require.Equal(t, []string{"without hash"}, got)
			require.Zero(t, reader.localCache.(*gocache.Cache).ItemCount())

			// A Redis read error must not serve the old local snapshot either.
			require.NoError(t, writer.Set(ctx, key, []string{"before error"}))
			require.NoError(t, reader.Get(ctx, key, &got))
			server.SetError("unavailable")
			require.Error(t, reader.Get(ctx, key, &got))
			require.Zero(t, reader.localCache.(*gocache.Cache).ItemCount())
		})
	}
}

func TestHashCache_LatestOnlyOwnsSnapshot(t *testing.T) {
	ctx := context.Background()
	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true))
	key := string(domain.NewSegmentsKey("one"))
	var got []string
	require.NoError(t, hc.Set(ctx, key, []string{"first"}))
	require.NoError(t, hc.Get(ctx, key, &got))
	first := got
	got = []string{"caller reassigned output"}
	var hit []string
	require.NoError(t, hc.Get(ctx, key, &hit))
	require.Equal(t, []string{"first"}, hit)
	require.NoError(t, hc.Set(ctx, key, []string{"second"}))
	require.NoError(t, hc.Get(ctx, key, &got))
	require.Equal(t, []string{"second"}, got)
	require.Equal(t, []string{"first"}, first)
	// Expiration still forces a refresh even when the hash has not changed.
	entry, ok := hc.localCache.Get(key)
	require.True(t, ok)
	hc.localCache.Set(key, entry, time.Nanosecond)
	time.Sleep(time.Millisecond)
	_, ok = hc.localCache.Get(key)
	require.False(t, ok)
	require.NoError(t, hc.Get(ctx, key, &got))
	require.Equal(t, []string{"second"}, got)
}

func TestHashCache_LatestOnlyConcurrentReads(t *testing.T) {
	ctx := context.Background()
	backend, _ := newHashCacheTestBackend(t)
	writer := NewHashCache(backend, time.Minute, 0)
	reader := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true))
	key := string(domain.NewFeatureConfigsKey("one"))
	require.NoError(t, writer.Set(ctx, key, []int{0}))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var got []int
			for j := 0; j < 50; j++ {
				if assert.NoError(t, reader.Get(ctx, key, &got)) {
					assert.Len(t, got, 1)
				}
			}
		}()
	}
	for version := 1; version <= 50; version++ {
		assert.NoError(t, writer.Set(ctx, key, []int{version}))
	}
	wg.Wait()
	var got []int
	require.NoError(t, reader.Get(ctx, key, &got))
	require.Equal(t, []int{50}, got)
	require.Equal(t, 1, reader.localCache.(*gocache.Cache).ItemCount())
}

func TestHashCache_LatestOnlySetWriteThrough(t *testing.T) {
	ctx := context.Background()
	backend, server := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true))
	key := string(domain.NewFeatureConfigsKey("one"))

	require.NoError(t, hc.Set(ctx, key, []string{"written"}))
	require.Equal(t, 1, hc.localCache.(*gocache.Cache).ItemCount())

	// Local hit after Set: only the -latest Redis key is read (not the full document).
	commands := server.CommandCount()
	var got []string
	require.NoError(t, hc.Get(ctx, key, &got))
	require.Equal(t, []string{"written"}, got)
	require.Equal(t, commands+1, server.CommandCount())
	require.Equal(t, 1, hc.localCache.(*gocache.Cache).ItemCount())

	require.NoError(t, hc.Set(ctx, key, []string{"updated"}))
	require.Equal(t, 1, hc.localCache.(*gocache.Cache).ItemCount())
	require.NoError(t, hc.Get(ctx, key, &got))
	require.Equal(t, []string{"updated"}, got)
}

func TestHashCache_LatestOnlyCallerMutationIsolation(t *testing.T) {
	ctx := context.Background()
	backend, _ := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true))
	key := string(domain.NewSegmentsKey("one"))

	require.NoError(t, hc.Set(ctx, key, []string{"first"}))
	var got []string
	require.NoError(t, hc.Get(ctx, key, &got))
	require.Equal(t, []string{"first"}, got)

	// Mutating the caller's slice must not poison the cached snapshot (Refresher path).
	got[0] = "mutated-by-caller"
	var hit []string
	require.NoError(t, hc.Get(ctx, key, &hit))
	require.Equal(t, []string{"first"}, hit)
}

func TestHashCache_LatestOnlyLeavesIndividualKeysAlone(t *testing.T) {
	ctx := context.Background()
	backend, server := newHashCacheTestBackend(t)
	hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true))

	// Individual flag/segment keys get no hash key, so they are never snapshotted
	// and must keep behaving exactly as they do with the flag off.
	key := string(domain.NewFeatureConfigKey("one", "flag"))
	require.NoError(t, hc.Set(ctx, key, domain.FeatureFlag{Feature: "flag"}))
	require.Zero(t, hc.localCache.(*gocache.Cache).ItemCount())

	commands := server.CommandCount()
	var got domain.FeatureFlag
	require.NoError(t, hc.Get(ctx, key, &got))
	require.Equal(t, "flag", got.Feature)
	require.Zero(t, hc.localCache.(*gocache.Cache).ItemCount())
	// Only the full document is read: no hash key lookup for a non-collection key.
	require.Equal(t, commands+1, server.CommandCount())
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

func (m *mockLocalCache) Delete(key string) {
	delete(m.data, key)
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

func TestHashCache_KeyNames(t *testing.T) {
	t.Parallel()
	key := string(domain.NewFeatureConfigsKey("one"))
	require.Equal(t, key+"-latest", hashKey(key))

	require.True(t, isCollectionKey(key))
	require.True(t, isCollectionKey(string(domain.NewSegmentsKey("one"))))
	require.False(t, isCollectionKey(string(domain.NewFeatureConfigKey("one", "flag"))))
	require.False(t, isCollectionKey(string(domain.NewSegmentKey("one", "seg"))))
}

type badUnmarshal struct{}

func (*badUnmarshal) UnmarshalJSON([]byte) error { return errors.New("boom") }
func (*badUnmarshal) MarshalJSON() ([]byte, error) { return []byte(`{}`), nil }

func TestHashCache_ErrorPaths(t *testing.T) {
	ctx := context.Background()
	key := string(domain.NewFeatureConfigsKey("err"))

	t.Run("Set underlying fails", func(t *testing.T) {
		hc := NewHashCache(&mCache{
			set: func(d map[string]interface{}, k string, v interface{}) error {
				return errors.New("set fail")
			},
		}, time.Minute, 0)
		require.Error(t, hc.Set(ctx, key, []string{"x"}))
	})

	t.Run("Set marshal fails", func(t *testing.T) {
		hc := NewHashCache(&mCache{
			set: func(d map[string]interface{}, k string, v interface{}) error {
				d[k] = v
				return nil
			},
		}, time.Minute, 0)
		require.Error(t, hc.Set(ctx, key, make(chan int)))
	})

	t.Run("Set latest key fails", func(t *testing.T) {
		hc := NewHashCache(&mCache{
			set: func(d map[string]interface{}, k string, v interface{}) error {
				if strings.HasSuffix(k, "-latest") {
					return errors.New("latest fail")
				}
				d[k] = v
				return nil
			},
		}, time.Minute, 0)
		require.Error(t, hc.Set(ctx, key, []string{"x"}))
	})

	t.Run("latestOnly Set write-through caches the bytes it hashed", func(t *testing.T) {
		backend, _ := newHashCacheTestBackend(t)
		hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true))
		require.NoError(t, hc.Set(ctx, key, []string{"x"}))

		s, ok := hc.localSnapshot(key)
		require.True(t, ok)
		require.JSONEq(t, `["x"]`, string(s.data))
		require.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(`["x"]`))), s.hash)
	})

	t.Run("latestOnly Get surfaces a snapshot decode failure", func(t *testing.T) {
		backend, _ := newHashCacheTestBackend(t)
		hc := NewHashCache(backend, time.Minute, 0, WithLatestOnly(true))
		require.NoError(t, hc.Set(ctx, key, []string{"x"}))

		// The snapshot is current, it just cannot be decoded into this caller's type.
		var bad badUnmarshal
		require.Error(t, hc.Get(ctx, key, &bad))
	})

	t.Run("latestOnly rejects non-pointer", func(t *testing.T) {
		hc := NewHashCache(&mCache{data: map[string]interface{}{}}, time.Minute, 0, WithLatestOnly(true))
		var notPtr []string
		require.Error(t, hc.Get(ctx, key, notPtr))
	})

	t.Run("latestOnly rejects nil pointer", func(t *testing.T) {
		hc := NewHashCache(&mCache{data: map[string]interface{}{}}, time.Minute, 0, WithLatestOnly(true))
		var nilPtr *[]string
		require.Error(t, hc.Get(ctx, key, nilPtr))
	})

	t.Run("default get redis miss", func(t *testing.T) {
		hc := NewHashCache(&mCache{data: map[string]interface{}{}}, time.Minute, 0)
		var got []string
		require.Error(t, hc.Get(ctx, key, &got))
	})

	t.Run("Delete latest key fails", func(t *testing.T) {
		hc := NewHashCache(&failDeleteCache{failOn: key + "-latest"}, time.Minute, 0)
		require.Error(t, hc.Delete(ctx, key))
	})
}

type failDeleteCache struct {
	Cache
	failOn string
}

func (f *failDeleteCache) Delete(ctx context.Context, key string) error {
	if key == f.failOn {
		return errors.New("delete fail")
	}
	return nil
}

func (f *failDeleteCache) Get(ctx context.Context, key string, value interface{}) error {
	return errors.New("not found")
}

func (f *failDeleteCache) Set(ctx context.Context, key string, value interface{}) error {
	return nil
}
