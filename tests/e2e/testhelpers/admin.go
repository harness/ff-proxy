package testhelpers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/harness/ff-proxy/v2/gen/admin"
)

// DefaultClient returns the default admin client
func DefaultClient() *admin.Client {
	client, err := admin.NewClient(GetClientURL())

	if err != nil {
		return nil
	}
	return client
}

// AddAuthToken adds the appropriate token to the request.  If we are using
// a JWT it sets the authorization header, and if were using a PAT/SAT it sets
// the x-api-key header
func AddAuthToken(ctx context.Context, req *http.Request) error {
	if GetUserAccessToken() != "" {
		req.Header.Set("x-api-key", GetAuthToken())
	} else {
		req.Header.Set("authorization", GetAuthToken())
	}

	return nil
}

// DeleteProject ...
func DeleteProject(identifier string) (*http.Response, error) {
	return DeleteProjectRemote(identifier)
}

// DeleteProjectRemote ...
func DeleteProjectRemote(identifier string) (*http.Response, error) {
	client := DefaultClient()
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/projects/%s?accountIdentifier=%s&orgIdentifier=%s", GetPlatformBaseURL(), identifier, GetDefaultAccount(), GetDefaultOrg()), nil)
	if err != nil {
		return nil, err
	}
	AddAuthToken(context.Background(), req)
	return client.Client.Do(req)
}
