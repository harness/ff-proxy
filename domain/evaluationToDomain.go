package domain

import (
	"github.com/harness/ff-golang-server-sdk/evaluation"
	gen "github.com/harness/ff-proxy/gen/client"
)

// Converts an evaluation.FeatureConfig which the sdk cache sends to a gen.FeatureConfig for our internal use
// Converts an evaluation.Segment which the sdk cache sends to a gen.Segment for our internal use

func convertEvaluationWeightedVariation(wv evaluation.WeightedVariation) *gen.WeightedVariation {
	return &gen.WeightedVariation{
		Variation: wv.Variation,
		Weight:    wv.Weight,
	}
}

func convertEvaluationDistribution(d *evaluation.Distribution) *gen.Distribution {
	if d == nil {
		return nil
	}
	vars := make([]gen.WeightedVariation, len(d.Variations))
	for i, val := range d.Variations {
		vars[i] = *convertEvaluationWeightedVariation(val)
	}
	return &gen.Distribution{
		BucketBy:   d.BucketBy,
		Variations: vars,
	}
}

func convertEvaluationVariation(v evaluation.Variation) *gen.Variation {
	return &gen.Variation{
		Description: v.Description,
		Identifier:  v.Identifier,
		Name:        v.Name,
		Value:       v.Value,
	}
}

func convertEvaluationServe(s evaluation.Serve) *gen.Serve {
	return &gen.Serve{
		Distribution: convertEvaluationDistribution(s.Distribution),
		Variation:    s.Variation,
	}
}

func convertEvaluationClause(c evaluation.Clause) *gen.Clause {
	return &gen.Clause{
		Attribute: c.Attribute,
		Id:        c.ID,
		Negate:    c.Negate,
		Op:        c.Op,
		Values:     c.Value,
	}
}

func convertEvaluationServingRule(r evaluation.ServingRule) *gen.ServingRule {
	clauses := make([]gen.Clause, len(r.Clauses))
	for i, val := range r.Clauses {
		clauses[i] = *convertEvaluationClause(val)
	}
	return &gen.ServingRule{
		Clauses:  clauses,
		Priority: r.Priority,
		RuleId:   r.RuleID,
		Serve:    *convertEvaluationServe(r.Serve),
	}
}

func convertEvaluationPrerequisite(p evaluation.Prerequisite) *gen.Prerequisite {
	return &gen.Prerequisite{
		Feature:    p.Feature,
		Variations: p.Variations,
	}
}

//convert converts variation map to evaluation object
func convertEvaluationVariationMap(v evaluation.VariationMap) *gen.VariationMap {
	return &gen.VariationMap{
		TargetSegments: &v.TargetSegments,
		Targets:        convertEvaluationTargetToTargetMap(v.Targets),
		Variation:      v.Variation,
	}
}

func convertEvaluationTargetToTargetMap(targets []string) *[]gen.TargetMap {
	result := make([]gen.TargetMap, 0, len(targets))
	for j := range targets {
		result = append(result, gen.TargetMap{
			Identifier: &targets[j],
		})
	}
	return &result
}

// ConvertEvaluationFeatureConfig - Convert evaluation feature config to domain object
func ConvertEvaluationFeatureConfig(fc evaluation.FeatureConfig) *FeatureConfig {
	vars := make([]gen.Variation, len(fc.Variations))
	for i, val := range fc.Variations {
		vars[i] = *convertEvaluationVariation(val)
	}

	var rules []gen.ServingRule
	if fc.Rules != nil {
		rules = make([]gen.ServingRule, len(fc.Rules))
		for i, val := range fc.Rules {
			rules[i] = *convertEvaluationServingRule(val)
		}
	}

	var pre []gen.Prerequisite
	if fc.Prerequisites != nil {
		pre = make([]gen.Prerequisite, len(fc.Prerequisites))
		for i, val := range fc.Prerequisites {
			pre[i] = *convertEvaluationPrerequisite(val)
		}
	}
	defaultServe := gen.Serve{}
	if fc.DefaultServe.Distribution != nil {
		defaultServe.Distribution = convertEvaluationDistribution(fc.DefaultServe.Distribution)
	}
	if fc.DefaultServe.Variation != nil {
		defaultServe.Variation = fc.DefaultServe.Variation
	}
	var vtm []gen.VariationMap
	if fc.VariationToTargetMap != nil {
		vtm = make([]gen.VariationMap, len(fc.VariationToTargetMap))
		for i, val := range fc.VariationToTargetMap {
			vtm[i] = *convertEvaluationVariationMap(val)
		}
	}

	var segments map[string]Segment
	if fc.Segments != nil {
		segments = make(map[string]Segment, len(fc.Segments))
		for i, val := range fc.Segments {
			segments[i] = *ConvertEvaluationSegment(*val)
		}
	}

	return &FeatureConfig{
		FeatureConfig: gen.FeatureConfig{
			DefaultServe:         defaultServe,
			Environment:          fc.Environment,
			Feature:              fc.Feature,
			Kind:                 fc.Kind,
			OffVariation:         fc.OffVariation,
			Prerequisites:        &pre,
			Project:              fc.Project,
			Rules:                &rules,
			State:                gen.FeatureState(fc.State),
			VariationToTargetMap: &vtm,
			Variations:           vars,
		},
		Segments:  segments,
	}
}

// ConvertEvaluationSegment - Convert evaluation segment domain segment object
func ConvertEvaluationSegment(s evaluation.Segment) *Segment {
	excluded := make([]gen.Target, 0)
	if s.Excluded != nil {
		excluded = make([]gen.Target, len(s.Excluded))
		for i, excl := range s.Excluded {
			excluded[i] = gen.Target{
				Identifier:  excl,
			}
		}
	}

	included := make([]gen.Target, 0)
	if s.Included != nil {
		included = make([]gen.Target, len(s.Included))
		for i, incl := range s.Included {
			included[i] = gen.Target{Identifier: incl}
		}
	}

	rules := make([]gen.Clause, 0)
	if s.Rules != nil {
		rules = make([]gen.Clause, len(s.Rules))
		for i, rule := range s.Rules {
			rules[i] = gen.Clause{
				Attribute: rule.Attribute,
				Id:        rule.ID,
				Negate:    rule.Negate,
				Op:        rule.Op,
				Values:    rule.Value,
			}
		}
	}

	tags := make([]gen.Tag, 0)
	if s.Rules != nil {
		if s.Tags != nil {
			tags = make([]gen.Tag, len(s.Tags))
			for i, tag := range s.Tags {
				tags[i] = gen.Tag{
					Name:  tag.Name,
					Value: tag.Value,
				}
			}
		}
	}

	return &Segment{
		gen.Segment {
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
