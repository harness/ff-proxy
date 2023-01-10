package proxyservice

import (
	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
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
func (f FeatureEvaluator) Evaluate(target domain.Target, configs ...domain.FeatureConfig) ([]clientgen.Evaluation, error) {
	evaluations := []clientgen.Evaluation{}
	//for _, c := range configs {
	//	evaluationConfig := domain.ConvertDomainFeatureConfig(c)
	//
	//	evaluation, err := evaluationConfig.Evaluate(&evaluation.Target{
	//		Identifier: target.Identifier,
	//		Name:       target.Name,
	//		Anonymous:  target.Anonymous,
	//		Attributes: target.Attributes,
	//	})
	//	if err != nil {
	//		return nil, err
	//	}
	//
	//	clientgenEvaluation := clientgen.Evaluation{
	//		Flag:       evaluation.Flag,
	//		Identifier: &evaluation.Variation.Identifier,
	//		Kind:       string(c.Kind),
	//		Value:      evaluation.Variation.Value,
	//	}
	//	evaluations = append(evaluations, clientgenEvaluation)
	//}
	return evaluations, nil
}
