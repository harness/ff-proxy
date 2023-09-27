package remote

import (
	"context"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/services"
)

// clientService defines the interface for interacting with the ff-client-service
type clientService interface {
	AuthenticateProxyKey(ctx context.Context, key string) (services.AuthenticateProxyKeyResponse, error)
	PageProxyConfig(ctx context.Context, input services.GetProxyConfigInput) ([]domain.ProxyConfig, error)
}

// Config is the type that fetches config from Harness SaaS
type Config struct {
	key string

	clientService clientService
}

// NewConfig creates a new Config
func NewConfig(key string, cs clientService) Config {
	c := Config{
		key:           key,
		clientService: cs,
	}
	return c
}

// Populate populates repositories with the config
func (c Config) Populate(ctx context.Context) error {
	authResp, err := authenticate(c.key, c.clientService)
	if err != nil {
		return err
	}

	proxyConfig, err := retrieveConfig(c.key, authResp.Token, authResp.ClusterIdentifier, c.clientService)
	if err != nil {
		return err
	}

	// TODO: Next PR we'll store this config in the cache
	_ = proxyConfig

	return nil
}

func authenticate(key string, cs clientService) (services.AuthenticateProxyKeyResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cs.AuthenticateProxyKey(ctx, key)
	if err != nil {
		return services.AuthenticateProxyKeyResponse{}, err
	}

	return resp, nil
}

func retrieveConfig(key string, authToken string, clusterIdentifier string, cs clientService) ([]domain.ProxyConfig, error) {
	if clusterIdentifier == "" {
		clusterIdentifier = "1"
	}
	input := services.GetProxyConfigInput{
		Key:               key,
		EnvID:             "",
		AuthToken:         authToken,
		ClusterIdentifier: clusterIdentifier,
		PageNumber:        0,
		PageSize:          10,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return cs.PageProxyConfig(ctx, input)
}
