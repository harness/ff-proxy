package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/harness/ff-golang-server-sdk/rest"

	"github.com/harness/ff-golang-server-sdk/dto"
	"github.com/harness/ff-proxy/log"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func int64Ptr(i int64) *int64 { return &i }

func memCachePtr(m MemCache) *MemCache { return &m }

var (
	evaluationSegmentFoo = rest.Segment{
		Identifier:  "foo",
		Name:        "fooSegment",
		CreatedAt:   int64Ptr(123),
		ModifiedAt:  int64Ptr(456),
		Environment: strPtr("env"),
		Excluded:    &[]rest.Target{{Identifier: "exlc1"}, {Identifier: "exlc2"}},
		Included:    &[]rest.Target{{Identifier: "incl1"}, {Identifier: "incl2"}},
		Rules:       &[]rest.Clause{{Attribute: "attr", Id: "id", Negate: false, Op: "contains", Values: []string{"val1", "val2"}}},
		Tags:        &[]rest.Tag{{Name: "tagName", Value: strPtr("tagValue")}},
		Version:     int64Ptr(2),
	}

	segmentFooKey = dto.Key{
		Type: dto.KeySegment,
		Name: evaluationSegmentFoo.Identifier,
	}

	evaluationFeatureBar = rest.FeatureConfig{
		DefaultServe: rest.Serve{
			Distribution: &rest.Distribution{
				BucketBy:   "bucketfield",
				Variations: []rest.WeightedVariation{{Variation: "var1", Weight: 30}, {Variation: "var2", Weight: 70}},
			},
			Variation: strPtr("var2"),
		},
		Environment:   "env",
		Feature:       "bar",
		Kind:          "bool",
		OffVariation:  "false",
		Prerequisites: &[]rest.Prerequisite{{Feature: "feat1", Variations: []string{"true"}}},
		Project:       "proj",
		Rules: &[]rest.ServingRule{
			{
				Clauses:  []rest.Clause{{Attribute: "attr", Id: "id", Negate: false, Op: "contains", Values: []string{"val1", "val2"}}},
				Priority: 1,
				RuleId:   "ID",
				Serve: rest.Serve{
					Distribution: nil,
					Variation:    strPtr("str"),
				},
			},
		},
		State: "on",
		VariationToTargetMap: &[]rest.VariationMap{
			{
				TargetSegments: &[]string{"segment1", "segment2", "segment3"},
				Targets:        &[]rest.TargetMap{{strPtr("target1"), "target1"}, {strPtr("target2"), "target2"}, {strPtr("target3"), "target3"}},
				Variation:      "var",
			},
		},
		Variations: []rest.Variation{{Description: strPtr("desc"), Identifier: "id", Name: strPtr("name"), Value: "val"}},
	}

	featureBarKey = dto.Key{
		Type: dto.KeyFeature,
		Name: evaluationSegmentFoo.Identifier,
	}
)

func TestEmptyCache(t *testing.T) {
	memCache := NewMemCache()
	wrapper := NewWrapper(&memCache, "testEnv", log.NoOpLogger{})

	t.Run("Empty cache should have len 0", func(t *testing.T) {
		assert.Equal(t, 0, wrapper.Len())
	})

	t.Run("Empty cache should have no keys", func(t *testing.T) {
		assert.Equal(t, []interface{}(nil), wrapper.Keys())
	})

	t.Run("Empty cache should have default updated time", func(t *testing.T) {
		assert.Equal(t, time.Time{}, wrapper.Updated())
	})

	t.Run("Empty cache get should fail", func(t *testing.T) {
		value, ok := wrapper.Get(segmentFooKey)
		assert.Equal(t, nil, value)
		assert.False(t, ok)
	})

	t.Run("Empty cache contains should fail", func(t *testing.T) {
		ok := wrapper.Contains(segmentFooKey)
		assert.False(t, ok)
	})

	t.Run("Empty cache remove should succeed", func(t *testing.T) {
		assert.False(t, wrapper.Remove(segmentFooKey))
	})
}

