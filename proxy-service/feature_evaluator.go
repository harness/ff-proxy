package proxyservice

import (
	"github.com/drone/ff-golang-server-sdk/evaluation"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/gen"
)

// FeatureEvaluator is a type that can evaluate a feature config for a given target
type FeatureEvaluator struct {
	// TODO: Could store a cache of the evaluations so as we don't have to perform
	// them everytime. Need to figure out how we signal that an evaluation is stale
}

// NewFeatureEvaluator creates and returns a FeatureEvaluator
func NewFeatureEvaluator() FeatureEvaluator {
	return FeatureEvaluator{}
}

// Evaluate evaluates featureConfig(s) against a target and returns an evaluation
func (f FeatureEvaluator) Evaluate(target domain.Target, configs ...domain.FeatureConfig) ([]gen.Evaluation, error) {
	evaluations := []gen.Evaluation{}
	for _, c := range configs {
		evaluationConfig := toEvaluationFeatureConfig(c)

		evaluation, err := evaluationConfig.Evaluate(&evaluation.Target{
			Identifier: target.Identifier,
			Name:       target.Name,
			Anonymous:  target.Anonymous,
			Attributes: target.Attributes,
		})
		if err != nil {
			return nil, err
		}

		genEvaluation := gen.Evaluation{
			Flag:       evaluation.Flag,
			Identifier: &evaluation.Variation.Identifier,
			Kind:       string(c.Kind),
			Value:      evaluation.Variation.Value,
		}
		evaluations = append(evaluations, genEvaluation)
	}
	return evaluations, nil
}

// toEvaluationFeatureConfig converts a domain.FeatureConfig to an evaluation.FeatureConfig
// which lets us reuse the evaluation logic from the go server sdk. It's not ideal
// having to do this but there doesn't seem to be another way since we don't have once
// consistent FeatureConfig that we can use across the ff-server, ff-go-sdk and ff-proxy
func toEvaluationFeatureConfig(f domain.FeatureConfig) evaluation.FeatureConfig {
	return evaluation.FeatureConfig{
		DefaultServe:         serve(&f.DefaultServe),
		Environment:          f.Environment,
		Feature:              f.Feature,
		Kind:                 string(f.Kind),
		OffVariation:         f.OffVariation,
		Prerequisites:        toEvaluationPrerequisite(f.Prerequisites),
		Project:              f.Project,
		Rules:                toEvaluationRules(f.Rules),
		State:                evaluation.FeatureState(f.State),
		VariationToTargetMap: toEvaluationVariationMap(f.VariationToTargetMap),
		Variations:           toEvaluationVariation(f.Variations),
		Segments:             toEvaluationSegments(f.Segments),
	}
}

// toEvaluationsRules converts *[]gen.ServingRule to []evaluation.ServingRule
func toEvaluationRules(r *[]gen.ServingRule) []evaluation.ServingRule {
	if r == nil {
		return []evaluation.ServingRule{}
	}

	rules := make([]evaluation.ServingRule, len(*r))
	for i, val := range *r {

		clauses := make([]evaluation.Clause, len(val.Clauses))
		for k, clause := range val.Clauses {
			clauses[k] = evaluation.Clause{
				Attribute: clause.Attribute,
				ID:        clause.Id,
				Negate:    clause.Negate,
				Op:        string(clause.Op),
				Value:     clause.Values,
			}
		}
		rules[i] = evaluation.ServingRule{
			Clauses:  clauses,
			Priority: int(val.Priority),
			RuleID:   val.RuleId,
			Serve:    serve(&val.Serve),
		}
	}
	return rules
}

// toEvaluationsPrerequisite converts *[]gen.Prerequisite to []evaluation.Prerequisite
func toEvaluationPrerequisite(p *[]gen.Prerequisite) []evaluation.Prerequisite {
	if p == nil {
		return []evaluation.Prerequisite{}
	}

	prerequisites := make([]evaluation.Prerequisite, len(*p))
	for i, val := range *p {
		prerequisites[i] = evaluation.Prerequisite{
			Feature:    val.Feature,
			Variations: val.Variations,
		}
	}
	return prerequisites
}

// toEvaluationVariationMap converts *[]gen.VariationMap to []evaluation.VariationMap
func toEvaluationVariationMap(vm *[]gen.VariationMap) []evaluation.VariationMap {
	if vm == nil {
		return []evaluation.VariationMap{}
	}

	variationMap := make([]evaluation.VariationMap, len(*vm))

	for i, val := range *vm {
		evaluationTargetMap := &evaluation.VariationMap{}
		evaluationTargetMap.Variation = val.Variation

		if val.Targets != nil {
			targets := make([]string, len(*val.Targets))
			for i, target := range *val.Targets {
				targets[i] = *target.Identifier
			}
			evaluationTargetMap.Targets = targets
		}

		if val.TargetSegments != nil {
			segments := make([]string, len(*val.TargetSegments))
			for i, segment := range *val.TargetSegments {
				segments[i] = segment
			}
			evaluationTargetMap.TargetSegments = segments
		}
		variationMap[i] = *evaluationTargetMap
	}
	return variationMap
}

// toEvaluationVariation converts []gen.VariationMap to []evaluation.Variation
func toEvaluationVariation(v []gen.Variation) []evaluation.Variation {
	if v == nil {
		return []evaluation.Variation{}
	}

	variations := make(evaluation.Variations, len(v))
	for i, val := range v {
		variations[i] = evaluation.Variation{
			Description: val.Description,
			Identifier:  val.Identifier,
			Name:        val.Name,
			Value:       val.Value,
		}
	}
	return variations
}

// toEvaluationSegments converts []gen.Segment to []evaluation.Segments
func toEvaluationSegments(s map[string]domain.Segment) evaluation.Segments {
	if s == nil {
		return make(map[string]*evaluation.Segment)
	}

	segments := evaluation.Segments{}
	for i, sp := range s {
		s := sp
		included := make([]string, len(*s.Included))
		for k, incl := range *s.Included {
			included[k] = incl.Identifier
		}

		excluded := make([]string, len(*s.Excluded))
		for k, incl := range *s.Excluded {
			excluded[k] = incl.Identifier
		}

		clauses := []evaluation.Clause{}
		if s.Rules != nil {
			for _, c := range *s.Rules {
				clauses = append(clauses, evaluation.Clause{
					Attribute: c.Attribute,
					ID:        c.Id,
					Negate:    c.Negate,
					Op:        string(c.Op),
					Value:     c.Values,
				})
			}
		}

		segment := evaluation.Segment{
			Identifier: s.Identifier,
			Name:       s.Name,
			Included:   included,
			Excluded:   excluded,
			Rules:      clauses,
		}
		segments[i] = &segment
	}
	return segments
}

// serve converts *gen.Server to an evaluation.Serve
func serve(serve *gen.Serve) evaluation.Serve {
	if serve == nil {
		return evaluation.Serve{}
	}

	newServe := evaluation.Serve{}
	if serve.Variation != nil {
		newServe.Variation = serve.Variation
	}

	if serve.Distribution != nil {
		vars := serve.Distribution.Variations
		variations := make([]evaluation.WeightedVariation, len(vars))
		for i, val := range vars {
			variations[i] = evaluation.WeightedVariation{
				Variation: val.Variation,
				Weight:    val.Weight,
			}
		}
		newServe.Distribution = &evaluation.Distribution{
			BucketBy:   serve.Distribution.BucketBy,
			Variations: variations,
		}
	}
	return newServe
}
