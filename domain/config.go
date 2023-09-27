package domain

import (
	"github.com/google/uuid"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

// ProxyConfig is the object that we receive from SaaS containing the config
// for each environment associated with a ProxyKey
type ProxyConfig struct {
	Environments []Environments `json:"environments" json:"-"`
}

// Environments contains the environment config that the Proxy needs to store
type Environments struct {
	ID             uuid.UUID                 `json:"id" json:"-"`
	ApiKeys        []string                  `json:"apiKeys"`
	FeatureConfigs []clientgen.FeatureConfig `json:"featureConfigs"`
	Segments       []clientgen.Segment       `json:"segments"`
}

// ToProxyConfig is a helper for converting from the generated ProxyConfig type to our domain ProxyConfig type
func ToProxyConfig(c clientgen.ProxyConfig) ProxyConfig {
	if *c.Environments == nil {
		return ProxyConfig{}
	}

	environments := make([]Environments, len(*c.Environments))
	for _, env := range *c.Environments {
		e := Environments{
			ID:             uuid.MustParse(*env.Id),
			ApiKeys:        *env.ApiKeys,
			FeatureConfigs: *env.FeatureConfigs,
			Segments:       *env.Segments,
		}

		environments = append(environments, e)
	}

	return ProxyConfig{Environments: environments}
}
