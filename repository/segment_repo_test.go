package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
	"github.com/stretchr/testify/assert"
)

var (
	segmentFoo = domain.Segment{
		Segment: clientgen.Segment{
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
		},
	}

	segmentBar = domain.Segment{
		Segment: clientgen.Segment{
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
		},
	}
)

func TestSegmentRepo_Add(t *testing.T) {
	key123 := domain.NewSegmentKey("123")

	emptyConfig := map[domain.SegmentKey][]domain.Segment{}
	populatedConfig := map[domain.SegmentKey][]domain.Segment{
		key123: {segmentFoo},
	}

	testCases := map[string]struct {
		cache      Cache
		repoConfig map[domain.SegmentKey][]domain.Segment
		segments   []domain.Segment
		key        domain.SegmentKey
		shouldErr  bool
		expected   []domain.Segment
	}{
		"Given I have an empty repo and I add a Segment to it": {
			cache:      cache.NewMemCache(),
			repoConfig: emptyConfig,
			segments:   []domain.Segment{segmentFoo},
			key:        key123,
			shouldErr:  false,
			expected:   []domain.Segment{segmentFoo},
		},
		"Given I have a repo with a segment in it and I add the same segment again under the same key": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			segments:   []domain.Segment{segmentFoo},
			key:        key123,
			shouldErr:  false,
			expected:   []domain.Segment{segmentFoo},
		},
		"Given I have a repo with a segment in it and I add a new segment under the same key": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			segments:   []domain.Segment{segmentBar},
			key:        key123,
			shouldErr:  false,
			expected:   []domain.Segment{segmentFoo, segmentBar},
		},
		"Given I add an segment to the repo but the cache errors": {
			cache: &mockCache{
				set:    func() error { return errors.New("an error") },
				getAll: func() (map[string][]byte, error) { return map[string][]byte{}, nil },
			},
			repoConfig: nil,
			segments:   []domain.Segment{segmentBar},
			key:        key123,
			shouldErr:  true,
			expected:   []domain.Segment{},
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			repo, err := NewSegmentRepo(tc.cache, tc.repoConfig)
			if err != nil {
				t.Fatalf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			err = repo.Add(context.Background(), tc.key, tc.segments...)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			actual, err := repo.Get(context.Background(), tc.key)
			if err != nil {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestSegmentRepo_GetByIdentifer(t *testing.T) {
	key123 := domain.NewSegmentKey("123")

	emptyConfig := map[domain.SegmentKey][]domain.Segment{}
	populatedConfig := map[domain.SegmentKey][]domain.Segment{
		key123: {segmentFoo},
	}

	testCases := map[string]struct {
		cache       Cache
		repoConfig  map[domain.SegmentKey][]domain.Segment
		key         domain.SegmentKey
		identifier  string
		shouldErr   bool
		expected    domain.Segment
		expectedErr error
	}{
		"Given I have an empty cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			key:         key123,
			identifier:  "foo",
			shouldErr:   true,
			expected:    domain.Segment{},
			expectedErr: domain.ErrCacheNotFound,
		},
		"Given I have a populated cache and I get an identifier that's in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  populatedConfig,
			key:         key123,
			identifier:  "foo",
			shouldErr:   false,
			expected:    segmentFoo,
			expectedErr: nil,
		},
		"Given I have a populated cache and I try to get an identifier that isn't in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			key:         key123,
			identifier:  "bar",
			shouldErr:   true,
			expected:    domain.Segment{},
			expectedErr: domain.ErrCacheNotFound,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			repo, err := NewSegmentRepo(tc.cache, tc.repoConfig)
			if err != nil {
				t.Fatalf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			actual, err := repo.GetByIdentifier(context.Background(), tc.key, tc.identifier)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
				ok := errors.Is(err, tc.expectedErr)
				assert.True(t, ok)
			}

			assert.Equal(t, tc.expected, actual)
		})
	}
}
