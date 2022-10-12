package testhelpers

import (
	"context"
	"strconv"

	"github.com/harness/ff-proxy/gen/admin"
)

// APIKeyName ...
const (
	APIKeyTypeServer = "Server"
	APIKeyTypeClient = "Client"
	APIKeyExpiredAt  = ""
)

// AddAPIKey ...
func AddAPIKey(reqBody admin.AddAPIKeyJSONRequestBody, projectIdentifier string, envIdentifier string) (*admin.AddAPIKeyResponse, error) {
	client := DefaultClient()

	segment, err := client.AddAPIKey(context.Background(), &admin.AddAPIKeyParams{
		AccountIdentifier:     GetDefaultAccount(),
		OrgIdentifier:         GetDefaultOrg(),
		EnvironmentIdentifier: envIdentifier,
		ProjectIdentifier:     projectIdentifier,
	}, reqBody, AddAuthToken)

	if err != nil {
		return nil, err
	}
	return admin.ParseAddAPIKeyResponse(segment)
}

// GetAddAPIKeyBody ...
func GetAddAPIKeyBody(identifier string, apiKeyType string, name string, description string, expiredAt string) admin.AddAPIKeyJSONRequestBody {
	expiredAtInt, _ := strconv.Atoi(expiredAt)
	return admin.AddAPIKeyJSONRequestBody{
		Description: &description,
		ExpiredAt:   &expiredAtInt,
		Identifier:  identifier,
		Name:        name,
		Type:        admin.ApiKeyRequestType(apiKeyType),
	}
}
