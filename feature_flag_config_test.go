package ffproxy

import (
	"embed"
	"testing"

	"github.com/harness/ff-proxy/gen"
	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func int64Ptr(i int64) *int64 { return &i }

func boolPtr(b bool) *bool { return &b }

//go:embed config/test/env-*
var testConfig embed.FS

const (
	testDir = "config/test"
)

func TestFeatureFlagConfig(t *testing.T) {
	expectedFeatureConfig := map[FeatureConfigKey][]*FeatureConfig{
		NewFeatureConfigKey("1234"): {
			{
				gen.FeatureConfig{
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
				map[string]*gen.Segment{
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
			},
		},
	}

	expectedTargetConfig := map[TargetKey][]*gen.Target{
		NewTargetKey("1234"): {
			{
				Account:     "foo",
				Anonymous:   boolPtr(false),
				CreatedAt:   int64Ptr(1634222520273),
				Environment: "featureflagsqa",
				Identifier:  "james",
				Name:        "james",
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
			},
		},
	}

	config, err := NewFeatureFlagConfig(testConfig, testDir)
	if err != nil {
		t.Fatal(err)
	}

	actualFeatureConfig := config.FeatureConfig()
	actualTargetConfig := config.Targets()

	assert.Equal(t, expectedFeatureConfig, actualFeatureConfig)
	assert.Equal(t, expectedTargetConfig, actualTargetConfig)
}
