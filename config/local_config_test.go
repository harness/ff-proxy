package config

import (
	"embed"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/stretchr/testify/assert"
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

func strPtr(s string) *string { return &s }

func int64Ptr(i int64) *int64 { return &i }

func boolPtr(b bool) *bool { return &b }

var (
	harnessAppDemoDarkModeConfig = domain.FeatureFlag{
		DefaultServe: clientgen.Serve{
			Variation: strPtr("true"),
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
						Id:        toPtr("79f5bca0-17ca-42c2-8934-5cee840fe2e0"),
						Negate:    false,
						Op:        "equal",
						Values: []string{
							"55",
						},
					},
				},
				Priority: 1,
				RuleId:   toPtr("8756c207-abf8-4202-83fd-dedf5d27e2c2"),
				Serve: clientgen.Serve{
					Variation: strPtr("false"),
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

	yetAnotherFlagConfig = domain.FeatureFlag{
		DefaultServe: clientgen.Serve{
			Variation: strPtr("1"),
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
				Name:        strPtr("1"),
				Value:       "1",
			},
			{
				Description: nil,
				Identifier:  "2",
				Name:        strPtr("2"),
				Value:       "2",
			},
		},
		Version: int64Ptr(6),
	}

	flagsTeamSegment = domain.Segment{
		Environment: strPtr("featureflagsqa"),
		Excluded:    &[]clientgen.Target{},
		Identifier:  "flagsTeam",
		Included:    &[]clientgen.Target{},
		Name:        "flagsTeam",
		Rules: &[]clientgen.Clause{
			{
				Attribute: "ip",
				Id:        toPtr("31c18ee7-8051-44cc-8507-b44580467ee5"),
				Negate:    false,
				Op:        "equal",
				Values:    []string{"2a00:23c5:b672:2401:158:f2a6:67a0:6a79"},
			},
		},
		Version:    int64Ptr(1),
		CreatedAt:  int64Ptr(123),
		ModifiedAt: int64Ptr(456),
	}

	targetFoo = domain.Target{
		Target: clientgen.Target{
			Account:     "foo",
			Anonymous:   boolPtr(false),
			CreatedAt:   int64Ptr(1634222520273),
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
			CreatedAt:   int64Ptr(1634222520273),
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

func TestLocalConfig(t *testing.T) {
	expectedFeatureConfig := map[domain.FeatureFlagKey]interface{}{
		domain.NewFeatureConfigKey("1234", harnessAppDemoDarkModeConfig.Feature): harnessAppDemoDarkModeConfig,
		domain.NewFeatureConfigKey("1234", yetAnotherFlagConfig.Feature):         yetAnotherFlagConfig,
		domain.NewFeatureConfigsKey("1234"): []domain.FeatureFlag{
			harnessAppDemoDarkModeConfig,
			yetAnotherFlagConfig,
		},
	}

	expectedTargetConfig := map[domain.TargetKey]interface{}{
		domain.NewTargetKey("1234", "foo"):   targetFoo,
		domain.NewTargetKey("1234", "james"): targetJames,
		domain.NewTargetsKey("1234"): []domain.Target{
			targetFoo,
			targetJames,
		},
	}

	expectedSegments := map[domain.SegmentKey]interface{}{
		domain.NewSegmentKey("1234", flagsTeamSegment.Identifier): flagsTeamSegment,
		domain.NewSegmentsKey("1234"):                             []domain.Segment{flagsTeamSegment},
	}

	lc, err := NewLocalConfig(testConfig)
	if err != nil {
		t.Fatal(err)
	}

	actualFeatureConfig := lc.FeatureFlag()
	actualTargetConfig := lc.Targets()
	actualSegments := lc.Segments()

	assert.Equal(t, expectedFeatureConfig, actualFeatureConfig)
	assert.Equal(t, expectedTargetConfig, actualTargetConfig)
	assert.Equal(t, actualSegments, expectedSegments)
}

func TestLocalConfig_Auth(t *testing.T) {
	const (
		apikey1Hash = "15fac8fa1c99022568b008b9df07b04b45354ac5ca4740041d904cd3cf2b39e3"
		apikey2Hash = "35ab1e0411c4cc6ecaaa676a4c7fef259798799ed40ad09fb07adae902bd0c7a"
		apikey3Hash = "d4f79b313f8106f5af108ad96ff516222dbfd5a0ab52f4308e4b1ad1d740de60"
	)
	expected := map[domain.AuthAPIKey]string{
		domain.NewAuthAPIKey(apikey1Hash): "1234",
		domain.NewAuthAPIKey(apikey2Hash): "1234",
		domain.NewAuthAPIKey(apikey3Hash): "1234",
	}

	lc, err := NewLocalConfig(testConfig)
	if err != nil {
		t.Fatal(err)
	}

	actual := lc.AuthConfig()
	assert.Equal(t, expected, actual)
}

func TestLocalConfig_Segments(t *testing.T) {
	emptySegments := []domain.Segment{}

	segment := domain.Segment{Identifier: "hello"}
	segment2 := domain.Segment{Identifier: "world"}

	testCases := map[string]struct {
		localConfig LocalConfig
		expected    map[domain.SegmentKey]interface{}
	}{
		"Given I have a LocalConfig with nil segments": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment: "123",
						Segments:    nil,
					},
				},
			},

			expected: map[domain.SegmentKey]interface{}{},
		},
		"Given I have a LocalConfig with empty segments": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment: "123",
						Segments:    emptySegments,
					},
				},
			},

			expected: map[domain.SegmentKey]interface{}{},
		},
		"Given I have a LocalConfig with one segment": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment: "123",
						Segments:    []domain.Segment{segment},
					},
				},
			},

			expected: map[domain.SegmentKey]interface{}{
				domain.NewSegmentsKey("123"):                    []domain.Segment{segment},
				domain.NewSegmentKey("123", segment.Identifier): segment,
			},
		},
		"Given I have a LocalConfig with two segments": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment: "123",
						Segments:    []domain.Segment{segment, segment2},
					},
				},
			},

			expected: map[domain.SegmentKey]interface{}{
				domain.NewSegmentsKey("123"):                     []domain.Segment{segment, segment2},
				domain.NewSegmentKey("123", segment.Identifier):  segment,
				domain.NewSegmentKey("123", segment2.Identifier): segment2,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			actual := tc.localConfig.Segments()
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestLocalConfig_Features(t *testing.T) {
	emptyFeatures := []domain.FeatureFlag{}

	feature := domain.FeatureFlag{Feature: "hello"}
	feature2 := domain.FeatureFlag{Feature: "world"}

	testCases := map[string]struct {
		localConfig LocalConfig
		expected    map[domain.FeatureFlagKey]interface{}
	}{
		"Given I have a LocalConfig with nil features": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment:  "123",
						FeatureFlags: nil,
					},
				},
			},

			expected: map[domain.FeatureFlagKey]interface{}{},
		},
		"Given I have a LocalConfig with empty features": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment:  "123",
						FeatureFlags: emptyFeatures,
					},
				},
			},

			expected: map[domain.FeatureFlagKey]interface{}{},
		},
		"Given I have a LocalConfig with one feature": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment:  "123",
						FeatureFlags: []domain.FeatureFlag{feature},
					},
				},
			},

			expected: map[domain.FeatureFlagKey]interface{}{
				domain.NewFeatureConfigsKey("123"):                 []domain.FeatureFlag{feature},
				domain.NewFeatureConfigKey("123", feature.Feature): feature,
			},
		},
		"Given I have a LocalConfig with two features": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment:  "123",
						FeatureFlags: []domain.FeatureFlag{feature, feature2},
					},
				},
			},

			expected: map[domain.FeatureFlagKey]interface{}{
				domain.NewFeatureConfigsKey("123"):                  []domain.FeatureFlag{feature, feature2},
				domain.NewFeatureConfigKey("123", feature.Feature):  feature,
				domain.NewFeatureConfigKey("123", feature2.Feature): feature2,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			actual := tc.localConfig.FeatureFlag()
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestLocalConfig_Targets(t *testing.T) {
	emptyTarget := []domain.Target{}

	target := domain.Target{clientgen.Target{Identifier: "hello"}}
	target2 := domain.Target{clientgen.Target{Identifier: "world"}}

	testCases := map[string]struct {
		localConfig LocalConfig
		expected    map[domain.TargetKey]interface{}
	}{
		"Given I have a LocalConfig with nil targets": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment: "123",
						Targets:     nil,
					},
				},
			},

			expected: map[domain.TargetKey]interface{}{},
		},
		"Given I have a LocalConfig with empty targets": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment: "123",
						Targets:     emptyTarget,
					},
				},
			},

			expected: map[domain.TargetKey]interface{}{},
		},
		"Given I have a LocalConfig with one target": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment: "123",
						Targets:     []domain.Target{target},
					},
				},
			},

			expected: map[domain.TargetKey]interface{}{
				domain.NewTargetsKey("123"):                   []domain.Target{target},
				domain.NewTargetKey("123", target.Identifier): target,
			},
		},
		"Given I have a LocalConfig with two targets": {
			localConfig: LocalConfig{
				config: map[string]config{
					"123": {
						Environment: "123",
						Targets:     []domain.Target{target, target2},
					},
				},
			},

			expected: map[domain.TargetKey]interface{}{
				domain.NewTargetsKey("123"):                    []domain.Target{target, target2},
				domain.NewTargetKey("123", target.Identifier):  target,
				domain.NewTargetKey("123", target2.Identifier): target2,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			actual := tc.localConfig.Targets()
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func toPtr[T any](t T) *T {
	return &t
}
