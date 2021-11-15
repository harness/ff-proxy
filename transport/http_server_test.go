package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	ffproxy "github.com/harness/ff-proxy"
	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/config"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/middleware"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
	"github.com/harness/ff-proxy/repository"
	"github.com/stretchr/testify/assert"
	"github.com/wings-software/ff-server/pkg/hash"
)

type fileSystem struct {
	path string
}

func (f fileSystem) Open(name string) (fs.File, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return file, nil
}

const (
	apiKey123       = "apikey-123"
	envID123        = "env-123"
	hashedAPIKey123 = "486089aa445aa0d9ee898f4f38dec4b0d1ee69da3ed7697afb1bdcd46f3fc5ec"
	apiKey123Token  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbnZpcm9ubWVudCI6ImVudi0xMjMiLCJpYXQiOjE2MzcwNTUyMjUsIm5iZiI6MTYzNzA1NTIyNX0.scUWHotiphYV_xYr3UvEkaUw9CuHQnFThcq3CpPkmu8"
)

// setupHTTPServer is a helper that loads test config for populating the repos
// and injects all the required dependencies into the proxy service and http server
func setupHTTPServer(t *testing.T, bypassAuth bool) *HTTPServer {
	fileSystem := fileSystem{path: "../config/test"}
	config, err := config.NewLocalConfig(fileSystem, "../config/test")
	if err != nil {
		t.Fatal(err)
	}

	cache := cache.NewMemCache()
	featureRepo, err := repository.NewFeatureConfigRepo(cache, config.FeatureConfig())
	if err != nil {
		t.Fatal(err)
	}

	targetRepo, err := repository.NewTargetRepo(cache, config.Targets())
	if err != nil {
		t.Fatal(err)
	}

	segmentRepo, err := repository.NewSegmentRepo(cache, config.Segments())
	if err != nil {
		t.Fatal(err)
	}

	authRepo := repository.NewAuthRepo(map[domain.AuthAPIKey]string{
		domain.AuthAPIKey(hashedAPIKey123): envID123,
	})
	tokenSource := ffproxy.NewTokenSource(log.NoOpLogger{}, authRepo, hash.NewSha256(), []byte(`secret`))

	logger := log.NewNoOpLogger()

	var service proxyservice.ProxyService
	service = proxyservice.NewService(featureRepo, targetRepo, segmentRepo, tokenSource.GenerateToken, proxyservice.NewFeatureEvaluator(), logger)
	service = middleware.NewAuthMiddleware(tokenSource.ValidateToken, bypassAuth, service)
	endpoints := NewEndpoints(service)

	return NewHTTPServer("localhost", 7000, endpoints, logger)
}

// featureConfigWithSegments is the expected response body for a FeatureConfigs request - the newline at the end is intentional
var featureConfigWithSegments = []byte(`[{"defaultServe":{"variation":"true"},"environment":"featureflagsqa","feature":"harnessappdemodarkmode","kind":"boolean","offVariation":"false","prerequisites":[],"project":"FeatureFlagsQADemo","rules":[{"clauses":[{"attribute":"age","id":"79f5bca0-17ca-42c2-8934-5cee840fe2e0","negate":false,"op":"equal","values":["55"]}],"priority":1,"ruleId":"8756c207-abf8-4202-83fd-dedf5d27e2c2","serve":{"variation":"false"}}],"state":"on","variationToTargetMap":[{"targetSegments":["flagsTeam"],"targets":[{"identifier":"davej","name":"Dave Johnston"}],"variation":"false"}],"variations":[{"identifier":"true","name":"True","value":"true"},{"identifier":"false","name":"False","value":"false"}],"version":568,"segments":{"flagsTeam":{"createdAt":123,"environment":"featureflagsqa","excluded":[],"identifier":"flagsTeam","included":[],"modifiedAt":456,"name":"flagsTeam","rules":[{"attribute":"ip","id":"31c18ee7-8051-44cc-8507-b44580467ee5","negate":false,"op":"equal","values":["2a00:23c5:b672:2401:158:f2a6:67a0:6a79"]}],"version":1}}},{"defaultServe":{"variation":"1"},"environment":"featureflagsqa","feature":"yet_another_flag","kind":"string","offVariation":"2","prerequisites":[],"project":"FeatureFlagsQADemo","rules":[],"state":"on","variations":[{"identifier":"1","name":"1","value":"1"},{"identifier":"2","name":"2","value":"2"}],"version":6,"segments":{"flagsTeam":{"createdAt":123,"environment":"featureflagsqa","excluded":[],"identifier":"flagsTeam","included":[],"modifiedAt":456,"name":"flagsTeam","rules":[{"attribute":"ip","id":"31c18ee7-8051-44cc-8507-b44580467ee5","negate":false,"op":"equal","values":["2a00:23c5:b672:2401:158:f2a6:67a0:6a79"]}],"version":1}}}]
`)

