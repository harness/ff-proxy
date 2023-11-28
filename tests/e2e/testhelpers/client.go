package testhelpers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/harness/ff-proxy/v2/gen/admin"
	log "github.com/sirupsen/logrus"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/gen/client"
)

const (
	localCertFile = "/harness/tests/e2e/certs/cert.crt"
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

	//if this is the devspace project.
	if strings.Contains(url, "pr2") {
		return c
	}

	// if we're connecting in https mode we should trust the self-signed certs used by the tests
	if strings.Contains(url, "https") {
		c.Client = GetCertClient()
	}

	if err != nil {
		return nil
	}
	return c
}

// GetCertClient returns a custom http client which defines a certificate authority and trusts our certs from the /cert folder
// this avoids any errors when run locally and doesn't require the certs to be manually trusted on your machine
func GetCertClient() *http.Client {
	// Get the SystemCertPool, continue with an empty pool on error
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	// Read in the cert file
	certs, err := ioutil.ReadFile(localCertFile)
	if err != nil {
		log.Fatalf("Failed to append %q to RootCAs: %v", localCertFile, err)
	}

	// Append our cert to the system pool
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		log.Println("No certs appended, using system certs only")
	}

	// Trust the augmented cert pool in our client
	config := &tls.Config{
		RootCAs:    rootCAs,
		MinVersion: tls.VersionTLS12,
	}

	tr := &http.Transport{TLSClientConfig: config}

	client := &http.Client{Transport: tr}

	return client
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

func AuthenticateProxyKey(ctx context.Context, key string) (string, error) {

	clientURL := GetClientURL()
	c := DefaultEvaluationClient(clientURL)

	body := client.AuthenticateProxyKeyJSONRequestBody{
		ProxyKey: key,
	}

	resp, err := c.AuthenticateProxyKey(ctx, body, AddAuthToken)
	if err != nil {
		return "", err
	}

	r, err := client.ParseAuthenticateResponse(resp)
	if err != nil {
		return "", err
	}

	return r.JSON200.AuthToken, nil
}

func CreateProxyKeyAndAuth(ctx context.Context, projectIdentifier, account string, org string, identifier string, environments []string) (string, string, error) {
	key, err := CreateProxyKey(ctx, projectIdentifier, account, org, identifier, environments)
	if err != nil {
		return "", "", nil
	}

	token, err := AuthenticateProxyKey(ctx, key)
	if err != nil {
		return "", "", nil
	}

	return key, token, nil
}

func CreateProxyKeyAndAuthForMultipleOrgs(ctx context.Context, keyIdentifier string, projects []TestProject) (string, string, error) {

	account := projects[0].Account
	org1 := projects[0].Organization
	org2 := projects[1].Organization
	project1 := projects[0].ProjectIdentifier
	project2 := projects[1].ProjectIdentifier
	project3 := projects[2].ProjectIdentifier
	emptyProject := projects[3].ProjectIdentifier

	key, err := CreateProxyKeyForMultipleOrgs(ctx, keyIdentifier, account, org1, org2, project1, project2, project3, emptyProject)
	log.Infof("key : %s\n", key)
	if err != nil {
		return "", "", nil
	}

	log.Info("sleeping for 10s")
	time.Sleep(10 * time.Second)
	log.Info("attempting proxykey auth")
	token, err := AuthenticateProxyKey(ctx, key)
	if err != nil {
		return "", "", nil
	}

	return key, token, nil
}

func EditProxyKey(ctx context.Context, account string, identifier string, body admin.UpdateProxyKeyJSONRequestBody) error {
	c := DefaultClient()

	params := admin.UpdateProxyKeyParams{
		AccountIdentifier: admin.AccountQueryParam(account),
	}

	resp, err := c.UpdateProxyKey(ctx, admin.Identifier(identifier), &params, body, AddAuthToken)
	if err != nil {
		return err
	}

	p, err := admin.ParseUpdateProxyKeyResponse(resp)
	if err != nil {
		return err
	}

	if p.StatusCode() != http.StatusOK {
		return fmt.Errorf("non 200 response code updating ProxyKey: %s", p.StatusCode())
	}

	return nil
}

func GetProxyKey(ctx context.Context, account string, identifier string) (*admin.GetProxyKeyResponse, error) {
	c := DefaultClient()

	params := admin.GetProxyKeyParams{
		AccountIdentifier: admin.AccountQueryParam(account),
	}

	resp, err := c.GetProxyKey(ctx, admin.Identifier(identifier), &params, AddAuthToken)
	if err != nil {
		return nil, err
	}

	r, err := admin.ParseGetProxyKeyResponse(resp)
	if err != nil {
		return nil, err
	}

	if r.JSON200 == nil {
		return nil, fmt.Errorf("non 200 status code for GetProxyKey %d", r.StatusCode())
	}

	return r, nil
}
