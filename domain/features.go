package domain

import (
	"fmt"

	"github.com/harness/ff-golang-server-sdk/rest"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	jsoniter "github.com/json-iterator/go"
)

// FeatureFlagKey is the key that maps to a FeatureConfig
type FeatureFlagKey string

// NewFeatureConfigKey creates a FeatureFlagKey from an environment and identifier
func NewFeatureConfigKey(envID string, identifier string) FeatureFlagKey {
	return FeatureFlagKey(fmt.Sprintf("env-%s-feature-config-%s", envID, identifier))
}

// NewFeatureConfigsKey creates a FeatureFlagKey from and environmet
func NewFeatureConfigsKey(envID string) FeatureFlagKey {
	return FeatureFlagKey(fmt.Sprintf("env-%s-feature-configs", envID))
}

// FeatureConfig is the type containing FeatureConfig information and is what
// we return from /GET client/env/<env>/feature-configs
type FeatureConfig struct {
	FeatureFlag
}

// MarshalBinary marshals a FeatureFlag to bytes. Currently it just uses json
// marshaling but if we want to optimise storage space we could use something
// more efficient
func (f *FeatureFlag) MarshalBinary() ([]byte, error) {
	return jsoniter.Marshal(f)
}

// UnmarshalBinary unmarshals bytes to a FeatureFlag
func (f *FeatureFlag) UnmarshalBinary(b []byte) error {
	return jsoniter.Unmarshal(b, f)
}

// FeatureFlag stores feature flag data
type FeatureFlag clientgen.FeatureConfig

func (f *FeatureFlag) ToSDKFeatureConfig() rest.FeatureConfig {
	prereqs := toSDKPrereqs(f.Prerequisites)
	variations := toSDKVariations(f.Variations)
	vtms := toSDKVariationMap(f.VariationToTargetMap)
	rules := toSDKRules(f.Rules)

	return rest.FeatureConfig{
		DefaultServe:         toSDKServe(f.DefaultServe),
		Environment:          f.Environment,
		Feature:              f.Feature,
		Kind:                 rest.FeatureConfigKind(f.Kind),
		OffVariation:         f.OffVariation,
		Prerequisites:        &prereqs,
		Project:              f.Project,
		Rules:                &rules,
		State:                rest.FeatureState(f.State),
		VariationToTargetMap: &vtms,
		Variations:           variations,
		Version:              f.Version,
	}
}

func toSDKServe(serve clientgen.Serve) rest.Serve {
	distribution := SafePtrDereference(serve.Distribution)

	return rest.Serve{
		Distribution: &rest.Distribution{
			BucketBy:   distribution.BucketBy,
			Variations: toSDKWeightedVariations(distribution.Variations),
		},
		Variation: serve.Variation,
	}
}

func toSDKRules(rules *[]clientgen.ServingRule) []rest.ServingRule {
	if rules == nil {
		return []rest.ServingRule{}
	}

	rulesCopy := *rules
	result := make([]rest.ServingRule, 0, len(*rules))
	for i := 0; i < len(*rules); i++ {
		r := rulesCopy[i]

		var distribution *rest.Distribution
		if r.Serve.Distribution != nil {
			distribution = &rest.Distribution{
				BucketBy:   r.Serve.Distribution.BucketBy,
				Variations: toSDKWeightedVariations(r.Serve.Distribution.Variations),
			}
		}

		clauses := toSDKClause(&r.Clauses)

		result = append(result, rest.ServingRule{
			Clauses:  clauses,
			Priority: r.Priority,
			RuleId:   r.RuleId,
			Serve: rest.Serve{
				Distribution: distribution,
				Variation:    r.Serve.Variation,
			},
		})
	}

	return result
}

func toSDKWeightedVariations(weightedVariations []clientgen.WeightedVariation) []rest.WeightedVariation {
	result := make([]rest.WeightedVariation, 0, len(weightedVariations))

	for _, wv := range weightedVariations {
		result = append(result, rest.WeightedVariation{
			Variation: wv.Variation,
			Weight:    wv.Weight,
		})
	}

	return result
}

func toSDKClause(clauses *[]clientgen.Clause) []rest.Clause {
	if clauses == nil {
		return []rest.Clause{}
	}

	results := make([]rest.Clause, 0, len(*clauses))
	for _, c := range *clauses {
		results = append(results, rest.Clause{
			Attribute: c.Attribute,
			Id:        c.Id,
			Negate:    c.Negate,
			Op:        c.Op,
			Values:    c.Values,
		})
	}

	return results
}

func toGroupServingRules(s *[]clientgen.GroupServingRule) []rest.GroupServingRule {
	if s == nil {
		return []rest.GroupServingRule{}
	}

	results := make([]rest.GroupServingRule, 0, len(*s))
	for i := range *s {
		v := (*s)[i]
		results = append(results, rest.GroupServingRule{
			Clauses:  toSDKClause(&v.Clauses),
			Priority: v.Priority,
			RuleId:   v.RuleId,
		})
	}
	return results
}

func toSDKPrereqs(prerequisite *[]clientgen.Prerequisite) []rest.Prerequisite {
	if prerequisite == nil {
		return []rest.Prerequisite{}
	}

	prereqs := make([]rest.Prerequisite, 0, len(*prerequisite))
	for _, prereq := range *prerequisite {
		prereqs = append(prereqs, rest.Prerequisite{
			Feature:    prereq.Feature,
			Variations: prereq.Variations,
		})
	}

	return prereqs
}

func toSDKVariations(variations []clientgen.Variation) []rest.Variation {
	result := make([]rest.Variation, 0, len(variations))

	for _, v := range variations {
		result = append(result, rest.Variation{
			Description: v.Description,
			Identifier:  v.Identifier,
			Name:        v.Name,
			Value:       v.Value,
		})
	}

	return result
}

func toSDKVariationMap(variationMap *[]clientgen.VariationMap) []rest.VariationMap {
	if variationMap == nil {
		return []rest.VariationMap{}
	}

	vtms := make([]rest.VariationMap, 0, len(*variationMap))

	variationMapCopy := *variationMap
	for i := 0; i < len(*variationMap); i++ {
		v := variationMapCopy[i]

		if v.Targets == nil {
			continue
		}

		targetsCopy := *v.Targets
		targetMap := make([]rest.TargetMap, 0, len(*v.Targets))

		for i := 0; i < len(*v.Targets); i++ {
			t := targetsCopy[i]

			targetMap = append(targetMap, rest.TargetMap{
				Identifier: t.Identifier,
				Name:       t.Name,
			})
		}

		vtms = append(vtms, rest.VariationMap{
			TargetSegments: v.TargetSegments,
			Targets:        &targetMap,
			Variation:      v.Variation,
		})
	}

	return vtms
}
