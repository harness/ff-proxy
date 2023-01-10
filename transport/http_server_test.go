package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/harness/ff-proxy/token"

	"github.com/harness/ff-proxy/stream"

	"github.com/fanout/go-gripcontrol"
	"github.com/go-redis/redis/v8"
	sdkstream "github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/config"
	"github.com/harness/ff-proxy/domain"
	admingen "github.com/harness/ff-proxy/gen/admin"
	"github.com/harness/ff-proxy/hash"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/middleware"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
	"github.com/harness/ff-proxy/repository"
	"github.com/r3labs/sse"
	"github.com/stretchr/testify/assert"
)

func boolPtr(b bool) *bool {
	return &b
}

type mockClientService struct {
	authenticate func(target domain.Target) (domain.Target, error)
	targetsc     chan domain.Target
}

type mockMetricService struct {
	storeMetrics func(ctx context.Context, metrics domain.MetricsRequest) error
}

func (m *mockClientService) Authenticate(ctx context.Context, apiKey string, target domain.Target) (string, error) {
	defer close(m.targetsc)

	t, err := m.authenticate(target)
	if err != nil {
		return "", err
	}

	m.targetsc <- t

	return "token-we-don't-care-about", nil
}

func (m *mockClientService) Targets() []domain.Target {
	targets := []domain.Target{}
	for t := range m.targetsc {
		targets = append(targets, t)
	}
	return targets
}

func (m *mockMetricService) StoreMetrics(ctx context.Context, req domain.MetricsRequest) error {
	return m.storeMetrics(ctx, req)
}

const (
	apiKey1         = "apikey1"
	envID123        = "1234"
	hashedAPIKey123 = "486089aa445aa0d9ee898f4f38dec4b0d1ee69da3ed7697afb1bdcd46f3fc5ec"
	apiKey123Token  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbnZpcm9ubWVudCI6ImVudi0xMjMiLCJpYXQiOjE2MzcwNTUyMjUsIm5iZiI6MTYzNzA1NTIyNX0.scUWHotiphYV_xYr3UvEkaUw9CuHQnFThcq3CpPkmu8"
)

type setupConfig struct {
	featureRepo   *repository.FeatureFlagRepo
	targetRepo    *repository.TargetRepo
	segmentRepo   *repository.SegmentRepo
	authRepo      *repository.AuthRepo
	cacheHealthFn proxyservice.CacheHealthFn
	envHealthFn   proxyservice.EnvHealthFn
	clientService *mockClientService
	streamWorker  stream.StreamWorker
	eventListener sdkstream.EventStreamListener
	cache         cache.Cache
	metricService *mockMetricService
}

type setupOpts func(s *setupConfig)

func setupWithFeatureRepo(r repository.FeatureFlagRepo) setupOpts {
	return func(s *setupConfig) {
		s.featureRepo = &r
	}
}

func setupWithTargetRepo(r repository.TargetRepo) setupOpts {
	return func(s *setupConfig) {
		s.targetRepo = &r
	}
}

func setupWithSegmentRepo(r repository.SegmentRepo) setupOpts {
	return func(s *setupConfig) {
		s.segmentRepo = &r
	}
}

func setupWithAuthRepo(r repository.AuthRepo) setupOpts {
	return func(s *setupConfig) {
		s.authRepo = &r
	}
}

func setupWithClientService(c *mockClientService) setupOpts {
	return func(s *setupConfig) {
		s.clientService = c
	}
}

func setupWithCacheHealthFn(fn proxyservice.CacheHealthFn) setupOpts {
	return func(s *setupConfig) {
		s.cacheHealthFn = fn
	}
}

func setupWithEnvHealthFn(fn proxyservice.EnvHealthFn) setupOpts {
	return func(s *setupConfig) {
		s.envHealthFn = fn
	}
}

func setupWithCache(c cache.Cache) setupOpts {
	return func(s *setupConfig) {
		s.cache = c
	}
}

