package domain

import (
	"testing"

	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func Test_toSDKRules(t *testing.T) {

	t.Log("When my SDK rule has a nil distribution")
	rules := []clientgen.ServingRule{
		{
			Clauses:  nil,
			Priority: 0,
			RuleId:   nil,
			Serve: clientgen.Serve{
				Distribution: nil,
				Variation:    nil,
			},
		},
	}

	actual := toSDKRules(&rules)
	assert.Nil(t, actual[0].Serve.Distribution)

	t.Log("When my SDK rule has a non nil distribution")
	rules2 := []clientgen.ServingRule{
		{
			Clauses:  nil,
			Priority: 0,
			RuleId:   nil,
			Serve: clientgen.Serve{
				Distribution: &clientgen.Distribution{
					BucketBy:   "foo",
					Variations: nil,
				},
			},
		},
	}

	actual2 := toSDKRules(&rules2)
	assert.NotNil(t, actual2[0].Serve.Distribution)
}

func TestNewFeatureConfigKey(t *testing.T) {
	expected := FeatureFlagKey("env-123-feature-config-foo")
	actual := NewFeatureConfigKey("123", "foo")

	assert.Equal(t, expected, actual)
}

func TestNewFeatureConfigsKey(t *testing.T) {
	expected := FeatureFlagKey("env-123-feature-configs")
	actual := NewFeatureConfigsKey("123")

	assert.Equal(t, expected, actual)
}

func TestFeatureFlag_MarshalBinary(t *testing.T) {
	ff := FeatureFlag{Feature: "foo"}
	expected, err := jsoniter.Marshal(ff)
	assert.Nil(t, err)

	actual, err := ff.MarshalBinary()
	assert.Nil(t, err)

	assert.Equal(t, expected, actual)
}
