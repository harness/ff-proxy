package domain

import "github.com/harness/ff-golang-server-sdk/evaluation"

// ConvertTarget converts types.Target to the evaluation.Target
func ConvertTarget(t Target) evaluation.Target {

	target := evaluation.Target{
		Identifier: t.Identifier,
		Name:       t.Name,
		Anonymous:  t.Anonymous,
		Attributes: t.Attributes,
	}
	return target
}
