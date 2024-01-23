package config

import (
	"context"
	"fmt"
	"os"

	"github.com/harness/ff-proxy/v2/config/local"
	"github.com/harness/ff-proxy/v2/config/remote"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/stream"
)

// Config defines the interface for populating repositories with configuration data
type Config interface {
	// FetchAndPopulate authenticates, fetches and populates the config.
	FetchAndPopulate(ctx context.Context, inventoryRepo domain.InventoryRepo, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error

	// Populate populates the repos with the config
	Populate(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error

	// Key returns proxyKey
	Key() string

	// Token returns the authToken that the Config uses to communicate with Harness SaaS
	Token() string

	// RefreshToken refreshes the auth token that the Config uses for fetching env config
	RefreshToken() (string, error)

	// ClusterIdentifier returns the identifier of the cluster that the Config authenticated against
	ClusterIdentifier() string

	// SetProxyConfig sets the proxyConfig member
	SetProxyConfig(proxyConfig []domain.ProxyConfig)
}

// NewConfig creates either a local or remote config type that implements the Config interface
func NewConfig(offline bool, configDir string, proxyKey string, clientService domain.ClientService, stream stream.Stream) (Config, error) {
	if !offline {
		return remote.NewConfig(proxyKey, clientService, stream), nil
	}

	conf, err := local.NewConfig(os.DirFS(configDir))
	if err != nil {
		return nil, fmt.Errorf("failed to load local config: %s", err)
	}
	return conf, nil
}
