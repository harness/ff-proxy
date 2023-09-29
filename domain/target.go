package domain

import (
	"fmt"

	"github.com/harness/ff-golang-server-sdk/evaluation"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	jsoniter "github.com/json-iterator/go"
)

// TargetKey is the key that maps to a Target
type TargetKey string

// NewTargetsKey creates a TargetKey from an environment
func NewTargetsKey(envID string) TargetKey {
	return TargetKey(fmt.Sprintf("env-%s-target-configs", envID))
}

// NewTargetKey creates a TargetKey from an environment and identifier
func NewTargetKey(envID string, identifier string) TargetKey {
	return TargetKey(fmt.Sprintf("env-%s-target-config-%s", envID, identifier))
}

// Target is a admingen.Target that we can declare methods on
type Target struct {
	clientgen.Target
}

// MarshalBinary marshals a Target to bytes. Currently it uses json marshaling
// but if we want to optimise storage space we could use something more efficient
func (t *Target) MarshalBinary() ([]byte, error) {
	return jsoniter.Marshal(t)
}

// UnmarshalBinary unmarshals bytes to a Target
func (t *Target) UnmarshalBinary(b []byte) error {
	return jsoniter.Unmarshal(b, t)
}

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
