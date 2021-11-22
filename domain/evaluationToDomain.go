package domain

import (
	"github.com/harness/ff-golang-server-sdk/evaluation"
	clientgen "github.com/harness/ff-proxy/gen/client"
)

// Converts an evaluation.FeatureFlag which the sdk cache sends to a clientgen.FeatureFlag for our internal use
// Converts an evaluation.Segment which the sdk cache sends to a clientgen.Segment for our internal use

func convertEvaluationWeightedVariation(wv evaluation.WeightedVariation) *clientgen.WeightedVariation {
	return &clientgen.WeightedVariation{
		Variation: wv.Variation,
		Weight:    wv.Weight,
	}
}

func convertEvaluationDistribution(d *evaluation.Distribution) *clientgen.Distribution {
	if d == nil {
		return nil
	}
	vars := make([]clientgen.WeightedVariation, len(d.Variations))
	for i, val := range d.Variations {
		vars[i] = *convertEvaluationWeightedVariation(val)
	}
	return &clientgen.Distribution{
		BucketBy:   d.BucketBy,
		Variations: vars,
	}
}

func convertEvaluationVariation(v evaluation.Variation) *clientgen.Variation {
	return &clientgen.Variation{
		Description: v.Description,
		Identifier:  v.Identifier,
		Name:        v.Name,
		Value:       v.Value,
	}
}

func convertEvaluationServe(s evaluation.Serve) *clientgen.Serve {
	return &clientgen.Serve{
		Distribution: convertEvaluationDistribution(s.Distribution),
		Variation:    s.Variation,
	}
}

func convertEvaluationClause(c evaluation.Clause) *clientgen.Clause {
	return &clientgen.Clause{
		Attribute: c.Attribute,
		Id:        c.ID,
		Negate:    c.Negate,
		Op:        c.Op,
		Values:    c.Value,
	}
}

func convertEvaluationServingRule(r evaluation.ServingRule) *clientgen.ServingRule {
	clauses := make([]clientgen.Clause, len(r.Clauses))
	for i, val := range r.Clauses {
		clauses[i] = *convertEvaluationClause(val)
	}
	return &clientgen.ServingRule{
		Clauses:  clauses,
		Priority: r.Priority,
		RuleId:   r.RuleID,
		Serve:    *convertEvaluationServe(r.Serve),
	}
}

func convertEvaluationPrerequisite(p evaluation.Prerequisite) *clientgen.Prerequisite {
	return &clientgen.Prerequisite{
		Feature:    p.Feature,
		Variations: p.Variations,
	}
}

//convert converts variation map to evaluation object
func convertEvaluationVariationMap(v evaluation.VariationMap) *clientgen.VariationMap {
	return &clientgen.VariationMap{
		TargetSegments: &v.TargetSegments,
		Targets:        convertEvaluationTargetToTargetMap(v.Targets),
		Variation:      v.Variation,
	}
}

func convertEvaluationTargetToTargetMap(targets []string) *[]clientgen.TargetMap {
	result := make([]clientgen.TargetMap, 0, len(targets))
	for j := range targets {
		result = append(result, clientgen.TargetMap{
			Identifier: &targets[j],
		})
	}
	return &result
}

// ConvertEvaluationFeatureConfig - Convert evaluation feature config to domain object
func ConvertEvaluationFeatureConfig(fc evaluation.FeatureConfig) *FeatureFlag {
	vars := make([]clientgen.Variation, len(fc.Variations))
	for i, val := range fc.Variations {
		vars[i] = *convertEvaluationVariation(val)
	}

	var rules []clientgen.ServingRule
	if fc.Rules != nil {
		rules = make([]clientgen.ServingRule, len(fc.Rules))
		for i, val := range fc.Rules {
			rules[i] = *convertEvaluationServingRule(val)
		}
	}

	var pre []clientgen.Prerequisite
	if fc.Prerequisites != nil {
		pre = make([]clientgen.Prerequisite, len(fc.Prerequisites))
		for i, val := range fc.Prerequisites {
			pre[i] = *convertEvaluationPrerequisite(val)
		}
	}
	defaultServe := clientgen.Serve{}
	if fc.DefaultServe.Distribution != nil {
		defaultServe.Distribution = convertEvaluationDistribution(fc.DefaultServe.Distribution)
	}
	if fc.DefaultServe.Variation != nil {
		defaultServe.Variation = fc.DefaultServe.Variation
	}
	var vtm []clientgen.VariationMap
	if fc.VariationToTargetMap != nil {
		vtm = make([]clientgen.VariationMap, len(fc.VariationToTargetMap))
		for i, val := range fc.VariationToTargetMap {
			vtm[i] = *convertEvaluationVariationMap(val)
		}
	}

	return &FeatureFlag{
		FeatureConfig: clientgen.FeatureConfig{
			DefaultServe:         defaultServe,
			Environment:          fc.Environment,
			Feature:              fc.Feature,
			Kind:                 fc.Kind,
			OffVariation:         fc.OffVariation,
			Prerequisites:        &pre,
			Project:              fc.Project,
			Rules:                &rules,
			State:                clientgen.FeatureState(fc.State),
			VariationToTargetMap: &vtm,
			Variations:           vars,
		},
	}
}

// ConvertEvaluationSegment - Convert evaluation segment domain segment object
func ConvertEvaluationSegment(s evaluation.Segment) *Segment {
	excluded := make([]clientgen.Target, 0)
	if s.Excluded != nil {
		excluded = make([]clientgen.Target, len(s.Excluded))
		for i, excl := range s.Excluded {
			excluded[i] = clientgen.Target{
				Identifier: excl,
			}
		}
	}

	included := make([]clientgen.Target, 0)
	if s.Included != nil {
		included = make([]clientgen.Target, len(s.Included))
		for i, incl := range s.Included {
			included[i] = clientgen.Target{Identifier: incl}
		}
	}

	rules := make([]clientgen.Clause, 0)
	if s.Rules != nil {
		rules = make([]clientgen.Clause, len(s.Rules))
		for i, rule := range s.Rules {
			rules[i] = clientgen.Clause{
				Attribute: rule.Attribute,
				Id:        rule.ID,
				Negate:    rule.Negate,
				Op:        rule.Op,
				Values:    rule.Value,
			}
		}
	}

	tags := make([]clientgen.Tag, 0)
	if s.Rules != nil {
		if s.Tags != nil {
			tags = make([]clientgen.Tag, len(s.Tags))
			for i, tag := range s.Tags {
				tags[i] = clientgen.Tag{
					Name:  tag.Name,
					Value: tag.Value,
				}
			}
		}
	}

	return &Segment{
		clientgen.Segment{
			Identifier:  s.Identifier,
			Name:        s.Name,
			CreatedAt:   s.CreatedAt,
			ModifiedAt:  s.ModifiedAt,
			Environment: s.Environment,
			Excluded:    &excluded,
			Included:    &included,
			Rules:       &rules,
			Tags:        &tags,
			Version:     &s.Version,
		},
	}
}
