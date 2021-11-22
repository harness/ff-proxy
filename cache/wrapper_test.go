package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/harness/ff-golang-server-sdk/dto"
	"github.com/harness/ff-golang-server-sdk/evaluation"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func int64Ptr(i int64) *int64 { return &i }

func memCachePtr(m MemCache) *MemCache { return &m }

var (
	evaluationSegmentFoo = evaluation.Segment{
		Identifier:  "foo",
		Name:        "fooSegment",
		CreatedAt:   int64Ptr(123),
		ModifiedAt:  int64Ptr(456),
		Environment: strPtr("env"),
		Excluded:    []string{"ecl1", "exlc2"},
		Included:    []string{"incl1", "incl2"},
		Rules:       []evaluation.Clause{{Attribute: "attr", ID: "id", Negate: false, Op: "contains", Value: []string{"val1", "val2"}}},
		Tags:        []evaluation.Tag{{Name: "tagName", Value: strPtr("tagValue")}},
		Version:     2,
	}

	segmentFooKey = dto.Key{
		Type: dto.KeySegment,
		Name: evaluationSegmentFoo.Identifier,
	}

	evaluationFeatureBar = evaluation.FeatureConfig{
		DefaultServe: evaluation.Serve{
			Distribution: &evaluation.Distribution{
				BucketBy:   "bucketfield",
				Variations: []evaluation.WeightedVariation{{Variation: "var1", Weight: 30}, {Variation: "var2", Weight: 70}},
			},
			Variation: strPtr("var2"),
		},
		Environment:   "env",
		Feature:       "bar",
		Kind:          "bool",
		OffVariation:  "false",
		Prerequisites: []evaluation.Prerequisite{{Feature: "feat1", Variations: []string{"true"}}},
		Project:       "proj",
		Rules: []evaluation.ServingRule{
			{
				Clauses:  []evaluation.Clause{{Attribute: "attr", ID: "id", Negate: false, Op: "contains", Value: []string{"val1", "val2"}}},
				Priority: 1,
				RuleID:   "ID",
				Serve: evaluation.Serve{
					Distribution: nil,
					Variation:    strPtr("str"),
				},
			},
		},
		State: "on",
		VariationToTargetMap: []evaluation.VariationMap{
			{
				TargetSegments: []string{"segment1", "segment2", "segment3"},
				Targets:        []string{"target1", "target2", "target3"},
				Variation:      "var",
			},
		},
		Variations: []evaluation.Variation{{Description: strPtr("desc"), Identifier: "id", Name: strPtr("name"), Value: "val"}},
		Segments:   map[string]*evaluation.Segment(nil),
	}

	featureBarKey = dto.Key{
		Type: dto.KeyFeature,
		Name: evaluationSegmentFoo.Identifier,
	}
)

func TestEmptyCache(t *testing.T) {
	memCache := NewMemCache()
	wrapper := NewWrapper(&memCache, "testEnv", logrus.New())

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
			cache := NewWrapper(memCachePtr(NewMemCache()), "env", logrus.New())
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
			cache := NewWrapper(memCachePtr(NewMemCache()), "env", logrus.New())
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
			cache := NewWrapper(memCachePtr(NewMemCache()), "env", logrus.New())
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
	wrapper := NewWrapper(&memCache, "testEnv", logrus.New())
	logger, hook := test.NewNullLogger()
	wrapper.SetLogger(logger)

	t.Run("Set with invalid key", func(t *testing.T) {
		wrapper.Set("invalidKey", evaluationSegmentFoo)
		assert.Equal(t, "Set failed: couldn't convert key to dto.Key: invalidKey", hook.LastEntry().Message)
		assert.Equal(t, 0, wrapper.Len())
	})

	t.Run("Get with invalid key", func(t *testing.T) {
		wrapper.Get("invalidKey")
		assert.Equal(t, "Get failed: couldn't convert key to dto.Key: invalidKey", hook.LastEntry().Message)
	})

	t.Run("Remove with invalid key", func(t *testing.T) {
		wrapper.Remove("invalidKey")
		assert.Equal(t, "Remove failed: couldn't convert key to dto.Key: invalidKey", hook.LastEntry().Message)
	})

	t.Run("Set segment invalid input", func(t *testing.T) {
		wrapper.Set(segmentFooKey, "invalidSegment")
		assert.Equal(t, "Set failed: couldn't convert to evaluation.Segment", hook.LastEntry().Message)
		assert.Equal(t, 0, wrapper.Len())
	})

	t.Run("Set feature invalid input", func(t *testing.T) {
		wrapper.Set(featureBarKey, "invalidFeature")
		assert.Equal(t, "Set failed: couldn't convert to evaluation.FeatureFlag", hook.LastEntry().Message)
		assert.Equal(t, 0, wrapper.Len())
	})

	t.Run("Set invalid key type", func(t *testing.T) {
		wrapper.Set(dto.Key{Name: "test", Type: "invalidType"}, "invalidFeature")
		assert.Equal(t, "Set failed: key type not recognised: invalidType", hook.LastEntry().Message)
		assert.Equal(t, 0, wrapper.Len())
	})

	t.Run("Resize not implemented", func(t *testing.T) {
		evicted := wrapper.Resize(100)
		assert.Equal(t, "Resize method not supported", hook.LastEntry().Message)
		assert.Equal(t, 0, evicted)
	})
}
