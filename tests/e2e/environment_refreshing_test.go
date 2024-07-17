package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/harness/ff-proxy/v2/gen/admin"
	"github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/tests/e2e/testhelpers"
	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
)

// Tests that when we create an environment in a project with scope=all that the config
// gets sent to the Proxy and we can fetch flags for that env from the Proxy.
//
// Also tests that if we create an environment in a project with scope=selected that the
// config won't be sent to the Proxy
func TestEnvironmentCreation(t *testing.T) {
	var (
		orgTwo     = GetSecondaryOrgIdentifier()
		projectTwo = GetSecondaryProjectIdentifier() // Scope = all
	)

	type args struct {
		orgIdentifier     string
		projectIdentifier string
		envIdentifier     string
	}

	type expected struct {
		featureConfigsStatusCode int
		numFeatureConfigs        int

		segmentsStatusCode int
		numSegments        int
	}

	createEnvironment := func(identifier string, project string, org string, t *testing.T) string {
		resp, envID, err := testhelpers.CreateEnvironment(org, project, identifier, identifier)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		return envID
	}

	deleteEnvironment := func(identifier string, project string, org string, t *testing.T) {
		testhelpers.DeleteEnvironment(org, project, identifier)
	}

	createSDKKey := func(identifier string, project string, org string, env string, t *testing.T) string {
		var (
			keyResp *admin.AddAPIKeyResponse
			err     error
		)

		err = retry.Do(
			func() error {
				keyResp, err = testhelpers.AddAPIKey(
					org,
					admin.AddAPIKeyJSONRequestBody{
						Identifier: identifier,
						Type:       "Server",
						Name:       identifier,
					},
					project,
					env,
				)
				return err
			},
			retry.Attempts(5), retry.Delay(1000*time.Millisecond),
		)
		assert.Nil(t, err)
		assert.NotNil(t, keyResp.JSON201)
		return keyResp.JSON201.ApiKey
	}

	deleteSDKKey := func(identifier string, project string, org string, env string) {
		testhelpers.DeleteSDKKey(org, project, env, identifier)
	}

	testCases := map[string]struct {
		args     args
		expected expected
	}{
		"Given I create an environment in a Project where scope=all": {
			args: args{
				orgIdentifier:     orgTwo,
				envIdentifier:     "TestEnvironmentCreation_HappyPath",
				projectIdentifier: projectTwo,
			},
			expected: expected{
				featureConfigsStatusCode: http.StatusOK,
				numFeatureConfigs:        2,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			// Create an env and SDK key
			envID := createEnvironment(tc.args.envIdentifier, tc.args.projectIdentifier, tc.args.orgIdentifier, t)
			defer deleteEnvironment(tc.args.envIdentifier, tc.args.projectIdentifier, tc.args.orgIdentifier, t)

			sdkKey := createSDKKey("sdkkey", tc.args.projectIdentifier, tc.args.orgIdentifier, tc.args.envIdentifier, t)
			defer deleteSDKKey("sdkkey", tc.args.projectIdentifier, tc.args.orgIdentifier, tc.args.envIdentifier)

			var (
				token *client.AuthenticateResponse
				err   error
			)

			err = retry.Do(
				func() error {
					token, err = testhelpers.Authenticate(sdkKey, GetStreamURL(), nil)
					if token.StatusCode() != http.StatusOK {
						return errors.New("non 200")
					}
					return err
				},
				retry.Attempts(5), retry.Delay(2000*time.Millisecond),
			)
			assert.Nil(t, err)
			assert.NotNil(t, token.JSON200)

			proxyClient := testhelpers.DefaultEvaluationClient(GetStreamURL())

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			validateFeatureConfigs := func(r *http.Response) bool {
				var (
					featureConfigsBody = bytes.NewBuffer([]byte{})
					featureConfigs     []client.FeatureConfig
				)

				_, err = io.Copy(featureConfigsBody, r.Body)
				assert.Nil(t, err)

				assert.Nil(t, jsoniter.Unmarshal(featureConfigsBody.Bytes(), &featureConfigs))

				return len(featureConfigs) == tc.expected.numFeatureConfigs
			}

			t.Log("When I make a /feature-configs request to the Proxy")
			resp, err := withRetry(
				validateFeatureConfigs,
				func() (*http.Response, error) {
					return proxyClient.GetFeatureConfig(ctx, envID, &client.GetFeatureConfigParams{}, func(ctx context.Context, req *http.Request) error {
						req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.JSON200.AuthToken))
						return nil
					})
				},
			)
			assert.Nil(t, err)
			if resp.Body != nil {
				defer resp.Body.Close()
			}

			t.Logf("Then the returned status code will be %d", tc.expected.featureConfigsStatusCode)
			assert.Equal(t, tc.expected.featureConfigsStatusCode, resp.StatusCode)
		})
	}
}

type retryFn func() (*http.Response, error)

