package services

import (
	"context"
	"fmt"

	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
	"github.com/harness/ff-proxy/log"
)

// ClientService is a type for interacting with the Feature Flag Client Service
type ClientService struct {
	log    log.Logger
	client clientgen.ClientWithResponsesInterface
}

// NewClientService creates a ClientService
func NewClientService(l log.Logger, addr string) (ClientService, error) {
	l = log.With(l, "component", "FF-ClientService-Client")

	client, err := clientgen.NewClientWithResponses(addr)
	if err != nil {
		return ClientService{}, err
	}

	return ClientService{log: l, client: client}, nil
}

// Authenticate makes an authentication request to the client service
func (c ClientService) Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error) {
	req := clientgen.AuthenticateJSONRequestBody{
		ApiKey: apiKey,
		Target: &struct {
			Anonymous  *bool                   `json:"anonymous,omitempty"`
			Attributes *map[string]interface{} `json:"attributes,omitempty"`
			Identifier string                  `json:"identifier"`
			Name       *string                 `json:"name,omitempty"`
		}{
			Anonymous:  target.Anonymous,
			Attributes: target.Attributes,
			Identifier: target.Identifier,
			Name:       &target.Name,
		},
	}

	resp, err := c.client.AuthenticateWithResponse(ctx, req)
	if err != nil {
		return "", err
	}

	if resp.JSON200 == nil {
		return "", fmt.Errorf("got non 200 response, status: %d, body: %s", resp.StatusCode(), resp.Body)
	}

	return resp.JSON200.AuthToken, nil
}