// TestHTTPServer_GetFeatureConfig sets up an service with repositories populated
// from config/test, injects it into the HTTPServer and makes HTTP requests
// to the /client/env/{environmentUUID}/feature-configs endpoint
func TestHTTPServer_GetFeatureConfig(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := map[string]struct {
		method               string
		url                  string
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/env/1234/feature-configs", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make GET request for an environment that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/abcd/feature-configs", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for an environment that does exist": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/feature-configs", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: featureConfigWithSegments,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			var req *http.Request
			var err error

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, tc.url, bytes.NewBuffer([]byte{}))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, tc.url, nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			if tc.expectedResponseBody != nil {
				actual, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("(%s): failed to read response body: %s", desc, err)
				}

				if !assert.ElementsMatch(t, tc.expectedResponseBody, actual) {
					t.Errorf("(%s) expected: %s \n got: %s ", desc, tc.expectedResponseBody, actual)
				}
			}
		})
	}
}

// harnessAppDemoDarkMode is the expected response body for a FeatureConfigsByIdentifier request where identifier='harnessappdemodarkmode' - the newline at the end is intentional
var harnessAppDemoDarkMode = []byte(`{"defaultServe":{"variation":"true"},"environment":"featureflagsqa","feature":"harnessappdemodarkmode","kind":"boolean","offVariation":"false","prerequisites":[],"project":"FeatureFlagsQADemo","rules":[{"clauses":[{"attribute":"age","id":"79f5bca0-17ca-42c2-8934-5cee840fe2e0","negate":false,"op":"equal","values":["55"]}],"priority":1,"ruleId":"8756c207-abf8-4202-83fd-dedf5d27e2c2","serve":{"variation":"false"}}],"state":"on","variationToTargetMap":[{"targetSegments":["flagsTeam"],"targets":[{"identifier":"davej","name":"Dave Johnston"}],"variation":"false"}],"variations":[{"identifier":"true","name":"True","value":"true"},{"identifier":"false","name":"False","value":"false"}],"version":568,"segments":{"flagsTeam":{"createdAt":123,"environment":"featureflagsqa","excluded":[],"identifier":"flagsTeam","included":[],"modifiedAt":456,"name":"flagsTeam","rules":[{"attribute":"ip","id":"31c18ee7-8051-44cc-8507-b44580467ee5","negate":false,"op":"equal","values":["2a00:23c5:b672:2401:158:f2a6:67a0:6a79"]}],"version":1}}}
`)

