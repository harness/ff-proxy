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
			sdkKey := createSDKKey(tc.args.orgIdentifier, tc.args.projectIdentifier, tc.args.envIdentifier, "env_creation_sdk_key", t)
			defer deleteSDKKey(tc.args.orgIdentifier, tc.args.projectIdentifier, tc.args.envIdentifier, "env_creation_sdk_key", t)

			proxyClient := testhelpers.DefaultEvaluationClient(GetStreamURL())

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			validateFeatureConfigs := func(r *http.Response) bool {
				var (
					featureConfigsBody = bytes.NewBuffer([]byte{})
					featureConfigs     []client.FeatureConfig
				)

				_, err := io.Copy(featureConfigsBody, r.Body)
				assert.Nil(t, err)

				if r.StatusCode != http.StatusOK {
					t.Errorf("got status %d, want %d", r.StatusCode, http.StatusOK)
					t.Errorf("response body: %s", featureConfigsBody.String())
				}

				assert.Nil(t, jsoniter.Unmarshal(featureConfigsBody.Bytes(), &featureConfigs))

				return len(featureConfigs) == tc.expected.numFeatureConfigs
			}

			var (
				token   string
				retries int
			)

			// Retry a few times because there could be some lag between the sdk
			// key being created in Saas and getting pushed down to the Proxy
			for token == "" && retries <= 5 {
				var err error

				token, _, err = testhelpers.AuthenticateSDKClient(sdkKey, GetStreamURL(), nil)
				if err != nil {
					t.Error(err)
				}

				if token == "" {
					time.Sleep(5 * time.Second)
					retries += 1
				}
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

//func TestEnvironmentDeletion(t *testing.T) {
//	var (
//		orgTwo        = GetSecondaryOrgIdentifier()
//		projectTwo    = GetSecondaryProjectIdentifier() // Scope = all
//		envIdentifier = "TestEnvironmentDeletion"
//	)
//
//	createEnvironment := func(identifier string, project string, org string, t *testing.T) string {
//		resp, envID, err := testhelpers.CreateEnvironment(org, project, identifier, identifier)
//		assert.Nil(t, err)
//		assert.Equal(t, http.StatusOK, resp.StatusCode)
//
//		if resp.StatusCode != http.StatusOK {
//			t.Errorf("got status %d, want %d", resp.StatusCode, http.StatusOK)
//		}
//		return envID
//	}
//
//	deleteEnvironment := func(identifier string, project string, org string, t *testing.T) {
//		testhelpers.DeleteEnvironment(org, project, identifier)
//	}
//
//	deleteSDKKey(orgTwo, projectTwo, envIdentifier, "test-key", t)
//
//	t.Run("When I Create an environment I should be able to fetch its Flag Config from the Proxy", func(t *testing.T) {
//		// Create the environment
//		envID := createEnvironment(envIdentifier, projectTwo, orgTwo, t)
//		sdkKey := createSDKKey(orgTwo, projectTwo, envIdentifier, "env_deletion_sdk_key", t)
//		defer deleteSDKKey(orgTwo, projectTwo, envIdentifier, "env_deletion_sdk_key", t)
//
//		proxyClient := testhelpers.DefaultEvaluationClient(GetStreamURL())
//
//		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//		defer cancel()
//
//		validateFeatureConfigs := func(r *http.Response) bool {
//			var (
//				featureConfigsBody = bytes.NewBuffer([]byte{})
//				featureConfigs     []client.FeatureConfig
//			)
//
//			if r.StatusCode != http.StatusOK {
//				t.Errorf("got status %d, want %d", r.StatusCode, http.StatusOK)
//				t.Errorf("response body: %s", featureConfigsBody.String())
//			}
//
//			_, err := io.Copy(featureConfigsBody, r.Body)
//			assert.Nil(t, err)
//
//			assert.Nil(t, jsoniter.Unmarshal(featureConfigsBody.Bytes(), &featureConfigs))
//
//			return len(featureConfigs) == 2
//		}
//
//		var (
//			token   string
//			retries int
//		)
//
//		// Retry a few times
//		for token == "" && retries <= 5 {
//			var err error
//
//			token, _, err = testhelpers.AuthenticateSDKClient(sdkKey, GetStreamURL(), nil)
//			if err != nil {
//				t.Error(err)
//			}
//
//			if token != "" {
//				time.Sleep(5 * time.Second)
//				retries += 1
//			}
//		}
//
//		t.Log("When I make a /feature-configs request to the Proxy")
//		resp, err := withRetry(
//			validateFeatureConfigs,
//			func() (*http.Response, error) {
//				return proxyClient.GetFeatureConfig(ctx, envID, &client.GetFeatureConfigParams{}, func(ctx context.Context, req *http.Request) error {
//					req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
//					return nil
//				})
//			},
//		)
//		assert.Nil(t, err)
//		defer resp.Body.Close()
//
//		assert.Equal(t, http.StatusOK, resp.StatusCode)
//
//		// Delete the environment and we shouldn't be able to get config for it anymore
//		deleteEnvironment(envIdentifier, projectTwo, orgTwo, t)
//
//		//validateFeatureConfigs2 := func(r *http.Response) bool {
//		//	// Expect a 401 because the environment that we're trying
//		//	// auth with doesn't exist anymore so our token shouldn't
//		//	// work
//		//	return r.StatusCode == http.StatusUnauthorized
//		//}
//
//		//t.Log("When I make a /feature-configs request to the Proxy")
//		//resp1, err := withRetry(
//		//	validateFeatureConfigs2,
//		//	func() (*http.Response, error) {
//		//		return proxyClient.GetFeatureConfig(ctx, envID, &client.GetFeatureConfigParams{}, func(ctx context.Context, req *http.Request) error {
//		//			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
//		//			return nil
//		//		})
//		//	},
//		//)
//		//assert.Nil(t, err)
//		//defer resp1.Body.Close()
//		//
//		//assert.Equal(t, http.StatusOK, resp1.StatusCode)
//	})
//}

func createSDKKey(org string, project string, env string, identifier string, t *testing.T) string {
	body := testhelpers.GetAddAPIKeyBody(identifier, "Server", identifier, "", "")

	var sdkKey string

	err := retry.Do(func() error {
		resp, err := testhelpers.AddAPIKey(org, body, project, env)
		if err != nil {
			t.Errorf("failed to create SDK Key: %s", err)
			return err
		}

		if resp.JSON201 == nil {
			return fmt.Errorf("SDK Key was not created: status_code=%d", resp.StatusCode())
		}

		sdkKey = resp.JSON201.ApiKey
		return nil
	})
	if err != nil {
		t.Error(err)
	}

	//resp, err := testhelpers.AddAPIKey(org, body, project, env)
	//if err != nil {
	//	t.Errorf("failed to create SDK Key: %s", err)
	//	return ""
	//}
	//
	//if resp.StatusCode() != http.StatusCreated {
	//	t.Errorf("got status %d, want %d", resp.StatusCode(), http.StatusCreated)
	//	t.Errorf("body: %s", resp.Body)
	//	return ""
	//}

	return sdkKey

}

func deleteSDKKey(org, project, env, identifier string, t *testing.T) {
	_, err := testhelpers.DeleteSDKKey(org, project, env, identifier)
	if err != nil {
		t.Logf("failed to cleanup sdk key at the end of test: %s", err)
	}
}
