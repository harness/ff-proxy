package remote

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
)

// Config is the type that fetches config from Harness SaaS
type Config struct {
	key               string
	token             string
	clusterIdentifier string
	proxyConfig       []domain.ProxyConfig
	ClientService     domain.ClientService
}

// NewConfig creates a new Config
func NewConfig(key string, cs domain.ClientService) *Config {
	c := &Config{
		key:           key,
		ClientService: cs,
	}
	return c
}

// Token returns the authToken that the Config uses to communicate with Harness SaaS
func (c *Config) Token() string {
	return c.token
}

func (c *Config) RefreshToken() (string, error) {
	authResp, err := authenticate(c.key, c.ClientService)
	if err != nil {
		return "", err
	}

	c.token = authResp.Token
	return c.token, nil
}

// ClusterIdentifier returns the identifier of the cluster that the Config authenticated against
func (c *Config) ClusterIdentifier() string {
	if c.clusterIdentifier == "" {
		return "1"
	}
	return c.clusterIdentifier
}

// Key returns proxyKey
func (c *Config) Key() string {
	return c.key
}

// SetProxyConfig sets the proxy config member
func (c *Config) SetProxyConfig(proxyConfig []domain.ProxyConfig) {
	c.proxyConfig = proxyConfig
}

// FetchAndPopulate Fetches and populates repositories with the config
func (c *Config) FetchAndPopulate(ctx context.Context, inventory domain.InventoryRepo, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {

	authResp, err := authenticate(c.key, c.ClientService)
	if err != nil {
		return err
	}
	c.token = authResp.Token
	c.clusterIdentifier = authResp.ClusterIdentifier

	proxyConfig, err := retrieveConfig(c.key, authResp.Token, authResp.ClusterIdentifier, c.ClientService)
	if err != nil {
		return err
	}

	// compare new and old config assets and delete difference.
	if err := inventory.Cleanup(ctx, c.key, proxyConfig); err != nil {
		return err
	}

	c.proxyConfig = proxyConfig
	return c.Populate(ctx, authRepo, flagRepo, segmentRepo)
}

// Populate populates repositories with the config
func (c *Config) Populate(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) error {
	for _, cfg := range c.proxyConfig {
		for _, env := range cfg.Environments {
			authConfig := make([]domain.AuthConfig, 0, len(env.APIKeys))
			apiKeys := make([]string, 0, len(env.APIKeys))

			for _, apiKey := range env.APIKeys {
				apiKeys = append(apiKeys, string(domain.NewAuthAPIKey(apiKey)))

				authConfig = append(authConfig, domain.AuthConfig{
					APIKey:        domain.NewAuthAPIKey(apiKey),
					EnvironmentID: domain.EnvironmentID(env.ID.String()),
				})

			}
			err := populate(ctx, authRepo, flagRepo, segmentRepo, apiKeys, authConfig, env)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// func extracted to satisfy lint complexity metrics.
func populate(ctx context.Context, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo, apiKeys []string, authConfig []domain.AuthConfig, env domain.Environments) error {

	// check for len is important to ensure we do not insert empty keys.
	// add apiKeys to cache.
	if len(apiKeys) > 0 {
		if err := authRepo.Add(ctx, authConfig...); err != nil {
			return fmt.Errorf("failed to add auth config to cache: %s", err)
		}
	}

	// add list of apiKeys for environment
	if len(authConfig) > 0 {
		if err := authRepo.AddAPIConfigsForEnvironment(ctx, env.ID.String(), apiKeys); err != nil {
			return fmt.Errorf("failed to add auth config to cache: %s", err)
		}
	}

	if len(env.FeatureConfigs) > 0 {
		if err := flagRepo.Add(ctx, domain.FlagConfig{
			EnvironmentID:  env.ID.String(),
			FeatureConfigs: env.FeatureConfigs,
		}); err != nil {
			return fmt.Errorf("failed to add flag config to cache: %s", err)
		}
	}
	if len(env.Segments) > 0 {
		if err := segmentRepo.Add(ctx, domain.SegmentConfig{
			EnvironmentID: env.ID.String(),
			Segments:      env.Segments,
		}); err != nil {
			return fmt.Errorf("failed to add segment config to cache: %s", err)
		}
	}
	return nil
}

func authenticate(key string, cs domain.ClientService) (domain.AuthenticateProxyKeyResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cs.AuthenticateProxyKey(ctx, key)
	if err != nil {
		return domain.AuthenticateProxyKeyResponse{}, err
	}

	return resp, nil
}

func retrieveConfig(key string, authToken string, clusterIdentifier string, cs domain.ClientService) ([]domain.ProxyConfig, error) {
	if clusterIdentifier == "" {
		clusterIdentifier = "1"
	}
	input := domain.GetProxyConfigInput{
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