// setupHTTPServer is a helper that loads test config for populating the repos
// and injects all the required dependencies into the proxy service and http server
func setupHTTPServer(t *testing.T, bypassAuth bool, opts ...setupOpts) *HTTPServer {
	fileSystem := os.DirFS("../config/test")
	config, err := config.NewLocalConfig(fileSystem)
	if err != nil {
		t.Fatal(err)
	}

	setupConfig := &setupConfig{}
	for _, opt := range opts {
		opt(setupConfig)
	}

	if setupConfig.cache == nil {
		setupConfig.cache = cache.NewMemCache()
	}

	if setupConfig.featureRepo == nil {
		fr, err := repository.NewFeatureFlagRepo(setupConfig.cache, config.FeatureFlag())
		if err != nil {
			t.Fatal(err)
		}

		setupConfig.featureRepo = &fr
	}

	if setupConfig.targetRepo == nil {
		tr, err := repository.NewTargetRepo(setupConfig.cache, config.Targets())
		if err != nil {
			t.Fatal(err)
		}

		setupConfig.targetRepo = &tr
	}

	if setupConfig.segmentRepo == nil {
		sr, err := repository.NewSegmentRepo(setupConfig.cache, config.Segments())
		if err != nil {
			t.Fatal(err)
		}

		setupConfig.segmentRepo = &sr
	}

	if setupConfig.authRepo == nil {
		ar, err := repository.NewAuthRepo(setupConfig.cache, config.AuthConfig())
		if err != nil {
			t.Fatal(err)
		}

		setupConfig.authRepo = &ar
	}

	if setupConfig.clientService == nil {
		setupConfig.clientService = &mockClientService{authenticate: func(t domain.Target) (domain.Target, error) {
			return t, nil
		}}
	}

	if setupConfig.metricService == nil {
		setupConfig.metricService = &mockMetricService{storeMetrics: func(ctx context.Context, metrics domain.MetricsRequest) error {
			return nil
		}}
	}

	if setupConfig.cacheHealthFn == nil {
		setupConfig.cacheHealthFn = func(ctx context.Context) error {
			return nil
		}
	}

	if setupConfig.envHealthFn == nil {
		setupConfig.envHealthFn = func(ctx context.Context) map[string]error {
			return map[string]error{"123": nil, "456": nil}
		}
	}

	logger := log.NoOpLogger{}

	tokenSource := token.NewTokenSource(logger, setupConfig.authRepo, hash.NewSha256(), []byte(`secret`))

	var service proxyservice.ProxyService
	service = proxyservice.NewService(proxyservice.Config{
		Logger:           log.NewNoOpContextualLogger(),
		FeatureRepo:      *setupConfig.featureRepo,
		TargetRepo:       *setupConfig.targetRepo,
		SegmentRepo:      *setupConfig.segmentRepo,
		AuthRepo:         *setupConfig.authRepo,
		CacheHealthFn:    setupConfig.cacheHealthFn,
		EnvHealthFn:      setupConfig.envHealthFn,
		AuthFn:           tokenSource.GenerateToken,
		ClientService:    setupConfig.clientService,
		MetricService:    setupConfig.metricService,
		Offline:          false,
		Hasher:           hash.NewSha256(),
		StreamingEnabled: true,
	})
	endpoints := NewEndpoints(service)

	server := NewHTTPServer(8000, endpoints, logger, false, "", "")
	server.Use(middleware.NewEchoAuthMiddleware([]byte(`secret`), bypassAuth))
	return server
}

