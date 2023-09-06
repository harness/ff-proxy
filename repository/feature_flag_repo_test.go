package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-golang-server-sdk/rest"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/stretchr/testify/assert"
)

var (
	featureFlagFoo = domain.FeatureFlag{
		DefaultServe: rest.Serve{
			Variation: strPtr("true"),
		},
		Environment:   "foo",
		Feature:       "foo",
		Kind:          "boolean",
		OffVariation:  "false",
		Prerequisites: &[]rest.Prerequisite{},
		Project:       "foo",
		Rules: &[]rest.ServingRule{
			{
				Clauses: []rest.Clause{
					{
						Attribute: "name",
						Id:        "79f5bca0-17ca-42c2-8934-5cee840fe2e0",
						Negate:    false,
						Op:        "equal",
						Values: []string{
							"John",
						},
					},
				},
				Priority: 1,
				RuleId:   "8756c207-abf8-4202-83fd-dedf5d27e2c2",
				Serve: rest.Serve{
					Variation: strPtr("false"),
				},
			},
		},
		State: rest.FeatureStateOn,
		VariationToTargetMap: &[]rest.VariationMap{
			{
				TargetSegments: &[]string{
					"flagsTeam",
				},
				Targets: &[]rest.TargetMap{
					{
						Identifier: strPtr("davej"),
						Name:       "Dave Johnston",
					},
				},
				Variation: "false",
			},
		},
		Variations: []rest.Variation{
			{
				Description: nil,
				Identifier:  "true",
				Name:        strPtr("True"),
				Value:       "true",
			},
			{
				Description: nil,
				Identifier:  "false",
				Name:        strPtr("False"),
				Value:       "false",
			},
		},
		Version: int64Ptr(568),
	}

	featureFlagBar = domain.FeatureFlag{
		DefaultServe: rest.Serve{
			Variation: strPtr("true"),
		},
		Environment:   "bar",
		Feature:       "bar",
		Kind:          "boolean",
		OffVariation:  "false",
		Prerequisites: &[]rest.Prerequisite{},
		Project:       "bar",
		Rules: &[]rest.ServingRule{
			{
				Clauses: []rest.Clause{
					{
						Attribute: "name",
						Id:        "79f5bca0-17ca-42c2-8934-5cee840fe2e0",
						Negate:    false,
						Op:        "equal",
						Values: []string{
							"John",
						},
					},
				},
				Priority: 1,
				RuleId:   "8756c207-abf8-4202-83fd-dedf5d27e2c2",
				Serve: rest.Serve{
					Variation: strPtr("false"),
				},
			},
		},
		State: rest.FeatureStateOn,
		VariationToTargetMap: &[]rest.VariationMap{
			{
				TargetSegments: &[]string{
					"flagsTeam",
				},
				Targets: &[]rest.TargetMap{
					{
						Identifier: strPtr("davej"),
						Name:       "Dave Johnston",
					},
				},
				Variation: "false",
			},
		},
		Variations: []rest.Variation{
			{
				Description: nil,
				Identifier:  "true",
				Name:        strPtr("True"),
				Value:       "true",
			},
			{
				Description: nil,
				Identifier:  "false",
				Name:        strPtr("False"),
				Value:       "false",
			},
		},
		Version: int64Ptr(568),
	}
)

func TestFeatureFlagRepo_GetByIdentifer(t *testing.T) {
	key123 := domain.NewFeatureConfigsKey("123")

	emptyConfig := map[domain.FeatureFlagKey]interface{}{}
	populatedConfig := map[domain.FeatureFlagKey]interface{}{
		key123: []domain.FeatureFlag{featureFlagFoo},
		domain.NewFeatureConfigKey("123", featureFlagFoo.Feature): featureFlagFoo,
	}

	testCases := map[string]struct {
		cache       cache.Cache
		repoConfig  map[domain.FeatureFlagKey]interface{}
		envID       string
		identifier  string
		shouldErr   bool
		expected    domain.FeatureFlag
		expectedErr error
	}{
		"Given I have an empty cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			envID:       "123",
			identifier:  "foo",
			shouldErr:   true,
			expected:    domain.FeatureFlag{},
			expectedErr: domain.ErrCacheNotFound,
		},
		"Given I have a populated cache and I get an identifier that's in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  populatedConfig,
			envID:       "123",
			identifier:  "foo",
			shouldErr:   false,
			expected:    featureFlagFoo,
			expectedErr: nil,
		},
		"Given I have a populated cache and I try to get an identifier that isn't in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			envID:       "123",
			identifier:  "bar",
			shouldErr:   true,
			expected:    domain.FeatureFlag{},
			expectedErr: domain.ErrCacheNotFound,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			repo, err := NewFeatureFlagRepo(tc.cache, WithFeatureConfig(tc.repoConfig))
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

func TestFeatureFlagRepo_Get(t *testing.T) {
	key123 := domain.NewFeatureConfigsKey("123")

	emptyConfig := map[domain.FeatureFlagKey]interface{}{}
	populatedConfig := map[domain.FeatureFlagKey]interface{}{
		key123: []domain.FeatureFlag{featureFlagFoo, featureFlagBar},
		domain.NewFeatureConfigKey("123", featureFlagFoo.Feature): featureFlagFoo,
		domain.NewFeatureConfigKey("123", featureFlagBar.Feature): featureFlagBar,
	}

	testCases := map[string]struct {
		cache      cache.MemCache
		repoConfig map[domain.FeatureFlagKey]interface{}
		shouldErr  bool
		expected   []domain.FeatureFlag
	}{
		"Given I call Get with an empty FeatureFlagRepo": {
			cache:      cache.NewMemCache(),
			repoConfig: emptyConfig,
			shouldErr:  true,
			expected:   []domain.FeatureFlag{},
		},
		"Given I call Get with a populated FeatureFlagRepo": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			shouldErr:  false,
			expected:   []domain.FeatureFlag{featureFlagFoo, featureFlagBar},
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			repo, err := NewFeatureFlagRepo(tc.cache, WithFeatureConfig(tc.repoConfig))
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
