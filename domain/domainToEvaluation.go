package domain

import (
	"github.com/harness/ff-golang-server-sdk/evaluation"
	clientgen "github.com/harness/ff-proxy/gen/client"
)

func convertGenWeightedVariation(wv clientgen.WeightedVariation) *evaluation.WeightedVariation {
	return &evaluation.WeightedVariation{
		Variation: wv.Variation,
		Weight:    wv.Weight,
	}
}

func convertGenDistribution(d *clientgen.Distribution) *evaluation.Distribution {
	if d == nil {
		return nil
	}
	vars := make([]evaluation.WeightedVariation, len(d.Variations))
	for i, val := range d.Variations {
		vars[i] = *convertGenWeightedVariation(val)
	}
	return &evaluation.Distribution{
		BucketBy:   d.BucketBy,
		Variations: vars,
	}
}

func convertGenVariation(v clientgen.Variation) *evaluation.Variation {
	return &evaluation.Variation{
		Description: v.Description,
		Identifier:  v.Identifier,
		Name:        v.Name,
		Value:       v.Value,
	}
}

func convertGenServe(s clientgen.Serve) *evaluation.Serve {
	return &evaluation.Serve{
		Distribution: convertGenDistribution(s.Distribution),
		Variation:    s.Variation,
	}
}

func convertGenClause(c clientgen.Clause) *evaluation.Clause {
	return &evaluation.Clause{
		Attribute: c.Attribute,
		ID:        c.Id,
		Negate:    c.Negate,
		Op:        c.Op,
		Value:     c.Values,
	}
}

func convertGenServingRule(r clientgen.ServingRule) *evaluation.ServingRule {
	clauses := make([]evaluation.Clause, len(r.Clauses))
	for i, val := range r.Clauses {
		clauses[i] = *convertGenClause(val)
	}
	return &evaluation.ServingRule{
		Clauses:  clauses,
		Priority: r.Priority,
		RuleID:   r.RuleId,
		Serve:    *convertGenServe(r.Serve),
	}
}

func convertGenPrerequisite(p clientgen.Prerequisite) *evaluation.Prerequisite {
	return &evaluation.Prerequisite{
		Feature:    p.Feature,
		Variations: p.Variations,
	}
}

//convert converts variation map to evaluation object
func convertGenVariationMap(v clientgen.VariationMap) *evaluation.VariationMap {
	return &evaluation.VariationMap{
		TargetSegments: *v.TargetSegments,
		Targets:        convertTargetToIdentifier(*v.Targets),
		Variation:      v.Variation,
	}
}

func convertTargetToIdentifier(tm []clientgen.TargetMap) []string {
	result := make([]string, 0, len(tm))
	for j := range tm {
		result = append(result, *tm[j].Identifier)
	}
	return result
}

// ConvertDomainFeatureConfig - Convert domain feature config to evaluation object
func ConvertDomainFeatureConfig(fc FeatureConfig) *evaluation.FeatureConfig {
	vars := make(evaluation.Variations, len(fc.Variations))
	for i, val := range fc.Variations {
		vars[i] = *convertGenVariation(val)
	}

	var rules evaluation.ServingRules
	if fc.Rules != nil {
		rules = make(evaluation.ServingRules, len(*fc.Rules))
		for i, val := range *fc.Rules {
			rules[i] = *convertGenServingRule(val)
		}
	}

	var pre []evaluation.Prerequisite
	if fc.Prerequisites != nil {
		pre = make([]evaluation.Prerequisite, len(*fc.Prerequisites))
		for i, val := range *fc.Prerequisites {
			pre[i] = *convertGenPrerequisite(val)
		}
	}
	defaultServe := evaluation.Serve{}
	if fc.DefaultServe.Distribution != nil {
		defaultServe.Distribution = convertGenDistribution(fc.DefaultServe.Distribution)
	}
	if fc.DefaultServe.Variation != nil {
		defaultServe.Variation = fc.DefaultServe.Variation
	}
	var vtm []evaluation.VariationMap
	if fc.VariationToTargetMap != nil {
		vtm = make([]evaluation.VariationMap, len(*fc.VariationToTargetMap))
		for i, val := range *fc.VariationToTargetMap {
			vtm[i] = *convertGenVariationMap(val)
		}
	}

	var segments  map[string]*evaluation.Segment
	if fc.Segments != nil {
		segments = make(map[string]*evaluation.Segment, len(fc.Segments))
		for i, val := range fc.Segments {
			segment := ConvertDomainSegment(val)
			segments[i] = &segment
		}
	}

	return &evaluation.FeatureConfig{
		DefaultServe:         defaultServe,
		Environment:          fc.Environment,
		Feature:              fc.Feature,
		Kind:                 string(fc.Kind),
		OffVariation:         fc.OffVariation,
		Prerequisites:        pre,
		Project:              fc.Project,
		Rules:                rules,
		State:                evaluation.FeatureState(fc.State),
		VariationToTargetMap: vtm,
		Variations:           vars,
		Segments:			  segments,
	}
}

// ConvertDomainSegment - Convert domain segment to evaluation segment object
func ConvertDomainSegment(s Segment) evaluation.Segment {
	excluded := make(evaluation.StrSlice, 0)
	if s.Excluded != nil {
		excluded = make(evaluation.StrSlice, len(*s.Excluded))
		for i, excl := range *s.Excluded {
			excluded[i] = excl.Identifier
		}
	}

	included := make(evaluation.StrSlice, 0)
	if s.Included != nil {
		included = make(evaluation.StrSlice, len(*s.Included))
		for i, incl := range *s.Included {
			included[i] = incl.Identifier
		}
	}

	rules := make(evaluation.SegmentRules, 0)
	if s.Rules != nil {
		rules = make(evaluation.SegmentRules, len(*s.Rules))
		for i, rule := range *s.Rules {
			rules[i] = evaluation.Clause{
				Attribute: rule.Attribute,
				ID:        rule.Id,
				Negate:    rule.Negate,
				Op:        rule.Op,
				Value:     rule.Values,
			}
		}
	}

	tags := make([]evaluation.Tag, 0)
	if s.Rules != nil {
		if s.Tags != nil {
			tags = make([]evaluation.Tag, len(*s.Tags))
			for i, tag := range *s.Tags {
				tags[i] = evaluation.Tag{
					Name:  tag.Name,
					Value: tag.Value,
				}
			}
		}
	}

	var version int64
	if s.Version != nil {
		version = *s.Version
	}
	return evaluation.Segment{
		Identifier:  s.Identifier,
		Name:        s.Name,
		CreatedAt:   s.CreatedAt,
		ModifiedAt:  s.ModifiedAt,
		Environment: s.Environment,
		Excluded:    excluded,
		Included:    included,
		Rules:       rules,
		Tags:        tags,
		Version:     version,
	}
}
