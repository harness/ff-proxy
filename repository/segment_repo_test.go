package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

var (
	segmentFoo = domain.Segment{
		CreatedAt:   int64Ptr(123),
		Environment: strPtr("featureFlagsQA"),
		Identifier:  "foo",
		ModifiedAt:  int64Ptr(456),
		Name:        "fooSegment",
		Excluded:    &[]clientgen.Target{},
		Included:    &[]clientgen.Target{},
		Rules:       &[]clientgen.Clause{},
		Tags:        &[]clientgen.Tag{},
		Version:     int64Ptr(12),
	}

	segmentBar = domain.Segment{
		CreatedAt:   int64Ptr(123),
		Environment: strPtr("featureFlagsQA"),
		Identifier:  "bar",
		ModifiedAt:  int64Ptr(456),
		Name:        "barSegment",
		Excluded:    &[]clientgen.Target{},
		Included:    &[]clientgen.Target{},
		Rules:       &[]clientgen.Clause{},
		Tags:        &[]clientgen.Tag{},
		Version:     int64Ptr(12),
	}
)

func TestSegmentRepo_GetByIdentifer(t *testing.T) {
	emptyConfig := []domain.SegmentConfig{}
	populatedConfig := []domain.SegmentConfig{
		{
			EnvironmentID: "123",
			Segments:      []domain.Segment{segmentFoo, segmentBar},
		},
	}

	testCases := map[string]struct {
		cache       cache.Cache
		repoConfig  []domain.SegmentConfig
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
		"Given I have a populated cache and I get identifier=foo that's in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  populatedConfig,
			envID:       "123",
			identifier:  "foo",
			shouldErr:   false,
			expected:    segmentFoo,
			expectedErr: nil,
		},
		"Given I have a populated cache and I get an identifier=bar that's in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  populatedConfig,
			envID:       "123",
			identifier:  "bar",
			shouldErr:   false,
			expected:    segmentBar,
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
			ctx := context.Background()

			repo := NewSegmentRepo(tc.cache)
			assert.Nil(t, repo.Add(ctx, tc.repoConfig...))

			actual, err := repo.GetByIdentifier(ctx, tc.envID, tc.identifier)
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

	emptyConfig := []domain.SegmentConfig{}
	populatedConfig := []domain.SegmentConfig{
		{
			EnvironmentID: "123",
			Segments:      []domain.Segment{segmentFoo, segmentBar},
		},
	}

	testCases := map[string]struct {
		cache      cache.MemCache
		repoConfig []domain.SegmentConfig
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
			ctx := context.Background()

			repo := NewSegmentRepo(tc.cache)
			assert.Nil(t, repo.Add(ctx, tc.repoConfig...))

			actual, err := repo.Get(ctx, "123")
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.ElementsMatch(t, tc.expected, actual)
		})
	}
}

func TestSegmentRepo_Remove(t *testing.T) {
	emptyConfig := []domain.SegmentConfig{}
	populatedConfig := []domain.SegmentConfig{
		{
			EnvironmentID: "123",
			Segments:      []domain.Segment{segmentFoo, segmentBar},
		},
	}

	testCases := map[string]struct {
		cache      cache.MemCache
		repoConfig []domain.SegmentConfig
		shouldErr  bool
	}{
		"Given I call Remove with and the Segment config does not exist": {
			cache:      cache.NewMemCache(),
			repoConfig: emptyConfig,
			shouldErr:  true,
		},
		"Given I call Remove with and Segment config does exist": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			shouldErr:  false,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()
			repo := NewSegmentRepo(tc.cache)

			if tc.shouldErr {
				assert.Error(t, repo.Remove(ctx, "123"))

			} else {
				assert.Nil(t, repo.Add(ctx, tc.repoConfig...))
				assert.Nil(t, repo.Remove(ctx, "123"))
				flags, err := repo.Get(ctx, "123")
				assert.Equal(t, flags, []domain.Segment{})
				assert.Error(t, err)

			}
		})
	}
}
