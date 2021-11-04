package repository

import (
	"context"
	"encoding"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/gen"
	"github.com/stretchr/testify/assert"
)

type mockCache struct {
	set    func() error
	get    func() error
	getAll func() (map[string][]byte, error)
}

// Set sets a value in the cache for a given key and field
func (m *mockCache) Set(ctx context.Context, key string, field string, value encoding.BinaryMarshaler) error {
	return m.set()
}

// Get gets the value of a field for a given key
func (m *mockCache) Get(ctx context.Context, key string, field string, v encoding.BinaryUnmarshaler) error {
	return m.get()
}

// GetAll gets all of the fiels and their values for a given key
func (m *mockCache) GetAll(ctx context.Context, key string) (map[string][]byte, error) {
	return m.getAll()
}

func strPtr(s string) *string { return &s }

func int64Ptr(i int64) *int64 { return &i }

func boolPtr(b bool) *bool { return &b }

var (
	targetFoo = domain.Target{
		Account:     "foo",
		Anonymous:   boolPtr(false),
		CreatedAt:   int64Ptr(1634222520273),
		Environment: "featureflagsqa",
		Identifier:  "foo",
		Name:        "foo",
		Org:         "foo",
		Project:     "FeatureFlagsQADemo",
		Segments:    &[]gen.Segment{},
		Attributes: &map[string]interface{}{
			"age": float64(55),
			"ages": []interface{}{
				float64(1),
				float64(2),
				float64(3),
			},
			"happy":      true,
			"host":       "foo.com",
			"userGroups": []interface{}{"Foo", "Volvo", "BMW"},
		},
	}

	targetBar = domain.Target{
		Account:     "bar",
		Anonymous:   boolPtr(false),
		CreatedAt:   int64Ptr(1634222520273),
		Environment: "featureflagsqa",
		Identifier:  "bar",
		Name:        "bar",
		Org:         "bar",
		Project:     "FeatureFlagsQADemo",
		Segments:    &[]gen.Segment{},
		Attributes: &map[string]interface{}{
			"age": float64(55),
			"ages": []interface{}{
				float64(1),
				float64(2),
				float64(3),
			},
			"happy":      true,
			"host":       "foo.com",
			"userGroups": []interface{}{"Foo", "Volvo", "BMW"},
		},
	}
)

func TestTargetRepo_Add(t *testing.T) {
	key123 := domain.NewTargetKey("123")

	emptyConfig := map[domain.TargetKey][]domain.Target{}
	populatedConfig := map[domain.TargetKey][]domain.Target{
		key123: {targetFoo},
	}

	testCases := map[string]struct {
		cache      Cache
		repoConfig map[domain.TargetKey][]domain.Target
		targets    []domain.Target
		key        domain.TargetKey
		shouldErr  bool
		expected   []domain.Target
	}{
		"Given I have an empty repo and I add a Target to it": {
			cache:      cache.NewMemCache(),
			repoConfig: emptyConfig,
			targets:    []domain.Target{targetFoo},
			key:        key123,
			shouldErr:  false,
			expected:   []domain.Target{targetFoo},
		},
		"Given I have a repo with a target in it and I add the same target again under the same key": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			targets:    []domain.Target{targetFoo},
			key:        key123,
			shouldErr:  false,
			expected:   []domain.Target{targetFoo},
		},
		"Given I have a repo with a target in it and I add a new target under the same key": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			targets:    []domain.Target{targetBar},
			key:        key123,
			shouldErr:  false,
			expected:   []domain.Target{targetFoo, targetBar},
		},
		"Given I add an target to the repo but the cache errors": {
			cache: &mockCache{
				set:    func() error { return errors.New("an error") },
				getAll: func() (map[string][]byte, error) { return map[string][]byte{}, nil },
			},
			repoConfig: nil,
			targets:    []domain.Target{targetBar},
			key:        key123,
			shouldErr:  true,
			expected:   []domain.Target{},
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			repo, err := NewTargetRepo(tc.cache, tc.repoConfig)
			if err != nil {
				t.Fatalf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			err = repo.Add(context.Background(), tc.key, tc.targets...)
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

func TestTargetRepo_GetByIdentifer(t *testing.T) {
	key123 := domain.NewTargetKey("123")

	emptyConfig := map[domain.TargetKey][]domain.Target{}
	populatedConfig := map[domain.TargetKey][]domain.Target{
		key123: {targetFoo},
	}

	testCases := map[string]struct {
		cache       Cache
		repoConfig  map[domain.TargetKey][]domain.Target
		key         domain.TargetKey
		identifier  string
		shouldErr   bool
		expected    domain.Target
		expectedErr error
	}{
		"Given I have an empty cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			key:         key123,
			identifier:  "foo",
			shouldErr:   true,
			expected:    domain.Target{},
			expectedErr: domain.ErrCacheNotFound,
		},
		"Given I have a populated cache and I get an identifier that's in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  populatedConfig,
			key:         key123,
			identifier:  "foo",
			shouldErr:   false,
			expected:    targetFoo,
			expectedErr: nil,
		},
		"Given I have a populated cache and I try to get an identifier that isn't in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			key:         key123,
			identifier:  "bar",
			shouldErr:   true,
			expected:    domain.Target{},
			expectedErr: domain.ErrCacheNotFound,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			repo, err := NewTargetRepo(tc.cache, tc.repoConfig)
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