// variationToTargetMap:null is intentional here - refer to FFM-3246 before removing
// featureConfigWithSegments is the expected response body for a FeatureConfigs request - the newline at the end is intentional
var featureConfigWithSegments = []byte(`[{"defaultServe":{"variation":"true"},"environment":"featureflagsqa","feature":"harnessappdemodarkmode","kind":"boolean","offVariation":"false","prerequisites":[],"project":"FeatureFlagsQADemo","rules":[{"clauses":[{"attribute":"age","id":"79f5bca0-17ca-42c2-8934-5cee840fe2e0","negate":false,"op":"equal","values":["55"]}],"priority":1,"ruleId":"8756c207-abf8-4202-83fd-dedf5d27e2c2","serve":{"variation":"false"}}],"state":"on","variationToTargetMap":[{"targetSegments":["flagsTeam"],"targets":[{"identifier":"davej","name":"Dave Johnston"}],"variation":"false"}],"variations":[{"identifier":"true","name":"True","value":"true"},{"identifier":"false","name":"False","value":"false"}],"version":568,"segments":{"flagsTeam":{"createdAt":123,"environment":"featureflagsqa","excluded":[],"identifier":"flagsTeam","included":[],"modifiedAt":456,"name":"flagsTeam","rules":[{"attribute":"ip","id":"31c18ee7-8051-44cc-8507-b44580467ee5","negate":false,"op":"equal","values":["2a00:23c5:b672:2401:158:f2a6:67a0:6a79"]}],"version":1}}},{"defaultServe":{"variation":"1"},"environment":"featureflagsqa","feature":"yet_another_flag","kind":"string","offVariation":"2","prerequisites":[],"project":"FeatureFlagsQADemo","rules":[],"state":"on","variations":[{"identifier":"1","name":"1","value":"1"},{"identifier":"2","name":"2","value":"2"}],"variationToTargetMap":[],"version":6,"segments":{"flagsTeam":{"createdAt":123,"environment":"featureflagsqa","excluded":[],"identifier":"flagsTeam","included":[],"modifiedAt":456,"name":"flagsTeam","rules":[{"attribute":"ip","id":"31c18ee7-8051-44cc-8507-b44580467ee5","negate":false,"op":"equal","values":["2a00:23c5:b672:2401:158:f2a6:67a0:6a79"]}],"version":1}}}]
`)

