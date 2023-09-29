package domain

import (
	"context"
)

// ClientService defines the interface for interacting with the ff-client-service
type ClientService interface {
	AuthenticateProxyKey(ctx context.Context, key string) (AuthenticateProxyKeyResponse, error)
	PageProxyConfig(ctx context.Context, input GetProxyConfigInput) ([]ProxyConfig, error)
}
