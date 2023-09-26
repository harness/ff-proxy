package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/harness/ff-proxy/v2/token"
	jsoniter "github.com/json-iterator/go"
)

var (
	ErrNotFound     = errors.New("ErrNotFound")
	ErrUnauthorized = errors.New("ErrUnauthorized")
	ErrInternal     = errors.New("ErrInternal")
)

// ClientService is a type for interacting with the Feature Flag Client Service
type ClientService struct {
	log    log.Logger
	client clientgen.ClientWithResponsesInterface
}

// NewClientService creates a ClientService
func NewClientService(l log.Logger, addr string) (ClientService, error) {
	l = l.With("component", "ClientServiceClient")

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

// AuthenticateProxyKeyResponse is the type returned by AuthenticateProxyKey
type AuthenticateProxyKeyResponse struct {
	Token             string
	ClusterIdentifier string
}

// AuthenticateProxyKey makes an auth request to the ff-client-service's /proxy/auth endpoint
func (c ClientService) AuthenticateProxyKey(ctx context.Context, key string) (AuthenticateProxyKeyResponse, error) {
	req := clientgen.AuthenticateProxyKeyJSONRequestBody{ProxyKey: key}

	resp, err := c.client.AuthenticateProxyKeyWithResponse(ctx, req)
	if err != nil {
		return AuthenticateProxyKeyResponse{}, err
	}

	if resp.JSON200 == nil {
		maskedKey := token.MaskRight(key)

		switch resp.StatusCode() {
		case http.StatusInternalServerError:
			return AuthenticateProxyKeyResponse{}, fmt.Errorf("%w: recevied 500 from Harness SaaS authenticating ProxyKey: %s", ErrInternal, maskedKey)
		case http.StatusNotFound:
			return AuthenticateProxyKeyResponse{}, fmt.Errorf("%w: received 404 from SaaS authenticating ProxyKey: %s", ErrNotFound, maskedKey)
		case http.StatusUnauthorized:
			return AuthenticateProxyKeyResponse{}, fmt.Errorf("%w: received unauthorised response from SaaS authenticatin ProxyKey: %s", ErrUnauthorized, maskedKey)
		case http.StatusForbidden:
			return AuthenticateProxyKeyResponse{}, fmt.Errorf("%w: received forbidden response from SaaS authenticating ProxyKey: %s", ErrUnauthorized, maskedKey)

		default:
			return AuthenticateProxyKeyResponse{}, fmt.Errorf("%w: unexpected error authenticatin proxy key: %s", ErrInternal, maskedKey)
		}
	}

	claims, err := decodeToken(resp.JSON200.AuthToken)
	if err != nil {
		return AuthenticateProxyKeyResponse{}, err
	}

	return AuthenticateProxyKeyResponse{Token: resp.JSON200.AuthToken, ClusterIdentifier: claims.ClusterIdentifier}, nil
}

type tokenClaims struct {
	ClusterIdentifier string `json:"cluster_identifier"`
}

func decodeToken(token string) (tokenClaims, error) {
	tc := tokenClaims{}

	tokenSegments := strings.Split(token, ".")
	if len(tokenSegments) < 3 {
		return tokenClaims{}, errors.New("received invalid token from SaaS")
	}

	payloadData, err := jwt.DecodeSegment(tokenSegments[1])
	if err != nil {
		return tokenClaims{}, err
	}

	if err = jsoniter.Unmarshal(payloadData, &tc); err != nil {
		return tokenClaims{}, err
	}

	return tc, nil
}
