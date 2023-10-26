package domain

import (
	"context"

	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

// ClientService defines the interface for interacting with the ff-client-service
type ClientService interface {
	AuthenticateProxyKey(ctx context.Context, key string) (AuthenticateProxyKeyResponse, error)
	PageProxyConfig(ctx context.Context, input GetProxyConfigInput) ([]ProxyConfig, error)
	FetchFeatureConfigForEnvironment(ctx context.Context, authToken, envId string) ([]clientgen.FeatureConfig, error)
}
