package domain

import (
	"github.com/google/uuid"

	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

// ProxyConfig is the object that we receive from SaaS containing the config
// for each environment associated with a ProxyKey
type ProxyConfig struct {
	Environments []Environments `json:"environments"`
}

// Environments contains the environment config that the Proxy needs to store
type Environments struct {
	ID             uuid.UUID     `json:"id"`
	APIKeys        []string      `json:"apiKeys"`
	FeatureConfigs []FeatureFlag `json:"featureConfigs"`
	Segments       []Segment     `json:"segments"`
}

// ToProxyConfig is a helper for converting from the generated ProxyConfig type to our domain ProxyConfig type
func ToProxyConfig(c clientgen.ProxyConfig) ProxyConfig {
	if c.Environments == nil || *c.Environments == nil {
		return ProxyConfig{}
	}

	environments := make([]Environments, 0, len(*c.Environments))
	for _, env := range *c.Environments {
		e := Environments{}

		if env.Id != nil {
			e.ID = uuid.MustParse(*env.Id)
		}

		if *env.ApiKeys != nil {
			e.APIKeys = *env.ApiKeys
		}

		if *env.FeatureConfigs != nil {
			e.FeatureConfigs = make([]FeatureFlag, 0, len(*env.FeatureConfigs))

			for _, fc := range *env.FeatureConfigs {
				e.FeatureConfigs = append(e.FeatureConfigs, FeatureFlag(fc))
			}
		}

		if *env.Segments != nil {
			e.Segments = make([]Segment, 0, len(*env.Segments))
			for _, s := range *env.Segments {
				e.Segments = append(e.Segments, Segment(s))
			}
		}

		environments = append(environments, e)
	}

	return ProxyConfig{Environments: environments}
}

type FlagConfig struct {
	EnvironmentID  string
	FeatureConfigs []FeatureFlag
}

type SegmentConfig struct {
	EnvironmentID string
	Segments      []Segment
}
