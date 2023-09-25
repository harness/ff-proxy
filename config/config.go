package config

import (
	"context"
)

// Config defines the interface for populating repositories with configuration data
type Config interface {
	// Populate populates the repos with the config
	Populate(ctx context.Context) error
}
