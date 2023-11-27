package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	)

	token, _, err := testhelpers.AuthenticateSDKClient(GetServerAPIKey(), GetStreamURL(), nil)
	if err != nil {
		t.Error(err)
	}

	type args struct {
		org               string
		project           string
		editProxyKeyScope func(ctx context.Context, account string, identifier string, org string, project string) error
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
				org:     org2,
				project: project3Identifier,
			},
			expected: expected{
				statusCode:        http.StatusOK,
				numFeatureConfigs: 2,
			},
		},
		{
			name: "Given I remove the ProxyKeys access to the environment",
			args: args{
				org:               org2,
				project:           project3Identifier,
				editProxyKeyScope: removeEnvironment,
			},
			expected: expected{
				statusCode:        http.StatusOK,
				numFeatureConfigs: 0,
			},
		},
		{
			name: "Given I re-add the ProxyKeys access to the environment",
			args: args{
				org:               org2,
				project:           project3Identifier,
				editProxyKeyScope: addEnvironment,
			},
			expected: expected{
				statusCode:        http.StatusOK,
				numFeatureConfigs: 2,
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
				err := tc.args.editProxyKeyScope(ctx, testhelpers.GetDefaultAccount(), testhelpers.GetProxyKeyIdentifier(), org2, project3Identifier)
				assert.Nil(t, err)
			}

			validateFeatureConfigs := func(r *http.Response) bool {
				var (
					featureConfigsBody = bytes.NewBuffer([]byte{})
					featureConfigs     []client.FeatureConfig
				)

				_, err := io.Copy(featureConfigsBody, r.Body)
				assert.Nil(t, err)

				assert.Nil(t, jsoniter.Unmarshal(featureConfigsBody.Bytes(), &featureConfigs))

				return len(featureConfigs) == tc.expected.numFeatureConfigs
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
			defer resp.Body.Close()

			assert.Equal(t, tc.expected.statusCode, resp.StatusCode)
		})
	}
}

func parseTokenClaims(token string) (domain.Claims, error) {
	ss := strings.Split(token, ".")

	if len(ss) != 3 {
		return domain.Claims{}, errors.New("unexpected token length")
	}

	claims := domain.Claims{}
	if err := jsoniter.Unmarshal([]byte(ss[1]), &claims); err != nil {
		return domain.Claims{}, err
	}

	return claims, nil
}

// Removes access to the default environment and adds access to the secondary environment
func removeEnvironment(ctx context.Context, account string, identifier string, org string, project string) error {
	resp, err := testhelpers.GetProxyKey(ctx, account, identifier)
	if err != nil {
		return err
	}

	if resp.JSON200 == nil {
		if resp.JSON404 != nil {
			return nil
		}
		return fmt.Errorf("failed to get proxy key to edit it: %s", err)
	}

	updatedProject := resp.JSON200.Organizations.AdditionalProperties[org].Projects.AdditionalProperties[project]
	updatedProject.Scope = "selected"
	updatedProject.Environments = domain.ToPtr([]string{"secondEnv"})

	resp.JSON200.Organizations.AdditionalProperties[org].Projects.AdditionalProperties[project] = updatedProject

	body := admin.UpdateProxyKeyJSONRequestBody{
		Organizations: resp.JSON200.Organizations,
		Version:       resp.JSON200.Version + 1,
	}

	return testhelpers.EditProxyKey(ctx, account, identifier, body)
}

// Removes access to the secondary environment and adds access to the default environment
func addEnvironment(ctx context.Context, account string, identifier string, org string, project string) error {
	resp, err := testhelpers.GetProxyKey(ctx, account, identifier)
	if err != nil {
		return err
	}

	updatedProject := resp.JSON200.Organizations.AdditionalProperties[org].Projects.AdditionalProperties[project]

	updatedProject.Scope = "selected"
	updatedProject.Environments = domain.ToPtr([]string{testhelpers.GetDefaultEnvironment()})

	resp.JSON200.Organizations.AdditionalProperties[org].Projects.AdditionalProperties[project] = updatedProject

	body := admin.UpdateProxyKeyJSONRequestBody{
		Organizations: resp.JSON200.Organizations,
		Version:       resp.JSON200.Version + 1,
	}

	return testhelpers.EditProxyKey(ctx, account, identifier, body)
}
