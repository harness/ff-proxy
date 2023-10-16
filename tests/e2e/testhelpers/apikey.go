package testhelpers

import (
	"context"
	"strconv"

	admin "github.com/harness/ff-proxy/v2/gen/admin"
)

// APIKeyName ...
const (
	APIKeyTypeServer = "Server"
	APIKeyTypeClient = "Client"
	APIKeyExpiredAt  = ""
)

// AddAPIKey ...
func AddAPIKey(org string, reqBody admin.AddAPIKeyJSONRequestBody, projectIdentifier string, envIdentifier string) (*admin.AddAPIKeyResponse, error) {
	client := DefaultClient()

	segment, err := client.AddAPIKey(context.Background(), &admin.AddAPIKeyParams{
		AccountIdentifier:     admin.AccountQueryParam(GetDefaultAccount()),
		OrgIdentifier:         admin.OrgQueryParam(org),
		EnvironmentIdentifier: admin.EnvironmentQueryParam(envIdentifier),
		ProjectIdentifier:     admin.ProjectQueryParam(projectIdentifier),
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
