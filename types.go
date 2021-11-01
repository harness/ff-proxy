package ffproxy

import (
	"fmt"

	"github.com/harness/ff-proxy/gen"
)

// FeatureConfigKey is the key that maps to a FeatureConfig
type FeatureConfigKey string

// NewFeatureConfigKey creates a FeatureConfigKey from an environment
func NewFeatureConfigKey(envID string) FeatureConfigKey {
	return FeatureConfigKey(fmt.Sprintf("%s", envID))
}

// TargetKey is the key that maps to a Target
type TargetKey string

// NewTargetKey creates a TargetKey from an environment
func NewTargetKey(envID string) TargetKey {
	return TargetKey(fmt.Sprintf("%s", envID))
}

// FeatureConfig is the type containing FeatureConfig information
type FeatureConfig struct {
	gen.FeatureConfig
	Segments map[string]*gen.Segment
}
