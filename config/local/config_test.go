package local

import (
	"context"
	"embed"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

var (
	// testConfig embeds the config used for testing
	//go:embed test/env-*
	testConfig embed.FS
)

const (
	// testDir is the directory that the test config lives in
	testDir = "test"
)

type mockAuthRepo struct {
	config []domain.AuthConfig

	add                           func(ctx context.Context, config ...domain.AuthConfig) error
	addAPIConfigsForEnvironmentFn func(ctx context.Context, envID string, apiKeys []string) error
}

func (m mockAuthRepo) AddAPIConfigsForEnvironment(ctx context.Context, envID string, apiKeys []string) error {
	return m.addAPIConfigsForEnvironmentFn(ctx, envID, apiKeys)
}

func (m *mockAuthRepo) PatchAPIConfigForEnvironment(ctx context.Context, envID, apikey, action string) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthRepo) Remove(ctx context.Context, id []string) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthRepo) RemoveAllKeysForEnvironment(ctx context.Context, envID string) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockAuthRepo) Add(ctx context.Context, config ...domain.AuthConfig) error {
	if err := m.add(ctx, config...); err != nil {
		return err
	}
	m.config = append(m.config, config...)
	return nil
}

type mockSegmentRepo struct {
	config []domain.SegmentConfig

	add                               func(ctx context.Context, config ...domain.SegmentConfig) error
	removeFn                          func(ctx context.Context, env, id string) error
	removeAllSegmentsForEnvironmentFn func(ctx context.Context, id string) error
	getSegmentsForEnvironmentFn       func(ctx context.Context, envID string) ([]domain.Segment, bool)
}

func (m *mockSegmentRepo) GetSegmentsForEnvironment(ctx context.Context, envID string) ([]domain.Segment, bool) {
	//TODO implement me
	panic("implement me")
}

func (m *mockSegmentRepo) RemoveAllSegmentsForEnvironment(ctx context.Context, id string) error {
	return m.removeAllSegmentsForEnvironmentFn(ctx, id)
}

func (m *mockSegmentRepo) Remove(ctx context.Context, env, id string) error {
	return m.removeFn(ctx, env, id)
}

func (m *mockSegmentRepo) Add(ctx context.Context, config ...domain.SegmentConfig) error {
	if err := m.add(ctx, config...); err != nil {
		return err
	}
	m.config = append(m.config, config...)
	return nil
}

type mockFlagRepo struct {
	config                           []domain.FlagConfig
	add                              func(ctx context.Context, config ...domain.FlagConfig) error
	getFeatureConfigForEnvironmentFn func(ctx context.Context, envID string) ([]domain.FeatureFlag, bool)
}

func (m *mockFlagRepo) GetFeatureConfigForEnvironment(ctx context.Context, envID string) ([]domain.FeatureFlag, bool) {
	return m.getFeatureConfigForEnvironmentFn(ctx, envID)
}

func (m *mockFlagRepo) RemoveAllFeaturesForEnvironment(ctx context.Context, id string) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockFlagRepo) Remove(ctx context.Context, env, id string) error {
	//TODO implement me
	panic("implement me")
}

func (m *mockFlagRepo) Add(ctx context.Context, config ...domain.FlagConfig) error {
	if err := m.add(ctx, config...); err != nil {
		return err
	}
	m.config = append(m.config, config...)
	return nil
}

