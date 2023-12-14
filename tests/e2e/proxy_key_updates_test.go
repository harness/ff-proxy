package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/gen/admin"
	"github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/tests/e2e/testhelpers"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

func TestProxyKeyUpdating(t *testing.T) {

	var (
		org2               = testhelpers.GetSecondaryOrg()
		project3Identifier = GetThirdProjectIdentifier()
		envID              = GetDefaultEnvironmentID()
		envIdentifier      = GetEnvironmentIdentifier()
		account            = testhelpers.GetDefaultAccount()
		proxyKeyIdentifier = testhelpers.GetProxyKeyIdentifier()
	)

	token, _, err := testhelpers.AuthenticateSDKClient(GetServerAPIKey(), GetStreamURL(), nil)
	if err != nil {
		t.Error(err)
	}

	// At the end of this test we want to reset the Proxy key's config back to its original
	// configuration so we don't break other tests
	defer func() {
		resp, err := testhelpers.GetProxyKey(context.Background(), account, proxyKeyIdentifier)
		if err != nil {
			t.Fatal(err)
		}

		if resp == nil {
			t.Fatal("a")
		}

		err = testhelpers.EditProxyKey(context.Background(), account, proxyKeyIdentifier, originalProxyConfig(resp.JSON200.Version))
		if err != nil {
			t.Fatal(err)
		}
	}()

	type args struct {
		org                string
		project            string
		envIdentifier      string
		proxyKeyIdentifier string
		editProxyKeyScope  func(ctx context.Context, account string, identifier string, org string, project string, envs ...string) error
	}

	type expected struct {
		statusCode        int
		numFeatureConfigs int
	}

	testCases := []struct {
		name     string
		args     args
		expected expected
	}{
		{
			name: "Given I have a ProxyKey scoped to an environment",
			args: args{
				org:                org2,
				project:            project3Identifier,
				envIdentifier:      envIdentifier,
				proxyKeyIdentifier: proxyKeyIdentifier,
			},
			expected: expected{
				statusCode:        http.StatusOK,
				numFeatureConfigs: 2,
			},
		},
		{
			name: "Given I remove the ProxyKeys access to the environment",
			args: args{
				org:                org2,
				project:            project3Identifier,
				envIdentifier:      testhelpers.GetSecondaryEnvironment(),
				proxyKeyIdentifier: proxyKeyIdentifier,
				editProxyKeyScope:  editScope,
			},
			expected: expected{
				statusCode:        http.StatusUnauthorized,
				numFeatureConfigs: 0,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			proxyClient := testhelpers.DefaultEvaluationClient(GetStreamURL())

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if tc.args.editProxyKeyScope != nil {
				err := tc.args.editProxyKeyScope(ctx, account, tc.args.proxyKeyIdentifier, tc.args.org, tc.args.project, tc.args.envIdentifier)
				assert.Nil(t, err)
			}

			validateFeatureConfigs := func(r *http.Response) bool {
				var (
					featureConfigsBody = bytes.NewBuffer([]byte{})
					featureConfigs     []client.FeatureConfig
				)

				if r.StatusCode == http.StatusUnauthorized && tc.expected.statusCode == http.StatusUnauthorized {
					return true
				}

				_, err := io.Copy(featureConfigsBody, r.Body)
				assert.Nil(t, err)

				assert.Nil(t, jsoniter.Unmarshal(featureConfigsBody.Bytes(), &featureConfigs))

				// If the featureConfigs slice isn't nil then that meant we got some
				// features back and can check if it matches the expected number of features
				if featureConfigs != nil {
					return len(featureConfigs) == tc.expected.numFeatureConfigs
				}

				// If it is nil then the only valid scenario is we weren't expecting
				// to get any feature configs in the response
				return tc.expected.numFeatureConfigs == 0
			}

			t.Log("When I make a /feature-configs request to the Proxy")
			resp, err := withRetry(
				validateFeatureConfigs,
				func() (*http.Response, error) {
					return proxyClient.GetFeatureConfig(ctx, envID, &client.GetFeatureConfigParams{}, func(ctx context.Context, req *http.Request) error {
						req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
						return nil
					})
				},
			)
			assert.Nil(t, err)

			assert.Equal(t, tc.expected.statusCode, resp.StatusCode)
		})
	}
}

