package ffproxy

import (
	"testing"

	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/gen"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func int64Ptr(i int64) *int64 { return &i }

func boolPtr(b bool) *bool { return &b }

var (
	harnessAppDemoDarkModeConfig = domain.FeatureConfig{
		FeatureConfig: gen.FeatureConfig{
			DefaultServe: gen.Serve{
				Variation: strPtr("true"),
			},
			Environment:   "featureflagsqa",
			Feature:       "harnessappdemodarkmode",
			Kind:          gen.FeatureConfigKindBoolean,
			OffVariation:  "false",
			Prerequisites: &[]gen.Prerequisite{},
			Project:       "FeatureFlagsQADemo",
			Rules: &[]gen.ServingRule{
				{
					Clauses: []gen.Clause{
						{
							Attribute: "age",
							Id:        "79f5bca0-17ca-42c2-8934-5cee840fe2e0",
							Negate:    false,
							Op:        "equal",
							Values: []string{
								"55",
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
		Segments: map[string]domain.Segment{
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

	yetAnotherFlagConfig = domain.FeatureConfig{
		FeatureConfig: gen.FeatureConfig{
			DefaultServe: gen.Serve{
				Variation: strPtr("1"),
			},
			Environment:   "featureflagsqa",
			Feature:       "yet_another_flag",
			Kind:          gen.FeatureConfigKindString,
			OffVariation:  "2",
			Prerequisites: &[]gen.Prerequisite{},
			Project:       "FeatureFlagsQADemo",
			Rules:         &[]gen.ServingRule{},
			State:         gen.FeatureStateOn,
			Variations: []gen.Variation{
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
		},
		Segments: map[string]domain.Segment{
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

	flagsTeamSegment = domain.Segment{
		Environment: strPtr("featureflagsqa"),
		Excluded:    &[]gen.Target{},
		Identifier:  "flagsTeam",
		Included:    &[]gen.Target{},
		Name:        "flagsTeam",
		Rules: &[]gen.Clause{
			{
				Attribute: "ip",
				Id:        "31c18ee7-8051-44cc-8507-b44580467ee5",
				Negate:    false,
				Op:        "equal",
				Values:    []string{"2a00:23c5:b672:2401:158:f2a6:67a0:6a79"},
			},
		},
		Version:    int64Ptr(1),
		CreatedAt:  int64Ptr(123),
		ModifiedAt: int64Ptr(456),
	}
)

func TestFeatureFlagConfig(t *testing.T) {
	expectedFeatureConfig := map[domain.FeatureConfigKey][]domain.FeatureConfig{
		domain.NewFeatureConfigKey("1234"): {
			harnessAppDemoDarkModeConfig,
			yetAnotherFlagConfig,
		},
	}

	expectedTargetConfig := map[domain.TargetKey][]domain.Target{
		domain.NewTargetKey("1234"): {
			{
				Account:     "foo",
				Anonymous:   boolPtr(false),
				CreatedAt:   int64Ptr(1634222520273),
				Environment: "featureflagsqa",
				Identifier:  "foo",
				Name:        "foo",
				Org:         "bar",
				Project:     "FeatureFlagsQADemo",
				Segments:    &[]gen.Segment{},
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
			{
				Account:     "",
				CreatedAt:   int64Ptr(1634222520273),
				Environment: "featureflagsqa",
				Identifier:  "james",
				Name:        "james",
				Org:         "",
				Project:     "FeatureFlagsQADemo",
				Segments:    &[]gen.Segment{},
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
		},
	}

	expectedSegments := map[domain.SegmentKey][]domain.Segment{
		domain.NewSegmentKey("1234"): []domain.Segment{flagsTeamSegment},
	}

	config, err := NewFeatureFlagConfig(testConfig, testDir)
	if err != nil {
		t.Fatal(err)
	}

	actualFeatureConfig := config.FeatureConfig()
	actualTargetConfig := config.Targets()
	actualSegments := config.Segments()

	assert.Equal(t, expectedFeatureConfig, actualFeatureConfig)
	assert.Equal(t, expectedTargetConfig, actualTargetConfig)
	assert.Equal(t, actualSegments, expectedSegments)
}