var (
	harnessAppDemoDarkModeConfig = domain.FeatureFlag{
		DefaultServe: clientgen.Serve{
			Variation: domain.ToPtr("true"),
		},
		Environment:   "featureflagsqa",
		Feature:       "harnessappdemodarkmode",
		Kind:          "boolean",
		OffVariation:  "false",
		Prerequisites: &[]clientgen.Prerequisite{},
		Project:       "FeatureFlagsQADemo",
		Rules: &[]clientgen.ServingRule{
			{
				Clauses: []clientgen.Clause{
					{
						Attribute: "age",
						Id:        domain.ToPtr("79f5bca0-17ca-42c2-8934-5cee840fe2e0"),
						Negate:    false,
						Op:        "equal",
						Values: []string{
							"55",
						},
					},
				},
				Priority: 1,
				RuleId:   domain.ToPtr("8756c207-abf8-4202-83fd-dedf5d27e2c2"),
				Serve: clientgen.Serve{
					Variation: domain.ToPtr("false"),
				},
			},
		},
		State: "on",
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
				Name:        domain.ToPtr("True"),
				Value:       "true",
			},
			{
				Description: nil,
				Identifier:  "false",
				Name:        domain.ToPtr("False"),
				Value:       "false",
			},
		},
		Version: domain.ToPtr(int64(568)),
	}

	yetAnotherFlagConfig = domain.FeatureFlag{
		DefaultServe: clientgen.Serve{
			Variation: domain.ToPtr("1"),
		},
		Environment:   "featureflagsqa",
		Feature:       "yet_another_flag",
		Kind:          "string",
		OffVariation:  "2",
		Prerequisites: &[]clientgen.Prerequisite{},
		Project:       "FeatureFlagsQADemo",
		Rules:         &[]clientgen.ServingRule{},
		State:         "on",
		Variations: []clientgen.Variation{
			{
				Description: nil,
				Identifier:  "1",
				Name:        domain.ToPtr("1"),
				Value:       "1",
			},
			{
				Description: nil,
				Identifier:  "2",
				Name:        domain.ToPtr("2"),
				Value:       "2",
			},
		},
		Version: domain.ToPtr(int64(6)),
	}

	flagsTeamSegment = domain.Segment{
		Environment: domain.ToPtr("featureflagsqa"),
		Excluded:    &[]clientgen.Target{},
		Identifier:  "flagsTeam",
		Included:    &[]clientgen.Target{},
		Name:        "flagsTeam",
		Rules: &[]clientgen.Clause{
			{
				Attribute: "ip",
				Id:        domain.ToPtr("31c18ee7-8051-44cc-8507-b44580467ee5"),
				Negate:    false,
				Op:        "equal",
				Values:    []string{"2a00:23c5:b672:2401:158:f2a6:67a0:6a79"},
			},
		},
		Version:    domain.ToPtr(int64(1)),
		CreatedAt:  domain.ToPtr(int64(123)),
		ModifiedAt: domain.ToPtr(int64(456)),
	}

	targetFoo = domain.Target{
		Target: clientgen.Target{
			Account:     "foo",
			Anonymous:   domain.ToPtr(false),
			CreatedAt:   domain.ToPtr(int64(1634222520273)),
			Environment: "featureflagsqa",
			Identifier:  "foo",
			Name:        "foo",
			Org:         "bar",
			Project:     "FeatureFlagsQADemo",
			Segments:    &[]clientgen.Segment{},
			Attributes: &map[string]interface{}{
				"age": float64(56),
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

	targetJames = domain.Target{
		Target: clientgen.Target{
			Account:     "",
			CreatedAt:   domain.ToPtr(int64(1634222520273)),
			Environment: "featureflagsqa",
			Identifier:  "james",
			Name:        "james",
			Org:         "",
			Project:     "FeatureFlagsQADemo",
			Segments:    &[]clientgen.Segment{},
			Attributes: &map[string]interface{}{
				"age": float64(55),
				"ages": []interface{}{
					float64(1),
					float64(2),
					float64(3),
				},
				"happy":       true,
				"host":        "file:///Users/jcox/github.com/drone/ff-javascript-client-sample/index.html?identifier=james",
				"lastUpdated": "Thu Oct 21 2021 11:58:20 GMT+0100 (British Summer Time)",
				"userGroups":  []interface{}{"Foo", "Volvo", "BMW"},
			},
		},
	}
)

func TestConfig_Populate(t *testing.T) {
	expectedAuthConfig := []domain.AuthConfig{
		{
			EnvironmentID: domain.EnvironmentID("1234"),
			APIKey:        domain.NewAuthAPIKey("d4f79b313f8106f5af108ad96ff516222dbfd5a0ab52f4308e4b1ad1d740de60"),
		},
		{
			EnvironmentID: domain.EnvironmentID("1234"),
			APIKey:        domain.NewAuthAPIKey("15fac8fa1c99022568b008b9df07b04b45354ac5ca4740041d904cd3cf2b39e3"),
		},
		{
			EnvironmentID: domain.EnvironmentID("1234"),
			APIKey:        domain.NewAuthAPIKey("35ab1e0411c4cc6ecaaa676a4c7fef259798799ed40ad09fb07adae902bd0c7a"),
		},
	}

	expectedFlagConfig := []domain.FlagConfig{
		{
			EnvironmentID: "1234",
			FeatureConfigs: []domain.FeatureFlag{
				harnessAppDemoDarkModeConfig,
				yetAnotherFlagConfig,
			},
		},
	}

	expectedSegmentConfig := []domain.SegmentConfig{
		{
			EnvironmentID: "1234",
			Segments:      []domain.Segment{flagsTeamSegment},
		},
	}

	type mocks struct {
		authRepo    *mockAuthRepo
		flagRepo    *mockFlagRepo
		segmentRepo *mockSegmentRepo
	}

	type expected struct {
		authConfig    []domain.AuthConfig
		flagConfig    []domain.FlagConfig
		segmentConfig []domain.SegmentConfig
	}

	testCases := map[string]struct {
		mocks     mocks
		shouldErr bool

		expected expected
	}{
		"Given I call Populate and the authRepo errors adding config to the cache": {
			mocks: mocks{
				authRepo: &mockAuthRepo{
					add: func(ctx context.Context, config ...domain.AuthConfig) error {
						return errors.New("an error")
					},
				},
				flagRepo:    &mockFlagRepo{},
				segmentRepo: &mockSegmentRepo{},
			},
			shouldErr: true,
		},
		"Given I call Populate and the flagRepo errors adding config to the cache": {
			mocks: mocks{
				authRepo: &mockAuthRepo{
					add: func(ctx context.Context, config ...domain.AuthConfig) error {
						return nil
					},
				},
				flagRepo: &mockFlagRepo{
					add: func(ctx context.Context, config ...domain.FlagConfig) error {
						return errors.New("an error")
					},
				},
				segmentRepo: &mockSegmentRepo{},
			},
			shouldErr: true,

			expected: expected{
				authConfig:    expectedAuthConfig,
				flagConfig:    nil,
				segmentConfig: nil,
			},
		},
		"Given I call Populate and the segmentRepo errors adding config to the cache": {
			mocks: mocks{
				authRepo: &mockAuthRepo{
					add: func(ctx context.Context, config ...domain.AuthConfig) error {
						return nil
					},
				},
				flagRepo: &mockFlagRepo{
					add: func(ctx context.Context, config ...domain.FlagConfig) error {
						return nil
					},
				},
				segmentRepo: &mockSegmentRepo{
					add: func(ctx context.Context, config ...domain.SegmentConfig) error {
						return errors.New("an error")
					},
				},
			},
			shouldErr: true,

			expected: expected{
				authConfig:    expectedAuthConfig,
				flagConfig:    expectedFlagConfig,
				segmentConfig: nil,
			},
		},
		"Given I call Populate and none of the repos error": {
			mocks: mocks{
				authRepo: &mockAuthRepo{
					add: func(ctx context.Context, config ...domain.AuthConfig) error {
						return nil
					},
				},
				flagRepo: &mockFlagRepo{
					add: func(ctx context.Context, config ...domain.FlagConfig) error {
						return nil
					},
				},
				segmentRepo: &mockSegmentRepo{
					add: func(ctx context.Context, config ...domain.SegmentConfig) error {
						return nil
					},
				},
			},
			shouldErr: false,

			expected: expected{
				authConfig:    expectedAuthConfig,
				flagConfig:    expectedFlagConfig,
				segmentConfig: expectedSegmentConfig,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c, err := NewConfig(testConfig)
			assert.Nil(t, err)

			err = c.Populate(context.Background(), tc.mocks.authRepo, tc.mocks.flagRepo, tc.mocks.segmentRepo)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.authConfig, tc.mocks.authRepo.config)
			assert.Equal(t, tc.expected.flagConfig, tc.mocks.flagRepo.config)
			assert.Equal(t, tc.expected.segmentConfig, tc.mocks.segmentRepo.config)
		})
	}
}