// TestHTTPServer_GetFeatureConfigByIdentifier sets up a service with repositories
// populated from config/test, injects it into the HTTPServer and makes HTTP
// requests to the /client/env/{environmentUUID}/feature-configs/{identifier} endpoint
func TestHTTPServer_GetFeatureConfigByIdentifier(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := map[string]struct {
		method               string
		url                  string
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/env/1234/feature-configs/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make GET request for an environment that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/abcd/feature-configs/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for an identifier that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/feature-configs/foobar", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for an environment and identifier that exist": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/feature-configs/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: harnessAppDemoDarkMode,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			var req *http.Request
			var err error

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, tc.url, bytes.NewBuffer([]byte{}))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, tc.url, nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			if tc.expectedResponseBody != nil {
				actual, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("(%s): failed to read response body: %s", desc, err)
				}

				if !assert.ElementsMatch(t, tc.expectedResponseBody, actual) {
					t.Errorf("(%s) expected: %s \n got: %s ", desc, tc.expectedResponseBody, actual)
				}
			}
		})
	}
}

// targetSegments is the expected response body for a TargetSegments request - the newline at the end is intentional
var targetSegments = []byte(`[{"createdAt":123,"environment":"featureflagsqa","excluded":[],"identifier":"flagsTeam","included":[],"modifiedAt":456,"name":"flagsTeam","rules":[{"attribute":"ip","id":"31c18ee7-8051-44cc-8507-b44580467ee5","negate":false,"op":"equal","values":["2a00:23c5:b672:2401:158:f2a6:67a0:6a79"]}],"version":1}]
`)

// TestHTTPServer_GetTargetSegments sets up a service with repositories
// populated from config/test, injects it into the HTTPServer and makes HTTP
// requests to the /client/env/{environmentUUID}/target-segments endpoint
func TestHTTPServer_GetTargetSegments(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := map[string]struct {
		method               string
		url                  string
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/env/1234/target-segments", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make GET request for an environment that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/abcd/target-segments", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for an environment and identifier that exist": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetSegments,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			var req *http.Request
			var err error

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, tc.url, bytes.NewBuffer([]byte{}))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, tc.url, nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			if tc.expectedResponseBody != nil {
				actual, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("(%s): failed to read response body: %s", desc, err)
				}

				if !assert.ElementsMatch(t, tc.expectedResponseBody, actual) {
					t.Errorf("(%s) expected: %s \n got: %s ", desc, tc.expectedResponseBody, actual)
				}
			}
		})
	}
}

// segmentFlagsTeam is the expected response body for a TargetSegmentsByIdentfier request where identifer='flagsTeam' - the newline at the end is intentional
var segmentFlagsTeam = []byte(`{"createdAt":123,"environment":"featureflagsqa","excluded":[],"identifier":"flagsTeam","included":[],"modifiedAt":456,"name":"flagsTeam","rules":[{"attribute":"ip","id":"31c18ee7-8051-44cc-8507-b44580467ee5","negate":false,"op":"equal","values":["2a00:23c5:b672:2401:158:f2a6:67a0:6a79"]}],"version":1}
`)

// TestHTTPServer_GetTargetSegmentsByIdentifier sets up a service with repositories
// populated from config/test, injects it into the HTTPServer and makes HTTP
// requests to the /client/env/{environmentUUID}/target-segments/{identifier} endpoint
func TestHTTPServer_GetTargetSegmentsByIdentifier(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := map[string]struct {
		method               string
		url                  string
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/env/1234/target-segments/james", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make GET request for an environment that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/abcd/target-segments", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for an identifier that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/target-segments/bar", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for an environment and identifier that exist": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments/flagsTeam", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: segmentFlagsTeam,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			var req *http.Request
			var err error

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, tc.url, bytes.NewBuffer([]byte{}))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, tc.url, nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			if tc.expectedResponseBody != nil {
				actual, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("(%s): failed to read response body: %s", desc, err)
				}

				if !assert.ElementsMatch(t, tc.expectedResponseBody, actual) {
					t.Errorf("(%s) expected: %s \n got: %s ", desc, tc.expectedResponseBody, actual)
				}
			}
		})
	}
}

