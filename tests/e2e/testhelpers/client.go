package testhelpers

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/golang-jwt/jwt"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/gen/client"
)

// AuthenticateSDKClient performs an auth request and returns the token and environment to use
func AuthenticateSDKClient(key string, url string, target *client.Target) (token string, claims domain.Claims, err error) {
	serverToken, err := Authenticate(key, url, target)
	if err != nil {
		return "", domain.Claims{}, err
	}

	if serverToken.StatusCode() != 200 {
		return "", domain.Claims{}, fmt.Errorf("unable to authenticate client with key %s", key)
	}
	serverClaims, err := DecodeClaims(serverToken.JSON200.AuthToken)
	return serverToken.JSON200.AuthToken, serverClaims, err
}

// Authenticate is wrapper around the rest client for the Auth API
func Authenticate(apiKey string, url string, target *client.Target) (*client.AuthenticateResponse, error) {
	c := DefaultEvaluationClient(url)
	resp, err := c.Authenticate(context.Background(), authRequest(apiKey, target))
	if err != nil {
		return nil, err
	}
	return client.ParseAuthenticateResponse(resp)
}

// DefaultEvaluationClient creates a default client for the evaluation service
func DefaultEvaluationClient(url string) *client.Client {
	log.Infof("Connecting client to %s", url)
	c, err := client.NewClient(url)
	if err != nil {
		return nil
	}
	return c
}

// DecodeClaims ...
func DecodeClaims(tokenString string) (domain.Claims, error) {
	claims := domain.Claims{}
	token, _ := jwt.Parse(tokenString, nil)
	if token == nil {
		return claims, fmt.Errorf("JWT token could not be parsed")
	}
	return marshallClaims(token.Claims)
}

// authRequest creates a JSON auth request body
func authRequest(apiKey string, target *client.Target) client.AuthenticateJSONRequestBody {
	req := client.AuthenticateJSONRequestBody{ApiKey: apiKey}

	// Add target if supplied
	if target != nil {
		req.Target = &struct {
			Anonymous  *bool                   `json:"anonymous,omitempty"`
			Attributes *map[string]interface{} `json:"attributes,omitempty"`
			Identifier string                  `json:"identifier"`
			Name       *string                 `json:"name,omitempty"`
		}{
			Anonymous:  target.Anonymous, // always false
			Attributes: target.Attributes,
			Identifier: target.Identifier,
		}
		// If target.Name s not empty set it
		if target.Name != "" {
			req.Target.Name = &target.Name
		}
	}

	return req
}

func marshallClaims(claims jwt.Claims) (domain.Claims, error) {
	if err := claims.Valid(); err != nil {
		return domain.Claims{}, err
	}

	str, err := json.Marshal(claims)
	if err != nil {
		return domain.Claims{}, fmt.Errorf("failed to marshal client claims: %s", err)
	}

	clientClaims := domain.Claims{}
	err = json.Unmarshal(str, &clientClaims)
	if err != nil {
		return domain.Claims{}, fmt.Errorf("failed to unmarshall claims into custom client claims: %s", err)
	}

	return clientClaims, nil
}