var emptyFeatureConfig = []byte(`[]
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
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/abcd/feature-configs", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: emptyFeatureConfig,
			// we return an empty feature array for this right now because we can't tell the difference between
			// an environment not existing at all and there just being no features in it
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
			method: http.MethodGet,
			url:    fmt.Sprintf("%s/client/env/abcd/target-segments", testServer.URL),
			// we return an empty segment array for this right now because we can't tell the difference between
			// an environment not existing at all and there just being no segments in it
			// if we return not found server SDK's won't be able to initialise unless they have at least one target group for each environment
			expectedStatusCode: http.StatusOK,
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
			expectedStatusCode: http.StatusOK,
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
	authenticateErr := func(t domain.Target) (domain.Target, error) {
		return domain.Target{}, errors.New("an error")
	}

	authenticateSuccess := func(t domain.Target) (domain.Target, error) {
		return t, nil
	}

	bodyWithTarget := []byte(fmt.Sprintf(`
	{
	"apiKey": "%s",
	"target": {
	"identifier": "foo",
	"name": "bar",
	"anonymous": true,
	"attributes": {
	"hello": "world",
	"age": 27
	}
	}
	}
	`, apiKey1))
	targetKey := domain.NewTargetKey(envID123)

	targets := []domain.Target{
		{Target: admingen.Target{
			Identifier: "foo",
			Name:       "bar",
			Anonymous:  boolPtr(true),
			Attributes: &map[string]interface{}{
				"hello": "world",
				"age":   float64(27),
			},
		}},
	}

	testCases := map[string]struct {
		method                       string
		url                          string
		body                         []byte
		expectedStatusCode           int
		clientService                *mockClientService
		expectedCacheTargets         []domain.Target
		expectedClientServiceTargets []domain.Target
	}{
		"Given I make a request that isn't a POST request": {
			method:             http.MethodGet,
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make an auth request with an APIKey that doesn't exist": {
			method:             http.MethodPost,
			body:               []byte(`{"apiKey": "hello"}`),
			expectedStatusCode: http.StatusUnauthorized,
		},
		"Given I make an auth request with an APIKey that does exist": {
			method:             http.MethodPost,
			body:               []byte(fmt.Sprintf(`{"apiKey": "%s"}`, apiKey1)),
			expectedStatusCode: http.StatusOK,
		},
		"Given I include a Target in my Auth request and have a working connection to FeatureFlags": {
			method: http.MethodPost,
			body:   bodyWithTarget,
			clientService: &mockClientService{
				authenticate: authenticateSuccess,
				targetsc:     make(chan domain.Target, 1),
			},
			expectedStatusCode:           http.StatusOK,
			expectedCacheTargets:         targets,
			expectedClientServiceTargets: targets,
		},
		"Given I include a Target in my Auth request and have no connection to FeatureFlags": {
			method: http.MethodPost,
			body:   bodyWithTarget,
			clientService: &mockClientService{
				authenticate: authenticateErr,
				targetsc:     make(chan domain.Target, 1),
			},
			expectedStatusCode:           http.StatusOK,
			expectedCacheTargets:         targets,
			expectedClientServiceTargets: []domain.Target{},
		},
	}

	for desc, tc := range testCases {
		tc := tc

		targetRepo, err := repository.NewTargetRepo(cache.NewMemCache(), nil)
		if err != nil {
			t.Fatalf("failed to setup targete repo: %s", err)
		}

		// setup HTTPServer with auth bypassed
		server := setupHTTPServer(
			t,
			true,
			setupWithClientService(tc.clientService),
			setupWithTargetRepo(targetRepo),
		)
		testServer := httptest.NewServer(server)

		t.Run(desc, func(t *testing.T) {
			defer testServer.Close()

			var (
				req *http.Request
				err error
				url string = fmt.Sprintf("%s/client/auth", testServer.URL)
			)

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(tc.body))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, url, nil)
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

			if tc.clientService != nil {
				actualClientTargets := []domain.Target{}
				for _, t := range tc.clientService.Targets() {
					actualClientTargets = append(actualClientTargets, t)
				}

				cacheTargets, err := targetRepo.Get(context.Background(), targetKey)
				if err != nil {
					t.Errorf("failed to get targets from cache: %s", err)
				}

				t.Log("Then the Targets in the ClientService should match the expected Targets")
				assert.ElementsMatch(t, tc.expectedClientServiceTargets, actualClientTargets)

				t.Log("And the Targets in the TargetRepo should match the expected Targets ")
				assert.ElementsMatch(t, tc.expectedCacheTargets, cacheTargets)
			}
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

// TestHTTPServer_Health sets up a service with health check functions
// injects it into the HTTPServer and makes HTTP requests to the /health endpoint
func TestHTTPServer_Health(t *testing.T) {

	healthyResponse := []byte(`{"cache":"healthy","env-123":"healthy","env-456":"healthy"}
`)
	cacheUnhealthyResponse := []byte(`{"cache":"unhealthy","env-123":"healthy","env-456":"healthy"}
`)
	env456UnhealthyResponse := []byte(`{"cache":"healthy","env-123":"healthy","env-456":"unhealthy"}
`)
	cacheHealthErr := func(ctx context.Context) error {
		return fmt.Errorf("cache is borked")
	}

	envHealthErr := func(ctx context.Context) map[string]error {
		return map[string]error{"123": nil, "456": fmt.Errorf("stream disconnected")}
	}

	testCases := map[string]struct {
		method               string
		url                  string
		cacheHealthFn        proxyservice.CacheHealthFn
		envHealthFn          proxyservice.EnvHealthFn
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make a GET request and everything is healthy": {
			method:               http.MethodGet,
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: healthyResponse,
		},
		"Given I make a GET request and cache is unhealthy": {
			method:               http.MethodGet,
			expectedStatusCode:   http.StatusServiceUnavailable,
			cacheHealthFn:        cacheHealthErr,
			expectedResponseBody: cacheUnhealthyResponse,
		},
		"Given I make a GET request and env 456 is unhealthy": {
			method:               http.MethodGet,
			expectedStatusCode:   http.StatusServiceUnavailable,
			envHealthFn:          envHealthErr,
			expectedResponseBody: env456UnhealthyResponse,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		// setup HTTPServer & service with auth bypassed
		server := setupHTTPServer(t, true,
			setupWithCacheHealthFn(tc.cacheHealthFn),
			setupWithEnvHealthFn(tc.envHealthFn),
		)
		testServer := httptest.NewServer(server)
		t.Run(desc, func(t *testing.T) {
			defer testServer.Close()
			var req *http.Request
			var err error
			url := fmt.Sprintf("%s/health", testServer.URL)

			switch tc.method {
			case http.MethodPost:
				req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer([]byte{}))
				if err != nil {
					t.Fatal(err)
				}
			case http.MethodGet:
				req, err = http.NewRequest(http.MethodGet, url, nil)
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

func TestHTTPServer_Stream(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	const (
		apiKey = "apikey1"
		envID  = "1234"
	)

	testCases := map[string]struct {
		method                  string
		headers                 http.Header
		url                     string
		expectedStatusCode      int
		expectedResponseHeaders http.Header
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			headers:            http.Header{},
			url:                fmt.Sprintf("%s/stream", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make a GET request but don't have an API-Key header": {
			method:             http.MethodGet,
			headers:            http.Header{},
			url:                fmt.Sprintf("%s/stream", testServer.URL),
			expectedStatusCode: http.StatusBadRequest,
		},
		"Given I make a GET request and have an empty API Key header": {
			method: http.MethodGet,
			headers: http.Header{
				"API-Key": []string{},
			},
			url:                fmt.Sprintf("%s/stream", testServer.URL),
			expectedStatusCode: http.StatusBadRequest,
		},
		"Given I make a GET request and with an API Key that isn't in the AuthConfig": {
			method: http.MethodGet,
			headers: http.Header{
				"API-Key": []string{"foobar"},
			},
			url:                fmt.Sprintf("%s/stream", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make a GET request with a valid API Key Header": {
			method: http.MethodGet,
			headers: http.Header{
				"API-Key": []string{apiKey},
			},
			url:                fmt.Sprintf("%s/stream", testServer.URL),
			expectedStatusCode: http.StatusOK,
			expectedResponseHeaders: http.Header{
				"Content-Type":    []string{"text/event-stream"},
				"Grip-Hold":       []string{"stream"},
				"Grip-Channel":    []string{envID},
				"Grip-Keep-Alive": []string{"\\n; format=cstring; timeout=15"},
			},
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
			req.Header = tc.headers

			resp, err := testServer.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)
			if tc.expectedResponseHeaders != nil {
				for header := range tc.expectedResponseHeaders {
					actualValue := resp.Header.Get(header)
					expectedValue := tc.expectedResponseHeaders.Get(header)

					assert.Equal(t, expectedValue, actualValue)
				}
			}

		})
	}
}

func TestHTTPServer_StreamIntegration(t *testing.T) {
	if !testing.Short() {
		t.Skipf("skipping test %s, requires redis & pushpin to be running and will only be run if the -short flag is passed", t.Name())
	}

	const (
		apiKey1     = "apikey1"
		apiKey2     = "apikey2"
		apiKey3     = "apikey3"
		apiKey1Hash = "d4f79b313f8106f5af108ad96ff516222dbfd5a0ab52f4308e4b1ad1d740de60"
		apiKey2Hash = "15fac8fa1c99022568b008b9df07b04b45354ac5ca4740041d904cd3cf2b39e3"
		apiKey3Hash = "35ab1e0411c4cc6ecaaa676a4c7fef259798799ed40ad09fb07adae902bd0c7a"

		envID = "1234"
	)

	rc := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Make sure we start and finish with a fresh cache

	cache := cache.NewRedisCache(rc)

	authRepo, err := repository.NewAuthRepo(cache, map[domain.AuthAPIKey]string{
		domain.AuthAPIKey(apiKey1Hash): envID,
		domain.AuthAPIKey(apiKey2Hash): envID,
		domain.AuthAPIKey(apiKey3Hash): envID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(
		t,
		true,
		setupWithCache(cache),
		setupWithAuthRepo(authRepo),
	)
	testServer := httptest.NewUnstartedServer(server)

	// Configure the test server to listen on port 8000
	// which is where pushpin proxies request to
	l, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		t.Fatal(err)
	}
	testServer.Listener = l
	testServer.Start()
	defer testServer.Close()

	logger := log.NewNoOpLogger()

	sdkEvents := []sdkstream.Event{
		{
			Environment: envID,
			SSEEvent: &sse.Event{
				ID:   []byte("1"),
				Data: []byte("hello world"),
			},
		},
		{
			Environment: envID,
			SSEEvent: &sse.Event{
				ID:   []byte("2"),
				Data: []byte("foo bar"),
			},
		},
		{
			Environment: envID,
			SSEEvent: &sse.Event{
				ID:   []byte("3"),
				Data: []byte("fizz buzz"),
			},
		},
		// Send a message with an EOF string in the data so as we have some
		// way to signal in our test that this is the last event and we're done
		{
			Environment: envID,
			SSEEvent: &sse.Event{
				ID:   []byte("4"),
				Data: []byte("EOF"),
			},
		},
	}

	expectedSSEEvents := []*sse.Event{
		{
			Event: []byte("*"),
			Data:  []byte("hello world"),
		},
		{
			Event: []byte("*"),
			Data:  []byte("foo bar"),
		},
		{
			Event: []byte("*"),
			Data:  []byte("fizz buzz"),
		},
	}

	testCases := map[string]struct {
		apiKeys            []string
		topics             []string
		numClients         int
		sdkEvents          []sdkstream.Event
		expectedEvents     []*sse.Event
		expectedStatusCode int
	}{
		"Given I have one client that makes a stream request and three events come in via the sdk": {
			apiKeys:            []string{apiKey1},
			topics:             []string{envID},
			numClients:         1,
			sdkEvents:          sdkEvents,
			expectedEvents:     expectedSSEEvents,
			expectedStatusCode: http.StatusOK,
		},
		"Given I have three clients making the same stream request and three events comes in via the sdk": {
			apiKeys:            []string{apiKey1},
			topics:             []string{envID},
			numClients:         3,
			sdkEvents:          sdkEvents,
			expectedEvents:     expectedSSEEvents,
			expectedStatusCode: http.StatusOK,
		},
		"Given I have three clients making stream requests with different apiKeys and three events comes in via the sdk": {
			apiKeys:            []string{apiKey1, apiKey2, apiKey3},
			topics:             []string{envID},
			numClients:         3,
			sdkEvents:          sdkEvents,
			expectedEvents:     expectedSSEEvents,
			expectedStatusCode: http.StatusOK,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			gpc := gripcontrol.NewGripPubControl([]map[string]interface{}{
				{
					"control_uri": "http://localhost:5561",
				},
			})

			streamWorker := stream.NewStreamWorker(logger, gpc)

			requests := []*http.Request{}
			for _, apiKey := range tc.apiKeys {
				req, err := http.NewRequest(http.MethodGet, "http://localhost:7000/stream", nil)
				if err != nil {
					t.Errorf("(%s) failed to create request: %s", desc, err)
				}
				req.Header.Add("API-Key", apiKey)
				requests = append(requests, req)
			}

			responseBodies := map[string]io.Reader{}

			for _, req := range requests {
				for i := 1; i <= tc.numClients; i++ {
					resp, err := testServer.Client().Do(req)
					if err != nil {
						t.Errorf("(%s) failed making stream response: %s", desc, err)
					}
					defer resp.Body.Close()

					t.Logf("Then the status code with be %d", tc.expectedStatusCode)
					assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

					key := fmt.Sprintf("client-%d", i)
					responseBodies[key] = resp.Body
				}
			}

			t.Log("And when the EventListener receives an event from the SDK")
			// Now that we've got an open stream with the server we can mimic
			// events from the embedded SDK coming in
			for _, apiKey := range tc.apiKeys {
				for _, sdkEvent := range tc.sdkEvents {
					sdkEvent.APIKey = apiKey
					if err := streamWorker.Pub(ctx, sdkEvent); err != nil {
						t.Errorf("(%s) eventListener failed to publish mocked sse event from sdk: %s", desc, err)
					}
				}
			}

			for key, body := range responseBodies {
				// And then we should expect to see these events being written to
				// the response body
				sseReader := sse.NewEventStreamReader(body)
				actualEvents := []*sse.Event{}

				done := false
				for !done {
					b, err := sseReader.ReadEvent()
					if err != nil {
						t.Errorf("(%s) failed reading sse event stream: %s", desc, err)
					}

					actualSSEEvent, err := parseRawSSEEvent(b)
					if err != nil {
						t.Errorf("(%s) failed parsing sse event: %s", desc, err)
					}

					if reflect.DeepEqual(actualSSEEvent.Data, []byte("EOF")) {
						done = true
						continue
					}

					actualEvents = append(actualEvents, actualSSEEvent)
				}
				cancel()

				t.Logf("Then %s will recieve the event(s)", key)
				assert.ElementsMatch(t, tc.expectedEvents, actualEvents)

			}
		})
	}
}

func parseRawSSEEvent(msg []byte) (event *sse.Event, err error) {
	var e sse.Event

	var (
		headerID    = []byte("id:")
		headerData  = []byte("data:")
		headerEvent = []byte("event:")
		headerRetry = []byte("retry:")
	)

	if len(msg) < 1 {
		return nil, errors.New("event message was empty")
	}

	// Normalize the crlf to lf to make it easier to split the lines.
	bytes.Replace(msg, []byte("\n\r"), []byte("\n"), -1)
	// Split the line by "\n" or "\r", per the spec.
	for _, line := range bytes.FieldsFunc(msg, func(r rune) bool { return r == '\n' || r == '\r' }) {
		switch {
		case bytes.HasPrefix(line, headerID):
			e.ID = append([]byte(nil), trimHeader(len(headerID), line)...)
		case bytes.HasPrefix(line, headerData):
			// The spec allows for multiple data fields per event, concatenated them with "\n".
			e.Data = append(append(trimHeader(len(headerData), line), e.Data[:]...), byte('\n'))
		// The spec says that a line that simply contains the string "data" should be treated as a data field with an empty body.
		case bytes.Equal(line, bytes.TrimSuffix(headerData, []byte(":"))):
			e.Data = append(e.Data, byte('\n'))
		case bytes.HasPrefix(line, headerEvent):
			e.Event = append([]byte(nil), trimHeader(len(headerEvent), line)...)
		case bytes.HasPrefix(line, headerRetry):
			e.Retry = append([]byte(nil), trimHeader(len(headerRetry), line)...)
		default:
			// Ignore any garbage that doesn't match what we're looking for.
		}
	}

	// Trim the last "\n" per the spec.
	e.Data = bytes.TrimSuffix(e.Data, []byte("\n"))
	return &e, err
}

func trimHeader(size int, data []byte) []byte {
	data = data[size:]
	// Remove optional leading whitespace
	if data[0] == 32 {
		data = data[1:]
	}
	// Remove trailing new line
	if len(data) > 0 && data[len(data)-1] == 10 {
		data = data[:len(data)-1]
	}
	return data
}