var (
	// targetJamesEvaluations is the expected response body for a Evaluations request - the newline at the end is intentional
	targetJamesEvaluations = []byte(`[{"flag":"harnessappdemodarkmode","identifier":"false","kind":"boolean","value":"false"},{"flag":"yet_another_flag","identifier":"1","kind":"string","value":"1"}]
`)

	// targetFooEvaluations is the expected response body for a Evaluations request - the newline at the end is intentional
	targetFooEvaluations = []byte(`[{"flag":"harnessappdemodarkmode","identifier":"true","kind":"boolean","value":"true"},{"flag":"yet_another_flag","identifier":"1","kind":"string","value":"1"}]
`)
)

// TestHTTPServer_GetEvaluations sets up a service with repositories populated
// from config/test, injects it into the HTTPServer and makes HTTP
// requests to the /client/env/{environmentUUID}/evaluations endpoint
func TestHTTPServer_GetEvaluations(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := map[string]struct {
		method               string
		url                  string
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/env/1234/target/james/evaluations", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make GET request for an environment that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/abcd/target/james/evaluations", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for target that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/target/bar/evaluations", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for an environment and the target 'james'": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/james/evaluations", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetJamesEvaluations,
		},
		"Given I make GET request for an environment and the target 'foo'": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/foo/evaluations", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetFooEvaluations,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			var req *http.Request
			var err error

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, tc.url, bytes.NewBuffer([]byte{}))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, tc.url, nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			if tc.expectedResponseBody != nil {
				actual, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("(%s): failed to read response body: %s", desc, err)
				}

				if !assert.ElementsMatch(t, tc.expectedResponseBody, actual) {
					t.Errorf("(%s) expected: %s \n got: %s ", desc, tc.expectedResponseBody, actual)
				}
			}
		})
	}
}

var (
	// darkModeEvaluationFalse is the expected response body for a EvaluationsByFeature request when identifer='james' and feature='harnessappdemodarkmode '- the newline at the end is intentional
	darkModeEvaluationFalse = []byte(`{"flag":"harnessappdemodarkmode","identifier":"false","kind":"boolean","value":"false"}
`)

	// darkModeEvaluationTrue is the expected response body for a EvaluationsByFeature request when identifer='james' and feature='harnessappdemodarkmode '- the newline at the end is intentional
	darkModeEvaluationTrue = []byte(`{"flag":"harnessappdemodarkmode","identifier":"true","kind":"boolean","value":"true"}
`)
)

// TestHTTPServer_GetEvaluationsByFeature sets up an service with repositories
// populated from config/test, injects it into the HTTPServer and makes HTTP
// requests to the /client/env/{environmentUUID}/evaluations/{feature} endpoint
func TestHTTPServer_GetEvaluationsByFeature(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := map[string]struct {
		method               string
		url                  string
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/env/1234/target/james/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make GET request for an environment that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/abcd/target/james/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for target that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/1234/target/bar/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for an environment and the target 'james'": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/james/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: darkModeEvaluationFalse,
		},
		"Given I make GET request for an environment and the target 'foo'": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/foo/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: darkModeEvaluationTrue,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			var req *http.Request
			var err error

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, tc.url, bytes.NewBuffer([]byte{}))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, tc.url, nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			if tc.expectedResponseBody != nil {
				actual, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("(%s): failed to read response body: %s", desc, err)
				}

				if !assert.ElementsMatch(t, tc.expectedResponseBody, actual) {
					t.Errorf("(%s) expected: %s \n got: %s ", desc, tc.expectedResponseBody, actual)
				}
			}
		})
	}
}

func TestHTTPServer_PostMetrics(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := map[string]struct {
		method               string
		url                  string
		body                 []byte
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a POST request": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/metrics/1234", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make a POST request to /metrics/{environmentUUID}": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/metrics/1234", testServer.URL),
			body:               []byte(`{}`),
			expectedStatusCode: http.StatusNotImplemented,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			var req *http.Request
			var err error

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, tc.url, bytes.NewBuffer(tc.body))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, tc.url, nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			if tc.expectedResponseBody != nil {
				actual, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("(%s): failed to read response body: %s", desc, err)
				}

				if !assert.Equal(t, tc.expectedResponseBody, actual) {
					t.Errorf("(%s) expected: %s \n got: %s ", desc, tc.expectedResponseBody, actual)
				}
			}
		})
	}
}

