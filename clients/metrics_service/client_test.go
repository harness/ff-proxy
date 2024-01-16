package metricsservice

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/stretchr/testify/assert"
)

const (
	defaultAccount    = "account"
	defaultMetricsURL = "https://events.ff.harness.io/api/1.0"
	defaultToken      = "token"
)

type mockService struct {
	*sync.Mutex
	clientgen.ClientWithResponsesInterface

	postMetricsWithResp func(environment string) (*clientgen.PostMetricsResponse, error)
	getProxyConfigCalls int
}

func (m mockService) PostMetricsWithResponse(ctx context.Context, environment clientgen.EnvironmentPathParam, params *clientgen.PostMetricsParams, body clientgen.PostMetricsJSONRequestBody, reqEditors ...clientgen.RequestEditorFn) (*clientgen.PostMetricsResponse, error) {
	var err error
	var resp *clientgen.PostMetricsResponse
	if m.postMetricsWithResp != nil {
		resp, err = m.postMetricsWithResp(environment)
	}

	return resp, err
}

func TestMetricService_addAuthToken(t *testing.T) {
	testCases := map[string]struct {
		token              string
		expectedAuthHeader string
		expectedErr        error
	}{
		"Given valid token exists in context then Authorization header is added to request": {
			token:              defaultToken,
			expectedAuthHeader: fmt.Sprintf("Bearer %s", defaultToken),
			expectedErr:        nil,
		},
		"Given no token in context error is returned": {
			token:              "",
			expectedAuthHeader: "",
			expectedErr:        fmt.Errorf("no auth token exists in context"),
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			// create empty request
			req, _ := http.NewRequest("GET", "url", nil)

			// create context and add token to it
			ctx := context.Background()
			ctx = context.WithValue(ctx, tokenKey, tc.token)

			// check metrics are cleared after sending
			err := addAuthToken(ctx, req)

			// get auth header from updated request
			assert.Equal(t, tc.expectedAuthHeader, req.Header.Get("Authorization"))
			// check how many times post metrics were called
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}
