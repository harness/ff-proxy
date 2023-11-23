package testhelpers

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/gen/admin"
)

// DefaultClient returns the default admin client
func DefaultClient() *admin.Client {
	client, err := admin.NewClient(GetAdminURL())

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

func AddProxyAuthToken(_ context.Context, req *http.Request) error {
	req.Header.Set("authorization", GetProxyAuthToken())
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

func DeleteProjectForOrg(identifier, org string) (*http.Response, error) {
	return DeleteProjectRemoteForOrg(identifier, org)
}

// DeleteProjectRemoteForOrg ...
func DeleteProjectRemoteForOrg(identifier, org string) (*http.Response, error) {
	client := DefaultClient()
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/projects/%s?accountIdentifier=%s&orgIdentifier=%s", GetPlatformBaseURL(), identifier, GetDefaultAccount(), org), nil)
	if err != nil {
		return nil, err
	}
	AddAuthToken(context.Background(), req)
	return client.Client.Do(req)
}

func DeleteProxyKey(ctx context.Context, account, keyIdentifier string) error {
	c := DefaultClient()
	identifier := admin.Identifier(keyIdentifier)
	params := &admin.DeleteProxyKeyParams{
		AccountIdentifier: admin.AccountQueryParam(account),
	}

	res, err := c.DeleteProxyKey(ctx, identifier, params, AddAuthToken)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	resBody, _ := ioutil.ReadAll(res.Body)
	response := string(resBody)
	fmt.Println(response)
	return nil

}

func CreateProxyKey(ctx context.Context, projectIdentifier, account string, org string, identifier string, environments []string) (string, error) {
	c := DefaultClient()

	params := &admin.CreateProxyKeyParams{
		AccountIdentifier: admin.AccountQueryParam(account),
	}

	body := admin.CreateProxyKeyJSONRequestBody{
		Identifier: identifier,
		Name:       identifier,
		Organizations: admin.OrganizationDictionary{
			AdditionalProperties: map[string]admin.ProjectDictionary{
				org: {
					Projects: &admin.ProjectDictionary_Projects{
						AdditionalProperties: map[string]admin.ProxyKeyProject{
							projectIdentifier: {
								Environments: &environments,
								Scope:        "selected",
							},
						},
					},
				},
			},
		},
	}

	resp, err := c.CreateProxyKey(ctx, params, body, AddAuthToken)
	if err != nil {
		return "", err
	}

	p, err := admin.ParseCreateProxyKeyResponse(resp)
	if err != nil {
		return "", err
	}

	if p.JSON201 == nil {
		return "", fmt.Errorf("non 200 response creating ProxyKey: %v", p.StatusCode())
	}

	if p.JSON201.Key == nil {
		return "", errors.New("nil proxy key returned")
	}

	return *p.JSON201.Key, nil
}

func CreateProxyKeyForMultipleOrgs(ctx context.Context, keyIdentifier, account, org1, org2, project1, project2, emptyProject string) (string, error) {
	c := DefaultClient()

	params := &admin.CreateProxyKeyParams{
		AccountIdentifier: admin.AccountQueryParam(account),
	}

	body := admin.CreateProxyKeyJSONRequestBody{
		Identifier: keyIdentifier,
		Name:       keyIdentifier,
		Organizations: admin.OrganizationDictionary{
			AdditionalProperties: map[string]admin.ProjectDictionary{
				org1: {
					Projects: &admin.ProjectDictionary_Projects{
						AdditionalProperties: map[string]admin.ProxyKeyProject{
							project1: {
								Scope:        "selected",
								Environments: domain.ToPtr([]string{GetDefaultEnvironment()}),
							},
							emptyProject: {
								Scope: "all",
							},
						},
					},
				},
				org2: {
					Projects: &admin.ProjectDictionary_Projects{
						AdditionalProperties: map[string]admin.ProxyKeyProject{
							project2: {
								Scope: "all",
							},
						},
					},
				},
			},
		},
	}

	resp, err := c.CreateProxyKey(ctx, params, body, AddAuthToken)
	if err != nil {
		return "", err
	}

	p, err := admin.ParseCreateProxyKeyResponse(resp)
	if err != nil {
		return "", err
	}

	if p.JSON201 == nil {
		return "", fmt.Errorf("non 200 response creating ProxyKey: %v", p.StatusCode())
	}

	if p.JSON201.Key == nil {
		return "", errors.New("nil proxy key returned")
	}

	return *p.JSON201.Key, nil
}