func TestSet(t *testing.T) {
	type value struct {
		key   interface{}
		value interface{}
	}
	testCases := map[string]struct {
		values       []value
		expectedLen  int
		expectedKeys []interface{}
	}{
		"segment": {
			values: []value{
				{
					key:   segmentFooKey,
					value: evaluationSegmentFoo,
				},
			},
			expectedLen:  1,
			expectedKeys: []interface{}{segmentFooKey},
		},
		"feature": {
			values: []value{
				{
					key:   featureBarKey,
					value: evaluationFeatureBar,
				},
			},
			expectedLen:  1,
			expectedKeys: []interface{}{featureBarKey},
		},
		"feature and segment": {
			values: []value{
				{
					key:   segmentFooKey,
					value: evaluationSegmentFoo,
				},
				{
					key:   featureBarKey,
					value: evaluationFeatureBar,
				},
			},
			expectedLen:  2,
			expectedKeys: []interface{}{segmentFooKey, featureBarKey},
		},
	}

	for desc, tc := range testCases {
		t.Run(fmt.Sprintf("Set %s", desc), func(t *testing.T) {
			cache := NewWrapper(memCachePtr(NewMemCache()), "env", log.NoOpLogger{})
			// add value(s)
			for _, val := range tc.values {
				cache.Set(val.key, val.value)
			}

			// check len, keys, contains, get
			assert.Equal(t, tc.expectedLen, cache.Len())
			assert.Equal(t, tc.expectedKeys, cache.Keys())

			// check each value contains and get
			for _, val := range tc.values {
				assert.True(t, cache.Contains(val.key))
				value, ok := cache.Get(val.key)
				assert.True(t, ok)
				assert.Equal(t, val.value, value)
			}
		})

		t.Run(fmt.Sprintf("Remove %s", desc), func(t *testing.T) {
			cache := NewWrapper(memCachePtr(NewMemCache()), "env", log.NoOpLogger{})
			// add value(s)
			for _, val := range tc.values {
				cache.Set(val.key, val.value)
			}
			assert.Equal(t, tc.expectedLen, cache.Len())

			// remove value(s)
			for _, val := range tc.values {
				assert.True(t, cache.Remove(val.key))
			}

			// check len and keys
			assert.Equal(t, 0, cache.Len())
			assert.Equal(t, []interface{}(nil), cache.Keys())

			// check each value contains and get
			for _, val := range tc.values {
				assert.False(t, cache.Contains(val.key))
				value, ok := cache.Get(val.key)
				assert.False(t, ok)
				assert.Equal(t, nil, value)
			}
		})

		t.Run(fmt.Sprintf("Purge %s", desc), func(t *testing.T) {
			cache := NewWrapper(memCachePtr(NewMemCache()), "env", log.NoOpLogger{})
			// add value(s)
			for _, val := range tc.values {
				cache.Set(val.key, val.value)
			}
			assert.Equal(t, tc.expectedLen, cache.Len())

			// remove value(s)
			cache.Purge()

			// check len and keys
			assert.Equal(t, 0, cache.Len())
			assert.Equal(t, []interface{}(nil), cache.Keys())

			// check each value contains and get
			for _, val := range tc.values {
				assert.False(t, cache.Contains(val.key))
				value, ok := cache.Get(val.key)
				assert.False(t, ok)
				assert.Equal(t, nil, value)
			}
		})
	}
}

func TestInvalidInputs(t *testing.T) {
	memCache := NewMemCache()
	wrapper := NewWrapper(&memCache, "testEnv", log.NoOpLogger{})

	t.Run("Set with invalid key", func(t *testing.T) {
		wrapper.Set("invalidKey", evaluationSegmentFoo)
		assert.Equal(t, 0, wrapper.Len())
	})

	t.Run("Set segment invalid input", func(t *testing.T) {
		wrapper.Set(segmentFooKey, "invalidSegment")
		assert.Equal(t, 0, wrapper.Len())
	})

	t.Run("Set feature invalid input", func(t *testing.T) {
		wrapper.Set(featureBarKey, "invalidFeature")
		assert.Equal(t, 0, wrapper.Len())
	})

	t.Run("Set invalid key type", func(t *testing.T) {
		wrapper.Set(dto.Key{Name: "test", Type: "invalidType"}, "invalidFeature")
		assert.Equal(t, 0, wrapper.Len())
	})

	t.Run("Resize not implemented", func(t *testing.T) {
		evicted := wrapper.Resize(100)
		assert.Equal(t, 0, evicted)
	})
}
