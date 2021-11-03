package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/gen"
	"github.com/stretchr/testify/assert"
)

var (
	featureConfigFoo = domain.FeatureConfig{
		FeatureConfig: gen.FeatureConfig{
			DefaultServe: gen.Serve{
				Variation: strPtr("true"),
			},
			Environment:   "foo",
			Feature:       "foo",
			Kind:          gen.FeatureConfigKindBoolean,
			OffVariation:  "false",
			Prerequisites: &[]gen.Prerequisite{},
			Project:       "foo",
			Rules: &[]gen.ServingRule{
				{
					Clauses: []gen.Clause{
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
					Serve: gen.Serve{
						Variation: strPtr("false"),
					},
				},
			},
			State: gen.FeatureStateOn,
			VariationToTargetMap: &[]gen.VariationMap{
				{
					TargetSegments: &[]string{
						"flagsTeam",
					},
					Targets: &[]gen.TargetMap{
						{
							Identifier: strPtr("davej"),
							Name:       "Dave Johnston",
						},
					},
					Variation: "false",
				},
			},
			Variations: []gen.Variation{
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
		},
		Segments: map[string]gen.Segment{
			"flagsTeam": {
				Environment: strPtr("featureflagsqa"),
				Identifier:  "flagsTeam",
				Name:        "flagsTeam",
				Excluded:    &[]gen.Target{},
				Included:    &[]gen.Target{},
				Version:     int64Ptr(1),
				CreatedAt:   int64Ptr(123),
				ModifiedAt:  int64Ptr(456),
				Tags:        nil,
				Rules: &[]gen.Clause{
					{
						Attribute: "ip",
						Id:        "31c18ee7-8051-44cc-8507-b44580467ee5",
						Negate:    false,
						Op:        "equal",
						Values: []string{
							"2a00:23c5:b672:2401:158:f2a6:67a0:6a79",
						},
					},
				},
			},
		},
	}

	featureConfigBar = domain.FeatureConfig{
		FeatureConfig: gen.FeatureConfig{
			DefaultServe: gen.Serve{
				Variation: strPtr("true"),
			},
			Environment:   "bar",
			Feature:       "bar",
			Kind:          gen.FeatureConfigKindBoolean,
			OffVariation:  "false",
			Prerequisites: &[]gen.Prerequisite{},
			Project:       "bar",
			Rules: &[]gen.ServingRule{
				{
					Clauses: []gen.Clause{
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
					Serve: gen.Serve{
						Variation: strPtr("false"),
					},
				},
			},
			State: gen.FeatureStateOn,
			VariationToTargetMap: &[]gen.VariationMap{
				{
					TargetSegments: &[]string{
						"flagsTeam",
					},
					Targets: &[]gen.TargetMap{
						{
							Identifier: strPtr("davej"),
							Name:       "Dave Johnston",
						},
					},
					Variation: "false",
				},
			},
			Variations: []gen.Variation{
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
		},
		Segments: map[string]gen.Segment{
			"flagsTeam": {
				Environment: strPtr("featureflagsqa"),
				Identifier:  "flagsTeam",
				Name:        "flagsTeam",
				Excluded:    &[]gen.Target{},
				Included:    &[]gen.Target{},
				Version:     int64Ptr(1),
				CreatedAt:   int64Ptr(123),
				ModifiedAt:  int64Ptr(456),
				Tags:        nil,
				Rules: &[]gen.Clause{
					{
						Attribute: "ip",
						Id:        "31c18ee7-8051-44cc-8507-b44580467ee5",
						Negate:    false,
						Op:        "equal",
						Values: []string{
							"2a00:23c5:b672:2401:158:f2a6:67a0:6a79",
						},
					},
				},
			},
		},
	}
)

func TestFeatureConfigRepo_Add(t *testing.T) {
	key123 := domain.NewFeatureConfigKey("123")

	emptyConfig := map[domain.FeatureConfigKey][]domain.FeatureConfig{}
	populatedConfig := map[domain.FeatureConfigKey][]domain.FeatureConfig{
		key123: {featureConfigFoo},
	}

	testCases := map[string]struct {
		cache      Cache
		repoConfig map[domain.FeatureConfigKey][]domain.FeatureConfig
		targets    []domain.FeatureConfig
		key        domain.FeatureConfigKey
		shouldErr  bool
		expected   []domain.FeatureConfig
		expecteErr error
	}{
		"Given I have an empty repo and I add a FeatureConfig to it": {
			cache:      cache.NewMemCache(),
			repoConfig: emptyConfig,
			targets:    []domain.FeatureConfig{featureConfigFoo},
			key:        key123,
			shouldErr:  false,
			expected:   []domain.FeatureConfig{featureConfigFoo},
			expecteErr: nil,
		},
		"Given I have a repo with a target in it and I add the same target again under the same key": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			targets:    []domain.FeatureConfig{featureConfigFoo},
			key:        key123,
			shouldErr:  false,
			expected:   []domain.FeatureConfig{featureConfigFoo},
			expecteErr: nil,
		},
		"Given I have a repo with a target in it and I add a new target under the same key": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			targets:    []domain.FeatureConfig{featureConfigBar},
			key:        key123,
			shouldErr:  false,
			expected:   []domain.FeatureConfig{featureConfigFoo, featureConfigBar},
			expecteErr: nil,
		},
		"Given I add an target to the repo but the cache errors": {
			cache: &mockCache{
				set:    func() error { return errors.New("an error") },
				getAll: func() (map[string][]byte, error) { return map[string][]byte{}, nil },
			},
			repoConfig: nil,
			targets:    []domain.FeatureConfig{featureConfigBar},
			key:        key123,
			shouldErr:  true,
			expected:   []domain.FeatureConfig{},
			expecteErr: domain.CacheInternalErr{},
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			repo, err := NewFeatureConfigRepo(tc.cache, tc.repoConfig)
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

func TestFeatureConfigRepo_GetByIdentifer(t *testing.T) {
	key123 := domain.NewFeatureConfigKey("123")

	emptyConfig := map[domain.FeatureConfigKey][]domain.FeatureConfig{}
	populatedConfig := map[domain.FeatureConfigKey][]domain.FeatureConfig{
		key123: {featureConfigFoo},
	}

	testCases := map[string]struct {
		cache       Cache
		repoConfig  map[domain.FeatureConfigKey][]domain.FeatureConfig
		key         domain.FeatureConfigKey
		identifier  string
		shouldErr   bool
		expected    domain.FeatureConfig
		expectedErr error
	}{
		"Given I have an empty cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			key:         key123,
			identifier:  "foo",
			shouldErr:   true,
			expected:    domain.FeatureConfig{},
			expectedErr: domain.CacheNotFoundErr{},
		},
		"Given I have a populated cache and I get an identifier that's in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  populatedConfig,
			key:         key123,
			identifier:  "foo",
			shouldErr:   false,
			expected:    featureConfigFoo,
			expectedErr: nil,
		},
		"Given I have a populated cache and I try to get an identifier that isn't in the cache": {
			cache:       cache.NewMemCache(),
			repoConfig:  emptyConfig,
			key:         key123,
			identifier:  "bar",
			shouldErr:   true,
			expected:    domain.FeatureConfig{},
			expectedErr: domain.CacheNotFoundErr{},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			repo, err := NewFeatureConfigRepo(tc.cache, tc.repoConfig)
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
