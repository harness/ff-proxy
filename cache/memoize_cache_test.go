package cache

import (
	"crypto/md5"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

type mockInternalCache struct {
	data map[string]interface{}
}

func (m *mockInternalCache) Get(key string) (interface{}, bool) {
	v, ok := m.data[key]
	return v, ok
}

func (m *mockInternalCache) Set(key string, v interface{}, d time.Duration) {
	m.data[key] = v
}

type mockMetrics struct {
	cacheMarshal        int
	cacheUnmarshal      int
	localCacheUnmarshal int
	localCacheHit       int
}

func (m *mockMetrics) cacheMarshalInc() {
	m.cacheMarshal++
}

func (m *mockMetrics) cacheMissInc() {
	m.cacheUnmarshal++
}

func (m *mockMetrics) cacheHitWithUnmarshalInc() {
	m.localCacheUnmarshal++
}

func (m *mockMetrics) cacheHitInc() {
	m.localCacheHit++
}

func TestNewMemoizeMetrics(t *testing.T) {
	// Just testing it doesn't panic when we call MustRegister
	_ = NewMemoizeMetrics("", prometheus.NewRegistry())
}

func TestMemoizeCache_makeMarshalFunc(t *testing.T) {
	mockMetrics := &mockMetrics{}

	c := NewMemoizeCache(nil, 1*time.Minute, 1*time.Minute, 1*time.Minute, mockMetrics)

	mc, ok := c.(memoizeCache)
	assert.True(t, ok)

	marshal := mc.makeMarshalFunc(cache.New(1*time.Minute, 1*time.Minute))

	m := map[string]string{
		"hello": "world",
	}

	b, err := marshal(m)
	assert.Nil(t, err)

	assert.Equal(t, b, mustMarshal(m))
	assert.Equal(t, 1, mockMetrics.cacheMarshal)
}

func TestMemoizeCache_makeUnmarshalFunc(t *testing.T) {
	type mocks struct {
		metrics       *mockMetrics
		internalCache *mockInternalCache
	}

	type cacheData struct {
		value map[string]string
	}

	type results struct {
		cacheUnmarshal      int
		localCacheUnmarshal int
		localCacheHit       int
	}

	testCases := map[string]struct {
		mocks     mocks
		cacheData cacheData
		shouldErr bool

		thing    interface{}
		expected results
	}{
		"Given my internal cache has the thing": {
			mocks: mocks{
				metrics:       &mockMetrics{},
				internalCache: &mockInternalCache{data: make(map[string]interface{})},
			},
			cacheData: cacheData{
				value: map[string]string{
					"hello": "world",
				},
			},
			shouldErr: false,

			thing: map[string]string{
				"hello": "world",
			},

			expected: results{
				cacheUnmarshal:      0,
				localCacheUnmarshal: 0,
				localCacheHit:       1,
			},
		},
		"Given I have an empty internal cache": {
			mocks: mocks{
				metrics:       &mockMetrics{},
				internalCache: &mockInternalCache{data: make(map[string]interface{})},
			},
			cacheData: cacheData{},
			shouldErr: false,

			thing: map[string]string{"thing": "foo"},

			expected: results{
				cacheUnmarshal:      1,
				localCacheUnmarshal: 0,
				localCacheHit:       0,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			c := memoizeCache{
				Cache2:  SetupTestKeyValCache(),
				metrics: tc.mocks.metrics,
			}

			if tc.cacheData.value != nil {
				// Prime the cache
				mFn := c.makeMarshalFunc(tc.mocks.internalCache)
				_, err := mFn(tc.cacheData.value)
				assert.Nil(t, err)

			}

			unmarshal := c.makeUnmarshalFunc(tc.mocks.internalCache)

			actual := map[string]string{}
			err := unmarshal(mustMarshal(tc.thing), &actual)
			assert.Nil(t, err)

			assert.Equal(t, tc.expected.localCacheUnmarshal, tc.mocks.metrics.localCacheUnmarshal)
			assert.Equal(t, tc.expected.localCacheHit, tc.mocks.metrics.localCacheHit)
			assert.Equal(t, tc.expected.cacheUnmarshal, tc.mocks.metrics.cacheUnmarshal)
		})
	}
}

func hash(b []byte) []byte {
	hasher := md5.New()
	hasher.Write(b)
	return hasher.Sum(nil)
}

func mustMarshal(v interface{}) []byte {
	b, err := jsoniter.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
