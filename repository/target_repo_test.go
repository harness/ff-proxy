package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
	admingen "github.com/harness/ff-proxy/v2/gen/admin"
	"github.com/stretchr/testify/assert"
)

type mockCache struct {
	set    func() error
	get    func() error
	delete func() error
}

func (m *mockCache) HealthCheck(ctx context.Context) error {
	return nil
}

// Set sets a value in the cache for a given key and field
func (m *mockCache) Set(ctx context.Context, key string, value interface{}) error {
	return m.set()
}

// Get gets the value of a field for a given key
func (m *mockCache) Get(ctx context.Context, key string, v interface{}) error {
	return m.get()
}

// Remove removes a field for a given key
func (m *mockCache) Delete(ctx context.Context, key string) error {
	return m.delete()
}

func (m *mockCache) Keys(ctx context.Context, key string) ([]string, error) {
	return []string{}, nil
}

func strPtr(s string) *string { return &s }

func int64Ptr(i int64) *int64 { return &i }

func boolPtr(b bool) *bool { return &b }

var (
	targetFoo = domain.Target{
		Target: admingen.Target{
			Account:     "foo",
			Anonymous:   boolPtr(false),
			CreatedAt:   int64Ptr(1634222520273),
			Environment: "featureflagsqa",
			Identifier:  "foo",
			Name:        "foo",
			Org:         "foo",
			Project:     "FeatureFlagsQADemo",
			Segments:    &[]admingen.Segment{},
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
		},
	}

	targetBar = domain.Target{
		Target: admingen.Target{
			Account:     "bar",
			Anonymous:   boolPtr(false),
			CreatedAt:   int64Ptr(1634222520273),
			Environment: "featureflagsqa",
			Identifier:  "bar",
			Name:        "bar",
			Org:         "bar",
			Project:     "FeatureFlagsQADemo",
			Segments:    &[]admingen.Segment{},
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
		},
	}
)

func TestTargetRepo_GetByIdentifer(t *testing.T) {
	key123 := domain.NewTargetsKey("123")

	emptyConfig := map[domain.TargetKey]interface{}{}
	populatedConfig := map[domain.TargetKey]interface{}{
		key123: []domain.Target{targetFoo},
		domain.NewTargetKey("123", targetFoo.Identifier): targetFoo,
	}

	testCases := map[string]struct {
		cache       cache.Cache
		repoConfig  map[domain.TargetKey]interface{}
		envID       string
		identifier  string
		shouldErr   bool
		expected    domain.Target
		expectedErr error
	}{
		"Given I have an empty cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			envID:       "123",
			identifier:  "foo",
			shouldErr:   true,
			expected:    domain.Target{},
			expectedErr: domain.ErrCacheNotFound,
		},
		"Given I have a populated cache and I get an identifier that's in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  populatedConfig,
			envID:       "123",
			identifier:  "foo",
			shouldErr:   false,
			expected:    targetFoo,
			expectedErr: nil,
		},
		"Given I have a populated cache and I try to get an identifier that isn't in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			envID:       "123",
			identifier:  "bar",
			shouldErr:   true,
			expected:    domain.Target{},
			expectedErr: domain.ErrCacheNotFound,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			repo, err := NewTargetRepo(tc.cache, WithTargetConfig(tc.repoConfig))
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

func TestTargetRepo_DeltaAdd(t *testing.T) {
	key123 := domain.NewTargetsKey("123")

	target1 := domain.Target{
		Target: admingen.Target{
			Identifier:  "target1",
			Name:        "target1",
			Environment: "123",
			Project:     "foo",
		},
	}

	target2 := domain.Target{
		Target: admingen.Target{
			Identifier:  "target2",
			Name:        "target2",
			Environment: "123",
			Project:     "foo",
		},
	}

	target3 := domain.Target{
		Target: admingen.Target{
			Identifier:  "target3",
			Name:        "target3",
			Environment: "123",
			Project:     "foo",
		},
	}

	target1ProjectBar := domain.Target{
		Target: admingen.Target{
			Identifier:  "target1",
			Name:        "target1",
			Environment: "123",
			Project:     "bar",
		},
	}

	target2ProjectBar := domain.Target{
		Target: admingen.Target{
			Identifier:  "target2",
			Name:        "target2",
			Environment: "123",
			Project:     "bar",
		},
	}

	testCases := map[string]struct {
		cache      cache.Cache
		repoConfig map[domain.TargetKey]interface{}
		env        string
		targets    []domain.Target
		expected   []domain.Target
		shouldErr  bool
	}{
		"Given I have an empty TargetRepo and I add two Targets": {
			cache:      cache.NewMemCache(),
			repoConfig: map[domain.TargetKey]interface{}{},
			env:        "123",
			targets:    []domain.Target{target1, target2},
			expected:   []domain.Target{target1, target2},
			shouldErr:  false,
		},
		"Given I have a TargetRepo with Target3 and I add Target1 and Target1": {
			cache: cache.NewMemCache(),
			repoConfig: map[domain.TargetKey]interface{}{
				key123: []domain.Target{target3},
			},
			env:       "123",
			targets:   []domain.Target{target1, target2},
			expected:  []domain.Target{target1, target2},
			shouldErr: false,
		},
		"Given I have a TargetRepo with two Targets and I add the same Targets with a different Project value ": {
			cache: cache.NewMemCache(),
			repoConfig: map[domain.TargetKey]interface{}{
				key123: []domain.Target{target1, target2},
			},
			env:       "123",
			targets:   []domain.Target{target1ProjectBar, target2ProjectBar},
			expected:  []domain.Target{target1ProjectBar, target2ProjectBar},
			shouldErr: false,
		},
		"Given I have a TargetRepo with two Targets and I try to add no Targets": {
			cache: cache.NewMemCache(),
			repoConfig: map[domain.TargetKey]interface{}{
				key123: []domain.Target{target1, target2},
			},
			env:       "123",
			targets:   []domain.Target{},
			expected:  []domain.Target{target1, target2},
			shouldErr: true,
		},
	}

	for desc, tc := range testCases {
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()

			repo, err := NewTargetRepo(tc.cache, WithTargetConfig(tc.repoConfig))
			if err != nil {
				t.Fatalf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			err = repo.DeltaAdd(ctx, tc.env, tc.targets...)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			t.Log("And the values in the repo should match the expected values")
			actual, err := repo.Get(ctx, tc.env)
			if err != nil {
				t.Errorf("(%s): unexpected error getting targets: %s", desc, err)
			}

			assert.ElementsMatch(t, tc.expected, actual)
		})
	}
}
