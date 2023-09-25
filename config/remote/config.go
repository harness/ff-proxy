package remote

import (
	"context"
	"time"

	"github.com/harness/ff-proxy/v2/services"
)

// clientService defines the interface for interacting with the ff-client-service
type clientService interface {
	AuthenticateProxyKey(ctx context.Context, key string) (services.AuthenticateProxyKeyResponse, error)
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

	// TODO: AuthResp will be used to fetch config
	_ = authResp

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
