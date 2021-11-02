package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
	"github.com/harness/ff-proxy/repository"
	"github.com/stretchr/testify/assert"
)

// For now this test just hits each endpoint and checks that it gets the correct
// status code in the response which is http.StatusNotImplemented for now.
// It might make sense when this grows to have a test function for each endpoint
// that contains valid & invalid request scenarios and their expected responses
// e.g. Something like this for each endpoint
// TestAuthEndpoint(t testing.T) {
//    // Test auth request with empty ApiKey in body - expect 401
//    // Test auth request with an empty body - expect 401
//    // Test auth request with a valid ApiKey in body - expect 200 and some token
//}
func TestServerEndpoints(t *testing.T) {

	// Create in mem cache and repos
	cache := cache.NewMemCache()
	featureRepo, err := repository.NewFeatureConfigRepo(cache, nil)
	if err != nil {
		t.Fatal(err)
	}

	targetRepo, err := repository.NewTargetRepo(cache, nil)
	if err != nil {
		t.Fatal(err)
	}

	logger := log.NewNoOpLogger()

	// Setup service and http servers
	proxyService := proxyservice.NewProxyService(featureRepo, targetRepo, logger)
	endpoints := NewEndpoints(proxyService)
	server := NewHTTPServer("localhost", 7000, endpoints, logger)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := map[string]struct {
		method             string
		url                string
		payload            interface{}
		headers            http.Header
		expectedStatusCode int
	}{
		"Given I make a valid request to /auth/client": {
			method: http.MethodPost,
			url:    fmt.Sprintf("%s/client/auth", testServer.URL),
			payload: domain.AuthRequest{
				APIKey: "123",
			},
			expectedStatusCode: http.StatusNotImplemented,
		},
		"Given I make a valid request to /client/env/{environmentUUID}/feature-configs": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/feature-configs", testServer.URL),
			expectedStatusCode: http.StatusNotImplemented,
		},
		"Given I make a valid request to /client/env/{environmentUUID}/feature-configs/{identifier}": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/feature-configs/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode: http.StatusNotImplemented,
		},
		"Given I make a valid request to /client/env/{environmentUUID}/target-segments": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/target-segments", testServer.URL),
			expectedStatusCode: http.StatusNotImplemented,
		},
		"Given I make a valid request to /client/env/{environmentUUID}/target-segments/{identifier}": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/target-segments/james}", testServer.URL),
			expectedStatusCode: http.StatusNotImplemented,
		},
		"Given I make a valid request to /client/env/{environmentUUID}/target/{target}/evaluations": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/target/james/evaluations", testServer.URL),
			expectedStatusCode: http.StatusNotImplemented,
		},
		"Given I make a valid request to /client/env/{environmentUUID}/target/{target}/evaluations/{feature}": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/target/james/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode: http.StatusNotImplemented,
		},
		"Given I make a valid request to /stream": {
			method: http.MethodGet,
			url:    fmt.Sprintf("%s/stream", testServer.URL),
			headers: http.Header{
				"API-Key": []string{"123"},
			},
			expectedStatusCode: http.StatusNotImplemented,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			var req *http.Request

			switch tc.method {
			case http.MethodPost:
				b, err := json.Marshal(tc.payload)
				if err != nil {
					t.Fatal(err)
				}

				req, err = http.NewRequest(http.MethodPost, tc.url, bytes.NewBuffer(b))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, tc.url, nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			if tc.headers != nil {
				req.Header = tc.headers
			}

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			// Validate response body once it's implemented
		})
	}
}
