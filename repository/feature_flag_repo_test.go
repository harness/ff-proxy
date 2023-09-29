package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/v2/cache"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/domain"
)

var (
	featureFlagFoo = domain.FeatureFlag{
		DefaultServe: clientgen.Serve{
			Variation: strPtr("true"),
		},
		Environment:   "foo",
		Feature:       "foo",
		Kind:          "boolean",
		OffVariation:  "false",
		Prerequisites: &[]clientgen.Prerequisite{},
		Project:       "foo",
		Rules: &[]clientgen.ServingRule{
			{
				Clauses: []clientgen.Clause{
					{
						Attribute: "name",
						Id:        domain.ToPtr("79f5bca0-17ca-42c2-8934-5cee840fe2e0"),
						Negate:    false,
						Op:        "equal",
						Values: []string{
							"John",
						},
					},
				},
				Priority: 1,
				RuleId:   domain.ToPtr("8756c207-abf8-4202-83fd-dedf5d27e2c2"),
				Serve: clientgen.Serve{
					Variation: strPtr("false"),
				},
			},
		},
		State: clientgen.On,
		VariationToTargetMap: &[]clientgen.VariationMap{
			{
				TargetSegments: &[]string{
					"flagsTeam",
				},
				Targets: &[]clientgen.TargetMap{
					{
						Identifier: "davej",
						Name:       "Dave Johnston",
					},
				},
				Variation: "false",
			},
		},
		Variations: []clientgen.Variation{
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
		DefaultServe: clientgen.Serve{
			Variation: strPtr("true"),
		},
		Environment:   "bar",
		Feature:       "bar",
		Kind:          "boolean",
		OffVariation:  "false",
		Prerequisites: &[]clientgen.Prerequisite{},
		Project:       "bar",
		Rules: &[]clientgen.ServingRule{
			{
				Clauses: []clientgen.Clause{
					{
						Attribute: "name",
						Id:        domain.ToPtr("79f5bca0-17ca-42c2-8934-5cee840fe2e0"),
						Negate:    false,
						Op:        "equal",
						Values: []string{
							"John",
						},
					},
				},
				Priority: 1,
				RuleId:   domain.ToPtr("8756c207-abf8-4202-83fd-dedf5d27e2c2"),
				Serve: clientgen.Serve{
					Variation: strPtr("false"),
				},
			},
		},
		State: clientgen.On,
		VariationToTargetMap: &[]clientgen.VariationMap{
			{
				TargetSegments: &[]string{
					"flagsTeam",
				},
				Targets: &[]clientgen.TargetMap{
					{
						Identifier: "davej",
						Name:       "Dave Johnston",
					},
				},
				Variation: "false",
			},
		},
		Variations: []clientgen.Variation{
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
	emptyConfig := []domain.FlagConfig{}
	populatedConfig := []domain.FlagConfig{
		{
			EnvironmentID: "123",
			FeatureConfigs: []domain.FeatureFlag{
				featureFlagFoo,
			},
		},
	}

	testCases := map[string]struct {
		cache       cache.Cache
		repoConfig  []domain.FlagConfig
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
			ctx := context.Background()

			repo := NewFeatureFlagRepo(tc.cache)
			assert.Nil(t, repo.Add(ctx, tc.repoConfig...))

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
	emptyConfig := []domain.FlagConfig{}

	populatedConfig := []domain.FlagConfig{
		{
			EnvironmentID: "123",
			FeatureConfigs: []domain.FeatureFlag{
				featureFlagFoo,
				featureFlagBar,
			},
		},
	}

	testCases := map[string]struct {
		cache      cache.MemCache
		repoConfig []domain.FlagConfig
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
			ctx := context.Background()

			repo := NewFeatureFlagRepo(tc.cache)
			assert.Nil(t, repo.Add(ctx, tc.repoConfig...))

			actual, err := repo.Get(ctx, "123")
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.ElementsMatch(t, tc.expected, actual)
		})
	}
}