func originalProxyConfig(version int) admin.PatchProxyKeyJSONRequestBody {
	var (
		defaultOrg     = GetOrgIdentifier()
		ffOrg          = testhelpers.GetSecondaryOrg()
		defaultProject = GetProjectIdentifier()
		secondProject  = GetSecondaryProjectIdentifier()
		defaultEnv     = testhelpers.GetDefaultEnvironment()

		thirdProject  = GetThirdProjectIdentifier()
		fourthProject = GetFourthProjectIdentifier()
	)

	config := admin.OrganizationDictionary{
		AdditionalProperties: map[string]admin.ProjectDictionary{
			ffOrg: admin.ProjectDictionary{
				Projects: &admin.ProjectDictionary_Projects{
					AdditionalProperties: map[string]admin.ProxyKeyProject{
						thirdProject: admin.ProxyKeyProject{
							Environments: domain.ToPtr([]string{defaultEnv}),
							Scope:        "selected",
						},
						secondProject: admin.ProxyKeyProject{
							Environments: nil,
							Scope:        "all",
						},
					},
				},
			},
			defaultOrg: admin.ProjectDictionary{
				Projects: &admin.ProjectDictionary_Projects{
					AdditionalProperties: map[string]admin.ProxyKeyProject{
						fourthProject: admin.ProxyKeyProject{
							Environments: nil,
							Scope:        "all",
						},
						defaultProject: admin.ProxyKeyProject{
							Environments: domain.ToPtr([]string{defaultEnv}),
							Scope:        "selected",
						},
					},
				},
			},
		},
	}

	return admin.PatchProxyKeyJSONRequestBody{
		Instructions: &struct {
			RotateKey    *string `json:"rotateKey,omitempty"`
			UpdateConfig *struct {
				Organizations admin.OrganizationDictionary `json:"organizations"`
				Version       int                          `json:"version"`
			} `json:"updateConfig,omitempty"`
			UpdateDescription *string `json:"updateDescription,omitempty"`
			UpdateName        *string `json:"updateName,omitempty"`
		}{
			UpdateConfig: &struct {
				Organizations admin.OrganizationDictionary `json:"organizations"`
				Version       int                          `json:"version"`
			}{
				Version:       version + 1,
				Organizations: config,
			},
		},
	}
}

// Removes access to the secondary environment and adds access to the default environment
func editScope(ctx context.Context, account string, identifier string, org string, project string, envs ...string) error {
	resp, err := testhelpers.GetProxyKey(ctx, account, identifier)
	if err != nil {
		return err
	}

	updatedProject := resp.JSON200.Organizations.AdditionalProperties[org].Projects.AdditionalProperties[project]

	updatedProject.Scope = "selected"
	updatedProject.Environments = domain.ToPtr(envs)

	resp.JSON200.Organizations.AdditionalProperties[org].Projects.AdditionalProperties[project] = updatedProject

	config := admin.OrganizationDictionary{
		AdditionalProperties: map[string]admin.ProjectDictionary{
			org: admin.ProjectDictionary{
				Projects: &admin.ProjectDictionary_Projects{
					AdditionalProperties: map[string]admin.ProxyKeyProject{
						project: updatedProject,
					},
				},
			},
		},
	}
	version := resp.JSON200.Version + 1

	body := createPatchRequestWithInstructions(version, config)

	return testhelpers.EditProxyKey(ctx, account, identifier, admin.PatchProxyKeyJSONRequestBody(body))
}

func createPatchRequestWithInstructions(version int, updateBody admin.OrganizationDictionary) admin.ProxyKeysPatchRequest {
	return admin.ProxyKeysPatchRequest{
		Instructions: &struct {
			RotateKey    *string `json:"rotateKey,omitempty"`
			UpdateConfig *struct {
				Organizations admin.OrganizationDictionary `json:"organizations"`
				Version       int                          `json:"version"`
			} `json:"updateConfig,omitempty"`
			UpdateDescription *string `json:"updateDescription,omitempty"`
			UpdateName        *string `json:"updateName,omitempty"`
		}{
			UpdateConfig: &struct {
				Organizations admin.OrganizationDictionary `json:"organizations"`
				Version       int                          `json:"version"`
			}{
				Version:       version,
				Organizations: updateBody,
			},
		},
	}
}
