package domain

import (
	"fmt"
)

// APIConfigsKey is the key that maps all APIKeys associated with environment.
type APIConfigsKey string

// NewAPIConfigsKey creates a APIConfigsKey from an environment and identifier. This key contains all the keys associated with the environment.
func NewAPIConfigsKey(envID string) APIConfigsKey {
	return APIConfigsKey(fmt.Sprintf("env-%s-api-configs", envID))
}
