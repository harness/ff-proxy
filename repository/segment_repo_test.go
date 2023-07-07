package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-golang-server-sdk/rest"

	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/domain"
	"github.com/stretchr/testify/assert"
)

var (
	segmentFoo = domain.Segment{
		CreatedAt:   int64Ptr(123),
		Environment: strPtr("featureFlagsQA"),
		Identifier:  "foo",
		ModifiedAt:  int64Ptr(456),
		Name:        "fooSegment",
		Excluded:    &[]rest.Target{},
		Included:    &[]rest.Target{},
		Rules:       &[]rest.Clause{},
		Tags:        &[]rest.Tag{},
		Version:     int64Ptr(12),
	}

	segmentBar = domain.Segment{
		CreatedAt:   int64Ptr(123),
		Environment: strPtr("featureFlagsQA"),
		Identifier:  "bar",
		ModifiedAt:  int64Ptr(456),
		Name:        "barSegment",
		Excluded:    &[]rest.Target{},
		Included:    &[]rest.Target{},
		Rules:       &[]rest.Clause{},
		Tags:        &[]rest.Tag{},
		Version:     int64Ptr(12),
	}
)

func TestSegmentRepo_GetByIdentifer(t *testing.T) {
	key123 := domain.NewSegmentsKey("123")

	emptyConfig := map[domain.SegmentKey]interface{}{}
	populatedConfig := map[domain.SegmentKey]interface{}{
		key123: []domain.Segment{segmentFoo},
		domain.NewSegmentKey("123", segmentFoo.Identifier): segmentFoo,
	}

	testCases := map[string]struct {
		cache       cache.Cache
		repoConfig  map[domain.SegmentKey]interface{}
		envID       string
		identifier  string
		shouldErr   bool
		expected    domain.Segment
		expectedErr error
	}{
		"Given I have an empty cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			envID:       "123",
			identifier:  "foo",
			shouldErr:   true,
			expected:    domain.Segment{},
			expectedErr: domain.ErrCacheNotFound,
		},
		"Given I have a populated cache and I get an identifier that's in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  populatedConfig,
			envID:       "123",
			identifier:  "foo",
			shouldErr:   false,
			expected:    segmentFoo,
			expectedErr: nil,
		},
		"Given I have a populated cache and I try to get an identifier that isn't in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			envID:       "123",
			identifier:  "bar",
			shouldErr:   true,
			expected:    domain.Segment{},
			expectedErr: domain.ErrCacheNotFound,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			repo, err := NewSegmentRepo(tc.cache, WithSegmentConfig(tc.repoConfig))
			if err != nil {
				t.Fatalf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			actual, err := repo.GetByIdentifier(context.Background(), tc.envID, tc.identifier)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
				ok := errors.Is(err, tc.expectedErr)
				assert.True(t, ok)
			}

			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestSegmentRepoGet(t *testing.T) {
	key123 := domain.NewSegmentsKey("123")

	emptyConfig := map[domain.SegmentKey]interface{}{}
	populatedConfig := map[domain.SegmentKey]interface{}{
		key123: []domain.Segment{segmentFoo, segmentBar},
		domain.NewSegmentKey("123", segmentFoo.Identifier): segmentFoo,
	}

	testCases := map[string]struct {
		cache      cache.MemCache
		repoConfig map[domain.SegmentKey]interface{}
		shouldErr  bool
		expected   []domain.Segment
	}{
		"Given I call Get with an empty SegmentRepo": {
			cache:      cache.NewMemCache(),
			repoConfig: emptyConfig,
			shouldErr:  true,
			expected:   []domain.Segment{},
		},
		"Given I call Get with a populated SegmentRepo": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			shouldErr:  false,
			expected:   []domain.Segment{segmentFoo, segmentBar},
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			repo, err := NewSegmentRepo(tc.cache, WithSegmentConfig(tc.repoConfig))
			if err != nil {
				t.Fatalf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			actual, err := repo.Get(context.Background(), "123")
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.ElementsMatch(t, tc.expected, actual)
		})
	}
}
