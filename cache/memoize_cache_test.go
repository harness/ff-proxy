package cache

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash/adler32"
	"hash/crc32"
	"hash/crc64"
	"hash/fnv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	jsoniter "github.com/json-iterator/go"
	gocache "github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/sha3"
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

	c := NewMemoizeCache(nil, 1*time.Minute, 1*time.Minute, mockMetrics)

	mc, ok := c.(memoizeCache)
	assert.True(t, ok)

	marshal := mc.makeMarshalFunc(gocache.New(1*time.Minute, 1*time.Minute))

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
				Cache:   setupTestKeyValCache(),
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

func mustMarshal(v interface{}) []byte {
	b, err := jsoniter.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
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
		ttl:         0,
		localCache:  nil,
		marshalFn:   jsoniter.Marshal,
		unmarshalFn: jsoniter.Unmarshal,
		redisClient: redisClient,
	}

	return k
}

// create the benchmar test
func generateBytes(length int) []byte {
	// Create a byte slice of the desired length
	bytes := make([]byte, length)
	return bytes
}

// Test data
var testData = generateBytes(1000)

// Benchmark function for MD5
func BenchmarkMD5Hash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		md5.Sum(testData)
	}
}

// Benchmark function for SHA-1
func BenchmarkSHA1Hash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sha1.Sum(testData)
	}
}

// Benchmark function for SHA-256
func BenchmarkSHA256Hash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sha256.Sum256(testData)
	}
}

// Benchmark function for SHA-512
func BenchmarkSHA512Hash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sha512.Sum512(testData)
	}
}

// Benchmark function for SHA-3
func BenchmarkSHA3Hash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sha3.Sum256(testData)
	}
}

// Benchmark function for BLAKE2b
func BenchmarkBLAKE2bHash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		blake2b.Sum256(testData)
	}
}

// Benchmark function for Adler-32
func BenchmarkAdler32Hash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		adler32.Checksum(testData)
	}
}

// Benchmark function for CRC-32
func BenchmarkCRC32Hash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		crc32.ChecksumIEEE(testData)
	}
}

// Benchmark function for CRC-64
func BenchmarkCRC64Hash(b *testing.B) {
	table := crc64.MakeTable(crc64.ISO)
	for i := 0; i < b.N; i++ {
		crc64.Checksum(testData, table)
	}
}

// Benchmark function for FNV-1a
func BenchmarkFNV1aHash(b *testing.B) {
	hasher := fnv.New64a()
	for i := 0; i < b.N; i++ {
		hasher.Reset()
		hasher.Write(testData)
		hasher.Sum64()
	}
}

func BenchmarkAllHashFunctions(b *testing.B) {
	b.Run("MD5Hash", BenchmarkMD5Hash)
	b.Run("SHA1Hash", BenchmarkSHA1Hash)
	b.Run("SHA256Hash", BenchmarkSHA256Hash)
	b.Run("SHA512Hash", BenchmarkSHA512Hash)
	b.Run("SHA3Hash", BenchmarkSHA3Hash)
	b.Run("BLAKE2bHash", BenchmarkBLAKE2bHash)
	b.Run("Adler32Hash", BenchmarkAdler32Hash)
	b.Run("CRC32Hash", BenchmarkCRC32Hash)
	b.Run("CRC64Hash", BenchmarkCRC64Hash)
	b.Run("FNV1aHash", BenchmarkFNV1aHash)
}