func TestHTTPServer_PostAuthentication(t *testing.T) {
	// setup HTTPServer with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	testCases := map[string]struct {
		method             string
		url                string
		body               []byte
		expectedStatusCode int
	}{
		"Given I make a request that isn't a POST request": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/auth", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make an auth request with an APIKey that doesn't exist": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/auth", testServer.URL),
			body:               []byte(`{"apiKey": "hello"}`),
			expectedStatusCode: http.StatusUnauthorized,
		},
		"Given I make an auth request with an APIKey that does exist": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/auth", testServer.URL),
			body:               []byte(fmt.Sprintf(`{"apiKey": "%s"}`, apiKey123)),
			expectedStatusCode: http.StatusOK,
		},
	}

	for desc, tc := range testCases {
		t.Run(desc, func(t *testing.T) {
			var req *http.Request
			var err error

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, tc.url, bytes.NewBuffer(tc.body))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, tc.url, nil)
				if err != nil {
					t.Fatal(err)
				}
			}

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			b, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("(%s): failed to read response body: %s", desc, err)
			}

			actualAuthResponse := domain.AuthResponse{}
			if tc.expectedStatusCode == http.StatusOK {
				if err := json.Unmarshal(b, &actualAuthResponse); err != nil {
					t.Fatalf("(%s): failed to unmarshal response body: %s", desc, err)
				}
				assert.NotEmpty(t, actualAuthResponse.AuthToken)
			}

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)
		})
	}
}

// TestAuthentication tests the endpoints when the auth middleware is enabled
func TestAuthentication(t *testing.T) {
	// setup HTTPServer & service with auth enabled
	server := setupHTTPServer(t, false)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	endpoints := map[string]string{
		"FeatureConfigs":             "/client/env/1234/feature-configs",
		"FeatureConfigsByIdentifier": "/client/env/1234/feature-configs/harnessappdemodarkmode",
		"TargetSegments":             "/client/env/1234/target-segments",
		"TargetSegmentsByIdentifier": "/client/env/1234/target-segments/flagsTeam",
		"Evaluations":                "/client/env/1234/target/james/evaluations",
		"EvaluationsByFeature":       "/client/env/1234/target/james/evaluations/harnessappdemodarkmode",
	}

	testCases := map[string]struct {
		method             string
		headers            http.Header
		expectedStatusCode int
	}{
		"Given I make requests to the service endpoints without an auth header": {
			method:             http.MethodGet,
			headers:            http.Header{},
			expectedStatusCode: http.StatusUnauthorized,
		},
		"Given I make requests to the service endpoints with an invalid auth header": {
			method: http.MethodGet,
			headers: http.Header{
				"Authorization": []string{"Bearer: foo"},
			},
			expectedStatusCode: http.StatusUnauthorized,
		},
		"Given I make requests to the service endpoints with a valid auth header": {
			method: http.MethodGet,
			headers: http.Header{
				"Authorization": []string{fmt.Sprintf("Bearer %s", apiKey123Token)},
			},
			expectedStatusCode: http.StatusOK,
		},
	}

	for desc, tc := range testCases {
		tc := tc

		for endpoint, path := range endpoints {
			url := fmt.Sprintf("%s%s", testServer.URL, path)

			req, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				t.Fatalf("(%s) - endpoint %s, failed to create request: %s", desc, endpoint, err)
			}
			req.Header = tc.headers

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if ok := assert.Equal(t, tc.expectedStatusCode, resp.StatusCode); !ok {
				t.Errorf("(%s) - endpoint=%q, expected: %d, got %d", desc, endpoint, tc.expectedStatusCode, resp.StatusCode)
			}
		}
	}

}
