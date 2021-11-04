package domain

import (
	"encoding/json"
	"fmt"

	"github.com/harness/ff-proxy/gen"
)

// FeatureConfigKey is the key that maps to a FeatureConfig
type FeatureConfigKey string

// NewFeatureConfigKey creates a FeatureConfigKey from an environment
func NewFeatureConfigKey(envID string) FeatureConfigKey {
	return FeatureConfigKey(fmt.Sprintf("env-%s-feature-config", envID))
}

// TargetKey is the key that maps to a Target
type TargetKey string

// NewTargetKey creates a TargetKey from an environment
func NewTargetKey(envID string) TargetKey {
	return TargetKey(fmt.Sprintf("env-%s-target-config", envID))
}

// FeatureConfig is the type containing FeatureConfig information and is what
// we return from /GET client/env/<env>/feature-configs
type FeatureConfig struct {
	gen.FeatureConfig
	Segments map[string]gen.Segment
}

// MarshalBinary marshals a FeatureConfig to bytes. Currently it just uses json
// marshaling but if we want to optimise storage space we could use something
// more efficient
func (f *FeatureConfig) MarshalBinary() ([]byte, error) {
	return json.Marshal(f)
}

// UnmarshalBinary unmarshals bytes to a FeatureConfig
func (f *FeatureConfig) UnmarshalBinary(b []byte) error {
	return json.Unmarshal(b, f)
}

// Target is an alias to gen.Target so we can make it implement the BinaryMarshaler
// interfaces
type Target gen.Target

// MarshalBinary marshals a Target to bytes. Currently it uses json marshaling
// but if we want to optimise storage space we could use something more efficient
func (t *Target) MarshalBinary() ([]byte, error) {
	return json.Marshal(t)
}

// UnmarshalBinary unmarshals bytes to a FeatureConfig
func (t *Target) UnmarshalBinary(b []byte) error {
	return json.Unmarshal(b, t)
}
