package remote

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/v2/config"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/services"
	"golang.org/x/exp/slices"
)

// ClientService defines the interface for interacting with the ff-client-service
type ClientService interface {
	AuthenticateProxyKey(ctx context.Context, key string) (services.AuthenticateProxyKeyResponse, error)
	PageProxyConfig(ctx context.Context, input services.GetProxyConfigInput) ([]domain.ProxyConfig, error)
}

// Config is the type that fetches config from Harness SaaS
type Config struct {
	key string

	clientService ClientService
}

// NewConfig creates a new Config
func NewConfig(key string, cs ClientService) Config {
	c := Config{
		key:           key,
		clientService: cs,
	}
	return c
}

// Populate populates repositories with the config
func (c Config) Populate(ctx context.Context, authRepo config.AuthRepo, flagRepo config.FlagRepo, segmentRepo config.SegmentRepo) error {
	authResp, err := authenticate(c.key, c.clientService)
	if err != nil {
		return err
	}

	proxyConfig, err := retrieveConfig(c.key, authResp.Token, authResp.ClusterIdentifier, c.clientService)
	if err != nil {
		return err
	}

	var (
		authConfig    []domain.AuthConfig
		flagConfig    []domain.FlagConfig
		segmentConfig []domain.SegmentConfig
	)

	for _, cfg := range proxyConfig {

		for _, env := range cfg.Environments {
			slices.Grow(authConfig, len(env.ApiKeys))
			slices.Grow(flagConfig, len(env.FeatureConfigs))
			slices.Grow(segmentConfig, len(env.Segments))

			for _, apiKey := range env.ApiKeys {
				authConfig = append(authConfig, domain.AuthConfig{
					APIKey:        domain.AuthAPIKey(apiKey),
					EnvironmentID: domain.EnvironmentID(env.ID.String()),
				})
			}

			flagConfig = append(flagConfig, makeFlagConfig(env))
			segmentConfig = append(segmentConfig, makeSegmentConfig(env))
		}
	}

	if err := authRepo.Add(ctx, authConfig...); err != nil {
		return fmt.Errorf("failed to add auth config to cache: %s", err)
	}

	if err := flagRepo.Add(ctx, flagConfig...); err != nil {
		return fmt.Errorf("failed to add flag config to cache: %s", err)
	}

	if err := segmentRepo.Add(ctx, segmentConfig...); err != nil {
		return fmt.Errorf("failed to add segment config to cache: %s", err)
	}

	return nil
}

func authenticate(key string, cs ClientService) (services.AuthenticateProxyKeyResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := cs.AuthenticateProxyKey(ctx, key)
	if err != nil {
		return services.AuthenticateProxyKeyResponse{}, err
	}

	return resp, nil
}

func retrieveConfig(key string, authToken string, clusterIdentifier string, cs ClientService) ([]domain.ProxyConfig, error) {
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

func makeFlagConfig(env domain.Environments) domain.FlagConfig {
	featureConfigs := make([]domain.FeatureFlag, 0, len(env.FeatureConfigs))
	for _, flag := range env.FeatureConfigs {
		featureConfigs = append(featureConfigs, domain.FeatureFlag(flag))
	}

	return domain.FlagConfig{
		EnvironmentID:  env.ID.String(),
		FeatureConfigs: featureConfigs,
	}
}

func makeSegmentConfig(env domain.Environments) domain.SegmentConfig {
	segments := make([]domain.Segment, 0, len(env.Segments))
	for _, seg := range env.Segments {
		segments = append(segments, domain.Segment(seg))
	}

	return domain.SegmentConfig{
		EnvironmentID: env.ID.String(),
		Segments:      segments,
	}
}
