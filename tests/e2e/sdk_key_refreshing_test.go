package e2e

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/avast/retry-go"
	"github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/tests/e2e/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestSDKKeyCreated(t *testing.T) {
	var (
		org          = GetOrgIdentifier()
		project      = GetProjectIdentifier()
		defaultEnv   = testhelpers.GetDefaultEnvironment()
		secondaryEnv = testhelpers.GetSecondaryEnvironment()
	)

	type args struct {
		sdkKeyIdentifier string
		environment      string
	}

	type mocks struct {
	}

	type expected struct {
		statusCode int
	}

	createSDKKey := func(identifier string, env string, t *testing.T) string {
		body := testhelpers.GetAddAPIKeyBody(identifier, "Server", identifier, "", "")
		resp, err := testhelpers.AddAPIKey(org, body, project, env)
		assert.Nil(t, err)
		assert.NotNil(t, resp.JSON201)

		return resp.JSON201.ApiKey
	}

	deleteSDKKey := func(identifier string, env string, t *testing.T) {
		_, err := testhelpers.DeleteSDKKey(org, project, env, identifier)
		if err != nil {
			t.Logf("failed to cleanup sdk key at the end of test: %s", err)
		}
	}

	testCases := map[string]struct {
		args          args
		mocks         mocks
		createSDKKey  func(identifier string, env string, t *testing.T) string
		cleanupSDKKey func(identifier string, env string, t *testing.T)
		expected      expected
	}{
		"Given I create a new SDK key in an environment that is associated with my ProxyKey": {
			args: args{
				sdkKeyIdentifier: "CreateSDKKeyTest_HappyPath",
				environment:      defaultEnv,
			},
			createSDKKey:  createSDKKey,
			cleanupSDKKey: deleteSDKKey,
			expected:      expected{statusCode: http.StatusOK},
		},
		"Given I create a new SDK key in an environment that is NOT associated with my ProxyKey": {
			args: args{
				sdkKeyIdentifier: "CreateSDKKeyTest_SadPath",
				environment:      secondaryEnv,
			},
			createSDKKey:  createSDKKey,
			cleanupSDKKey: deleteSDKKey,
			expected:      expected{statusCode: http.StatusUnauthorized},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			defer func() {
				tc.cleanupSDKKey(tc.args.sdkKeyIdentifier, tc.args.environment, t)
			}()
			sdkKey := tc.createSDKKey(tc.args.sdkKeyIdentifier, tc.args.environment, t)

			proxyClient := testhelpers.DefaultEvaluationClient(GetStreamURL())

			var resp *http.Response
			err := retry.Do(func() error {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				r, err := proxyClient.Authenticate(ctx, client.AuthenticateJSONRequestBody{
					ApiKey: sdkKey,
				})
				if err != nil {
					return err
				}
				resp = r

				if resp.StatusCode != tc.expected.statusCode {
					return errors.New("blah")
				}

				return nil
			},
				retry.Attempts(5), retry.Delay(1000*time.Millisecond),
			)

			assert.Nil(t, err)
			assert.Equal(t, tc.expected.statusCode, resp.StatusCode)
		})
	}
}