func withRetry(conditionFn func(r *http.Response) bool, fn retryFn) (*http.Response, error) {
	var resp *http.Response = &http.Response{}
	err := retry.Do(func() error {
		r, err := fn()
		if err != nil {
			return err
		}
		resp = r

		// If our conditional func doesn't return true then what we're trying to assert
		// must have failed so return an error to get us to retry
		if !conditionFn(r) {
			return errors.New("conditional func didn't evaluate to true")
		}

		return nil
	},
		retry.Attempts(5), retry.Delay(2000*time.Millisecond),
	)

	if resp.Body != nil {
		resp.Body.Close()
	}

	return resp, err
}

func TestEnvironmentDeletion(t *testing.T) {
	var (
		orgTwo        = GetSecondaryOrgIdentifier()
		projectTwo    = GetSecondaryProjectIdentifier() // Scope = all
		envIdentifier = "TestEnvironmentDeletion"
	)

	createEnvironment := func(identifier string, project string, org string, t *testing.T) string {
		resp, envID, err := testhelpers.CreateEnvironment(org, project, identifier, identifier)
		assert.Nil(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		return envID
	}

	deleteEnvironment := func(identifier string, project string, org string, t *testing.T) {
		testhelpers.DeleteEnvironment(org, project, identifier)
	}

	createSDKKey := func(identifier string, project string, org string, env string, t *testing.T) string {
		var (
			keyResp *admin.AddAPIKeyResponse
			err     error
		)

		err = retry.Do(
			func() error {
				keyResp, err = testhelpers.AddAPIKey(
					org,
					admin.AddAPIKeyJSONRequestBody{
						Identifier: identifier,
						Type:       "Server",
						Name:       identifier,
					},
					project,
					env,
				)
				return err
			},
			retry.Attempts(5), retry.Delay(1000*time.Millisecond),
		)
		assert.Nil(t, err)
		assert.NotNil(t, keyResp.JSON201)
		return keyResp.JSON201.ApiKey
	}

	deleteSDKKey := func(identifier string, project string, org string, env string) {
		testhelpers.DeleteSDKKey(org, project, env, identifier)
	}

	t.Run("When I Create an environment I should be able to fetch its Flag Config from the Proxy", func(t *testing.T) {
		// Create the environment
		envID := createEnvironment(envIdentifier, projectTwo, orgTwo, t)
		defer deleteEnvironment(envIdentifier, projectTwo, orgTwo, t)

		proxyClient := testhelpers.DefaultEvaluationClient(GetStreamURL())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		sdkKey := createSDKKey("sdkkey", projectTwo, orgTwo, envIdentifier, t)
		defer deleteSDKKey("sdkkey", projectTwo, orgTwo, envIdentifier)

		var (
			token *client.AuthenticateResponse
			err   error
		)

		err = retry.Do(
			func() error {
				token, err = testhelpers.Authenticate(sdkKey, GetStreamURL(), nil)
				if token.StatusCode() != http.StatusOK {
					return errors.New("non 200")
				}
				return err
			},
			retry.Attempts(5), retry.Delay(2000*time.Millisecond),
		)
		assert.Nil(t, err)
		assert.NotNil(t, token.JSON200)

		validateFeatureConfigs := func(r *http.Response) bool {
			var (
				featureConfigsBody = bytes.NewBuffer([]byte{})
				featureConfigs     []client.FeatureConfig
			)

			_, err = io.Copy(featureConfigsBody, r.Body)
			assert.Nil(t, err)

			assert.Nil(t, jsoniter.Unmarshal(featureConfigsBody.Bytes(), &featureConfigs))

			return len(featureConfigs) == 2
		}

		t.Log("When I make a /feature-configs request to the Proxy")
		resp, err := withRetry(
			validateFeatureConfigs,
			func() (*http.Response, error) {
				return proxyClient.GetFeatureConfig(ctx, envID, &client.GetFeatureConfigParams{}, func(ctx context.Context, req *http.Request) error {
					req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.JSON200.AuthToken))
					return nil
				})
			},
		)
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Delete the environment and we shouldn't be able to get config for it anymore
		deleteEnvironment(envIdentifier, projectTwo, orgTwo, t)

		validateFeatureConfigs2 := func(r *http.Response) bool {
			var (
				featureConfigsBody = bytes.NewBuffer([]byte{})
				featureConfigs     []client.FeatureConfig
			)

			_, err = io.Copy(featureConfigsBody, r.Body)
			assert.Nil(t, err)

			_ = jsoniter.Unmarshal(featureConfigsBody.Bytes(), &featureConfigs)

			return len(featureConfigs) == 0
		}

		t.Log("When I make a /feature-configs request to the Proxy")
		resp1, err := withRetry(
			validateFeatureConfigs2,
			func() (*http.Response, error) {
				return proxyClient.GetFeatureConfig(ctx, envID, &client.GetFeatureConfigParams{}, func(ctx context.Context, req *http.Request) error {
					req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.JSON200))
					return nil
				})
			},
		)
		assert.Nil(t, err)
		defer resp1.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp1.StatusCode)
	})
}
