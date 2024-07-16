package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/harness-community/sse/v3"
	sdkstream "github.com/harness/ff-golang-server-sdk/stream"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	jsoniter "github.com/json-iterator/go"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/config/local"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/hash"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/harness/ff-proxy/v2/middleware"
	proxyservice "github.com/harness/ff-proxy/v2/proxy-service"
	"github.com/harness/ff-proxy/v2/repository"
	"github.com/harness/ff-proxy/v2/token"
)

type mockSDKClient struct {
	data map[string]bool
}

func (n *mockSDKClient) StreamConnected(key string) bool {
	v, ok := n.data[key]
	if !ok {
		return false
	}
	return v
}

func boolPtr(b bool) *bool {
	return &b
}

type mockClientService struct {
	authenticate func(target domain.Target) (domain.Target, error)
	targetsc     chan domain.Target
}

type mockMetricService struct {
	storeMetrics func(metrics domain.MetricsRequest) error
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
	return m.storeMetrics(req)
}

const (
	apiKey1         = "apikey1"
	envID123        = "1234"
	hashedAPIKey123 = "486089aa445aa0d9ee898f4f38dec4b0d1ee69da3ed7697afb1bdcd46f3fc5ec"
	apiKey123Token  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbnZpcm9ubWVudCI6ImVudi0xMjMiLCJpYXQiOjE2MzcwNTUyMjUsIm5iZiI6MTYzNzA1NTIyNX0.scUWHotiphYV_xYr3UvEkaUw9CuHQnFThcq3CpPkmu8"
)

type setupConfig struct {
	featureRepo       *repository.FeatureFlagRepo
	targetRepo        *repository.TargetRepo
	segmentRepo       *repository.SegmentRepo
	authRepo          *repository.AuthRepo
	cacheHealthFn     func(ctx context.Context) domain.HealthResponse
	clientService     *mockClientService
	eventListener     sdkstream.EventStreamListener
	cache             cache.Cache
	metricService     *mockMetricService
	promReg           prometheusRegister
	sdkClients        *mockSDKClient
	healthFn          func(ctx context.Context) domain.HealthResponse
	healthySaasStream func() bool
	andRulesEnabled   bool
	port              int
}

type setupOpts func(s *setupConfig)

func setupWithSDKClients(m *mockSDKClient) setupOpts {
	return func(s *setupConfig) {
		s.sdkClients = m
	}
}

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

func setupHealthFn(fn func(ctx context.Context) domain.HealthResponse) setupOpts {
	return func(s *setupConfig) {
		s.healthFn = fn
	}
}

func setupWithCache(c cache.Cache) setupOpts {
	return func(s *setupConfig) {
		s.cache = c
	}
}

func setupWithHealthySaasStream(fn func() bool) setupOpts {
	return func(s *setupConfig) {
		s.healthySaasStream = fn
	}
}

func setupWithAndRulesEnabled(enabled bool) setupOpts {
	return func(s *setupConfig) {
		s.andRulesEnabled = enabled
	}
}

func setupWithPort(port int) setupOpts {
	return func(s *setupConfig) {
		s.port = port
	}
}

// setupHTTPServer is a helper that loads test config for populating the repos
// and injects all the required dependencies into the proxy service and http server
func setupHTTPServer(t *testing.T, bypassAuth bool, opts ...setupOpts) *HTTPServer {
	fileSystem := os.DirFS("../config/local/test")
	config, err := local.NewConfig(fileSystem)
	if err != nil {
		t.Fatal(err)
	}

	setupConfig := &setupConfig{}
	for _, opt := range opts {
		opt(setupConfig)
	}

	if setupConfig.cache == nil {
		mr, err := miniredis.Run()
		if err != nil {
			panic(err)
		}

		redisClient := redis.NewClient(&redis.Options{
			Addr: mr.Addr(),
		})

		setupConfig.cache = cache.NewMemoizeCache(redisClient, 1*time.Minute, 2*time.Minute, nil)
	}

	if setupConfig.featureRepo == nil {
		fr := repository.NewFeatureFlagRepo(setupConfig.cache)

		setupConfig.featureRepo = &fr
	}

	if setupConfig.targetRepo == nil {
		tr := repository.NewTargetRepo(setupConfig.cache, log.NewNoOpLogger())

		setupConfig.targetRepo = &tr
	}

	if setupConfig.segmentRepo == nil {
		sr := repository.NewSegmentRepo(setupConfig.cache)

		setupConfig.segmentRepo = &sr
	}

	if setupConfig.authRepo == nil {
		ar := repository.NewAuthRepo(setupConfig.cache)
		setupConfig.authRepo = &ar
	}

	if setupConfig.clientService == nil {
		setupConfig.clientService = &mockClientService{authenticate: func(t domain.Target) (domain.Target, error) {
			return t, nil
		}}
	}

	if setupConfig.metricService == nil {
		setupConfig.metricService = &mockMetricService{storeMetrics: func(metrics domain.MetricsRequest) error {
			return nil
		}}
	}

	if setupConfig.healthFn == nil {
		setupConfig.healthFn = func(ctx context.Context) domain.HealthResponse {
			return domain.HealthResponse{
				ConfigStatus: domain.ConfigStatus{
					State: domain.ConfigStateSynced,
					Since: 1699877509155,
				},
				StreamStatus: domain.StreamStatus{
					State: domain.StreamStateConnected,
					Since: 1699877509155,
				},
				CacheStatus: "healthy",
			}
		}
	}

	if setupConfig.sdkClients == nil {
		setupConfig.sdkClients = &mockSDKClient{data: make(map[string]bool)}
	}

	if setupConfig.healthySaasStream == nil {
		setupConfig.healthySaasStream = func() bool { return true }
	}

	if setupConfig.port == 0 {
		setupConfig.port = 8000
	}

	logger := log.NoOpLogger{}

	tokenSource := token.NewSource(logger, setupConfig.authRepo, hash.NewSha256(), []byte(`secret`))

	err = config.Populate(context.Background(), setupConfig.authRepo, setupConfig.featureRepo, setupConfig.segmentRepo)
	assert.Nil(t, err)

	var service proxyservice.ProxyService
	service = proxyservice.NewService(proxyservice.Config{
		Logger:             log.NewNoOpContextualLogger(),
		FeatureRepo:        *setupConfig.featureRepo,
		TargetRepo:         *setupConfig.targetRepo,
		SegmentRepo:        *setupConfig.segmentRepo,
		AuthRepo:           *setupConfig.authRepo,
		Health:             setupConfig.healthFn,
		AuthFn:             tokenSource.GenerateToken,
		ClientService:      setupConfig.clientService,
		MetricStore:        setupConfig.metricService,
		Offline:            false,
		Hasher:             hash.NewSha256(),
		HealthySaasStream:  setupConfig.healthySaasStream,
		AndRulesEnabled:    setupConfig.andRulesEnabled,
		SDKStreamConnected: func(envID string) {},
		ForwardTargets:     true,
	})
	endpoints := NewEndpoints(service)

	repo := mockRepo{
		getFn: func(context context.Context, key domain.AuthAPIKey) (string, bool, error) {
			return "", true, nil
		},
	}

	server := NewHTTPServer(setupConfig.port, endpoints, logger, false, "", "")
	server.Use(
		middleware.NewPrometheusMiddleware(prometheus.NewRegistry()),
		middleware.NewCorsMiddleware(),
		middleware.AllowQuerySemicolons(),
		middleware.NewEchoRequestIDMiddleware(),
		middleware.NewEchoLoggingMiddleware(logger),
		middleware.NewEchoAuthMiddleware(logger, repo, []byte(`secret`), bypassAuth),
		middleware.ValidateEnvironment(bypassAuth),
	)
	return server
}

// variationToTargetMap:null is intentional here - refer to FFM-3246 before removing
// featureConfig is the expected response body for a FeatureConfigs request - the newline at the end is intentional
var featureConfig = []byte(`[{"defaultServe":{"variation":"true"},"environment":"featureflagsqa","feature":"harnessappdemodarkmode","kind":"boolean","offVariation":"false","prerequisites":[],"project":"FeatureFlagsQADemo","rules":[{"clauses":[{"attribute":"age","id":"79f5bca0-17ca-42c2-8934-5cee840fe2e0","negate":false,"op":"equal","values":["55"]}],"priority":1,"ruleId":"8756c207-abf8-4202-83fd-dedf5d27e2c2","serve":{"variation":"false"}}],"state":"on","variationToTargetMap":[{"targetSegments":["flagsTeam"],"targets":[{"identifier":"davej","name":"Dave Johnston"}],"variation":"false"}],"variations":[{"identifier":"true","name":"True","value":"true"},{"identifier":"false","name":"False","value":"false"}],"version":568},{"defaultServe":{"variation":"1"},"environment":"featureflagsqa","feature":"yet_another_flag","kind":"string","offVariation":"2","prerequisites":[],"project":"FeatureFlagsQADemo","rules":[],"state":"on","variationToTargetMap":[],"variations":[{"identifier":"1","name":"1","value":"1"},{"identifier":"2","name":"2","value":"2"}],"version":6}]
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
			expectedResponseBody: featureConfig,
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
var harnessAppDemoDarkMode = []byte(`{"defaultServe":{"variation":"true"},"environment":"featureflagsqa","feature":"harnessappdemodarkmode","kind":"boolean","offVariation":"false","prerequisites":[],"project":"FeatureFlagsQADemo","rules":[{"clauses":[{"attribute":"age","id":"79f5bca0-17ca-42c2-8934-5cee840fe2e0","negate":false,"op":"equal","values":["55"]}],"priority":1,"ruleId":"8756c207-abf8-4202-83fd-dedf5d27e2c2","serve":{"variation":"false"}}],"state":"on","variationToTargetMap":[{"targetSegments":["flagsTeam"],"targets":[{"identifier":"davej","name":"Dave Johnston"}],"variation":"false"}],"variations":[{"identifier":"true","name":"True","value":"true"},{"identifier":"false","name":"False","value":"false"}],"version":568}
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

// targetSegments is the expected response body for a TargetSegments request with rules=v2 param - the newline at the end is intentional
var targetSegmentsWithServingRules = []byte(`[{"createdAt":123,"environment":"featureflagsqa","excluded":[],"identifier":"flagsTeam","included":[],"modifiedAt":456,"name":"flagsTeam","servingRules":[{"clauses":[{"attribute":"ip","id":"31c18ee7-8051-44cc-8507-b44580467ee5","negate":false,"op":"equal","values":["2a00:23c5:b672:2401:158:f2a6:67a0:6a79"]}],"priority":1,"ruleId":"990a58a4-8ff8-4376-ae6e-a95d10387c4c"}],"version":1}]
`)

// TestHTTPServer_GetTargetSegments sets up a service with repositories
// populated from config/test, injects it into the HTTPServer and makes HTTP
// requests to the /client/env/{environmentUUID}/target-segments endpoint
func TestHTTPServer_GetTargetSegments(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// setup HTTPServer & service with auth bypassed and AND rules enabled
	andRulesServer := setupHTTPServer(t, true, setupWithAndRulesEnabled(true), setupWithPort(8001))
	andRulesTestServer := httptest.NewServer(andRulesServer)
	defer andRulesTestServer.Close()

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
		"Given I make GET request for an environment and identifier that exist with andRulesEnabled false without rules=v2 param only returns rules": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetSegments,
		},
		"Given I make GET request for an environment and identifier that exist with andRulesEnabled false and rules=v2 only returns rules": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments?rules=v2", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetSegments,
		},
		"Given I make GET request for an environment and identifier that exist with andRulesEnabled true without rules=v2 param only returns rules": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments", andRulesTestServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetSegments,
		},
		"Given I make GET request for an environment and identifier that exist with with andRulesEnabled true and rules=v2 only returns serving rules": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments?rules=v2", andRulesTestServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetSegmentsWithServingRules,
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

// segmentFlagsTeamWithServingRules is the expected response body for a TargetSegmentsByIdentfier request where identifer='flagsTeam' and rules=v2 - the newline at the end is intentional
var segmentFlagsTeamWithServingRules = []byte(`{"createdAt":123,"environment":"featureflagsqa","excluded":[],"identifier":"flagsTeam","included":[],"modifiedAt":456,"name":"flagsTeam","servingRules":[{"clauses":[{"attribute":"ip","id":"31c18ee7-8051-44cc-8507-b44580467ee5","negate":false,"op":"equal","values":["2a00:23c5:b672:2401:158:f2a6:67a0:6a79"]}],"priority":1,"ruleId":"990a58a4-8ff8-4376-ae6e-a95d10387c4c"}],"version":1}
`)

// TestHTTPServer_GetTargetSegmentsByIdentifier sets up a service with repositories
// populated from config/test, injects it into the HTTPServer and makes HTTP
// requests to the /client/env/{environmentUUID}/target-segments/{identifier} endpoint
func TestHTTPServer_GetTargetSegmentsByIdentifier(t *testing.T) {
	// setup HTTPServer & service with auth bypassed
	server := setupHTTPServer(t, true)
	testServer := httptest.NewServer(server)
	defer testServer.Close()

	// setup HTTPServer & service with auth bypassed and AND rules enabled
	andRulesServer := setupHTTPServer(t, true, setupWithAndRulesEnabled(true), setupWithPort(8001))
	andRulesTestServer := httptest.NewServer(andRulesServer)
	defer andRulesTestServer.Close()

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
		"Given I make GET request for an environment that doesn't exist": {
			method:             http.MethodGet,
			url:                fmt.Sprintf("%s/client/env/noexist/target-segments/james", testServer.URL),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for an environment and identifier that exist with andRulesEnabled false without rules=v2 param only returns rules": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments/flagsTeam", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: segmentFlagsTeam,
		},
		"Given I make GET request for an environment and identifier that exist with andRulesEnabled false and rules=v2 only returns rules": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments/flagsTeam?rules=v2", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: segmentFlagsTeam,
		},
		"Given I make GET request for an environment and identifier that exist with andRulesEnabled true without rules=v2 param only returns rules": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments/flagsTeam", andRulesTestServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: segmentFlagsTeam,
		},
		"Given I make GET request for an environment and identifier that exist with andRulesEnabled true and rules=v2 only returns rules": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target-segments/flagsTeam?rules=v2", andRulesTestServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: segmentFlagsTeamWithServingRules,
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

	targetDavejEvaluations = []byte(`[{"flag":"harnessappdemodarkmode","identifier":"false","kind":"boolean","value":"false"},{"flag":"yet_another_flag","identifier":"1","kind":"string","value":"1"}]
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

	target := domain.Target{
		Target: clientgen.Target{
			Attributes: domain.ToPtr(map[string]interface{}{
				"email": "foo@gmail.com",
			}),
			Identifier: "davej",
			Name:       "Dave Johnson",
		},
	}

	b, err := jsoniter.Marshal(target)
	assert.Nil(t, err)

	encodedTarget := base64.StdEncoding.EncodeToString(b)

	badlyEncodedTarget := base64.StdEncoding.EncodeToString([]byte(`{"identifier": "foo", "name": "foo"`))

	testCases := map[string]struct {
		method               string
		url                  string
		headers              map[string]string
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/env/1234/target/james/evaluations", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		// we return an empty array for this right now because we can't tell the difference between
		// an environment not existing at all and there just being no flags in it
		"Given I make GET request for an environment that doesn't exist": {
			method: http.MethodGet,
			url:    fmt.Sprintf("%s/client/env/abcd/target/james/evaluations", testServer.URL),
			expectedResponseBody: []byte(`[]
`),
			expectedStatusCode: http.StatusOK,
		},
		"Given I make GET request for target that doesn't exist": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/bar/evaluations", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetFooEvaluations,
		},
		// TODO - commented out due to an issue with the new go sdk evaluator in evaluating certain attribute based rules on flags
		// these rules are now deprecated and can't be created from the UI but we should upgrade once the go sdk fixes this issue
		//"Given I make GET request for an environment and the target 'james'": {
		//	method:               http.MethodGet,
		//	url:                  fmt.Sprintf("%s/client/env/1234/target/james/evaluations", testServer.URL),
		//	expectedStatusCode:   http.StatusOK,
		//	expectedResponseBody: targetJamesEvaluations,
		//},
		"Given I make GET request for an environment and the target 'foo'": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/foo/evaluations", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetFooEvaluations,
		},
		"Given I make a GET request with a valid Target in the Harness-Target header then the Target in the header will be used for evaluations": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/foo/evaluations", testServer.URL),
			headers:              map[string]string{targetHeader: encodedTarget},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetDavejEvaluations,
		},
		"Given I make a GET request with an invalid Target in the Harness-Target, then the target from the path will be used": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/foo/evaluations", testServer.URL),
			headers:              map[string]string{targetHeader: badlyEncodedTarget},
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

			for h, v := range tc.headers {
				req.Header.Set(h, v)
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

	targetDavejDarkModeEvaluation = []byte(`{"flag":"harnessappdemodarkmode","identifier":"false","kind":"boolean","value":"false"}
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

	target := domain.Target{
		Target: clientgen.Target{
			Attributes: domain.ToPtr(map[string]interface{}{
				"email": "foo@gmail.com",
			}),
			Identifier: "davej",
			Name:       "Dave Johnson",
		},
	}

	b, err := jsoniter.Marshal(target)
	assert.Nil(t, err)

	encodedTarget := base64.StdEncoding.EncodeToString(b)

	badlyEncodedTarget := base64.StdEncoding.EncodeToString([]byte(`{"identifier": "foo", "name": "foo"`))

	testCases := map[string]struct {
		method               string
		url                  string
		headers              map[string]string
		expectedStatusCode   int
		expectedResponseBody []byte
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			url:                fmt.Sprintf("%s/client/env/1234/target/james/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make GET request for an environment that doesn't exist": {
			method: http.MethodGet,
			url:    fmt.Sprintf("%s/client/env/abcd/target/james/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedResponseBody: []byte(`{"error":"not found"}
`),
			expectedStatusCode: http.StatusNotFound,
		},
		"Given I make GET request for target that doesn't exist": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/bar/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: darkModeEvaluationTrue,
		},
		// TODO - commented out due to an issue with the new go sdk evaluator in evaluating certain attribute based rules on flags
		// these rules are now deprecated and can't be created from the UI but we should upgrade once the go sdk fixes this issue
		//"Given I make GET request for an environment and the target 'james'": {
		//	method:               http.MethodGet,
		//	url:                  fmt.Sprintf("%s/client/env/1234/target/james/evaluations/harnessappdemodarkmode", testServer.URL),
		//	expectedStatusCode:   http.StatusOK,
		//	expectedResponseBody: darkModeEvaluationFalse,
		//},
		"Given I make GET request for an environment and the target 'foo'": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/foo/evaluations/harnessappdemodarkmode", testServer.URL),
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: darkModeEvaluationTrue,
		},
		"Given I make a GET request with a valid Target in the Harness-Target header then the Target in the header will be used for evaluations": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/foo/evaluations/harnessappdemodarkmode", testServer.URL),
			headers:              map[string]string{targetHeader: encodedTarget},
			expectedStatusCode:   http.StatusOK,
			expectedResponseBody: targetDavejDarkModeEvaluation,
		},
		"Given I make a GET request with an invalid Target in the Harness-Target, then the target from the path will be used": {
			method:               http.MethodGet,
			url:                  fmt.Sprintf("%s/client/env/1234/target/foo/evaluations/harnessappdemodarkmode", testServer.URL),
			headers:              map[string]string{targetHeader: badlyEncodedTarget},
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

			for h, v := range tc.headers {
				req.Header.Set(h, v)
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

	targets := []domain.Target{
		{Target: clientgen.Target{
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
		"Given I make an auth request with an invalid Target Identifier": {
			method:             http.MethodPost,
			body:               []byte(fmt.Sprintf(`{"apiKey": "%s", "target": {"identifier": "hello world"}}`, apiKey1)),
			expectedStatusCode: http.StatusBadRequest,
		},
		"Given I make an auth request with an invalid Target Name": {
			method:             http.MethodPost,
			body:               []byte(fmt.Sprintf(`{"apiKey": "%s", "target": {"identifier": "helloworld", "name": "Hello/World"}}`, apiKey1)),
			expectedStatusCode: http.StatusBadRequest,
		},
	}

	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	for desc, tc := range testCases {
		tc := tc

		targetRepo := repository.NewTargetRepo(cache.NewMemoizeCache(redisClient, 1*time.Minute, 2*time.Minute, nil), log.NewNoOpLogger())

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

				cacheTargets, err := targetRepo.Get(context.Background(), envID123)
				if err != nil {
					t.Errorf("failed to get targets from cache: %s", err)
				}

				cacheTarget, err := targetRepo.GetByIdentifier(context.Background(), envID123, tc.expectedCacheTargets[0].Identifier)
				assert.Nil(t, err)

				assert.Equal(t, tc.expectedCacheTargets[0], cacheTarget)

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
	healthyResponse := []byte(`{"configStatus":{"state":"SYNCED","since":1699877509155},"streamStatus":{"state":"CONNECTED","since":1699877509155},"cacheStatus":"healthy"}
`)

	unhealthyResponse := []byte(`{"error":"internal error"}
`)

	healthErr := func(ctx context.Context) domain.HealthResponse {
		return domain.HealthResponse{
			ConfigStatus: domain.ConfigStatus{
				State: domain.ConfigStateFailedToSync,
				Since: 1709648163438,
			},
			StreamStatus: domain.StreamStatus{
				State: domain.StreamStateDisconnected,
				Since: 1699877509155,
			},
			CacheStatus: "unhealthy",
		}
	}

	testCases := map[string]struct {
		method               string
		url                  string
		healthFn             func(ctx context.Context) domain.HealthResponse
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
			expectedStatusCode:   http.StatusInternalServerError,
			healthFn:             healthErr,
			expectedResponseBody: unhealthyResponse,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		// setup HTTPServer & service with auth bypassed
		server := setupHTTPServer(t, true,
			setupHealthFn(tc.healthFn),
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

			//assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			if tc.expectedResponseBody != nil {
				actual, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("(%s): failed to read response body: %s", desc, err)
				}
				assert.Equal(t, string(tc.expectedResponseBody), string(actual))

				if !assert.ElementsMatch(t, tc.expectedResponseBody, actual) {
					t.Errorf("(%s) expected: %s \n got: %s ", desc, tc.expectedResponseBody, actual)
				}
			}
		})
	}
}

func TestHTTPServer_Stream(t *testing.T) {
	const (
		apiKey       = "apikey1"
		hashedAPIKey = "d4f79b313f8106f5af108ad96ff516222dbfd5a0ab52f4308e4b1ad1d740de60"

		apiKey2       = "apikey2"
		hashedAPIKey2 = "17ac3d5d395c9ac2f2685cb75229b5faedd7d207b31516bf2b506c1598fd55ef"

		envID = "1234"
		env2  = "5678"
	)

	authRepo := repository.NewAuthRepo(cache.NewMemCache())

	healthySaasStream := func() bool {
		return true
	}

	unhealthySaasStream := func() bool {
		return false
	}

	testCases := map[string]struct {
		method                  string
		headers                 http.Header
		healthySaasStream       func() bool
		expectedStatusCode      int
		expectedResponseHeaders http.Header
	}{
		"Given I make a request that isn't a GET request": {
			method:             http.MethodPost,
			headers:            http.Header{},
			healthySaasStream:  healthySaasStream,
			expectedStatusCode: http.StatusMethodNotAllowed,
		},
		"Given I make a GET request but don't have an API-Key header": {
			method:             http.MethodGet,
			headers:            http.Header{},
			healthySaasStream:  healthySaasStream,
			expectedStatusCode: http.StatusBadRequest,
		},
		"Given I make a GET request and have an empty API Key header": {
			method: http.MethodGet,
			headers: http.Header{
				"API-Key": []string{},
			},
			healthySaasStream:  healthySaasStream,
			expectedStatusCode: http.StatusBadRequest,
		},
		"Given I make a GET request and with an API Key that isn't in the AuthConfig": {
			method: http.MethodGet,
			headers: http.Header{
				"API-Key": []string{"foobar"},
			},
			healthySaasStream:  healthySaasStream,
			expectedStatusCode: http.StatusNotFound,
		},

		// Disable these tests temporarily until we implement new stream client

		"Given I make a GET request with a valid API Key Header": {
			method: http.MethodGet,
			headers: http.Header{
				"API-Key": []string{apiKey},
			},
			healthySaasStream:  healthySaasStream,
			expectedStatusCode: http.StatusOK,
			expectedResponseHeaders: http.Header{
				"Content-Type":    []string{"text/event-stream"},
				"Grip-Hold":       []string{"stream"},
				"Grip-Channel":    []string{envID},
				"Grip-Keep-Alive": []string{":\\n\\n; format=cstring; timeout=15"},
			},
		},

		"Given I make a GET request with an API Key Header for a stream that isn't connected": {
			method: http.MethodGet,
			headers: http.Header{
				"API-Key": []string{"apiKey2"},
			},
			healthySaasStream:  unhealthySaasStream,
			expectedStatusCode: http.StatusServiceUnavailable,
			expectedResponseHeaders: http.Header{
				"Content-Type": []string{"application/json; charset=UTF-8"},
			},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			// setup HTTPServer & service with auth bypassed
			server := setupHTTPServer(t, true,
				setupWithAuthRepo(authRepo),
				setupWithSDKClients(&mockSDKClient{
					data: map[string]bool{
						"1234": true,
						"5678": false,
					},
				}),
				setupWithHealthySaasStream(tc.healthySaasStream),
			)
			testServer := httptest.NewServer(server)
			defer testServer.Close()

			var req *http.Request
			var err error

			url := fmt.Sprintf("%s/stream", testServer.URL)

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

func TestHTTPServer_WithCustomHandler(t *testing.T) {
	type args struct {
		method  string
		route   string
		handler http.Handler
	}

	type mocks struct {
	}

	type expected struct {
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given I try to register a custom handler on /client/auth": {
			args: args{
				method:  http.MethodGet,
				route:   domain.AuthRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /health": {
			args: args{
				method:  http.MethodGet,
				route:   domain.HealthRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /feature-configs": {
			args: args{
				method:  http.MethodGet,
				route:   domain.FeatureConfigsRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /feature-configs/:identifier": {
			args: args{
				method:  http.MethodGet,
				route:   domain.FeatureConfigsIdentifierRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /target-segments": {
			args: args{
				method:  http.MethodGet,
				route:   domain.SegmentsRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /target-segments/:identifier": {
			args: args{
				method:  http.MethodGet,
				route:   domain.SegmentsRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /evaluations": {
			args: args{
				method:  http.MethodGet,
				route:   domain.EvaluationsRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /evaluations/:feature": {
			args: args{
				method:  http.MethodGet,
				route:   domain.EvaluationsFlagRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /metrics/:environmentUUID": {
			args: args{
				method:  http.MethodPost,
				route:   domain.MetricsRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /stream": {
			args: args{
				method:  http.MethodGet,
				route:   domain.StreamRoute,
				handler: nil,
			},
			shouldErr: true,
		},
		"Given I try to register a custom handler on /metrics": {
			args: args{
				method:  http.MethodGet,
				route:   "/metrics",
				handler: nil,
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			server := &HTTPServer{
				router: echo.New(),
			}

			err := server.WithCustomHandler(tc.args.method, tc.args.route, tc.args.handler)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

//func TestHTTPServer_StreamIntegration(t *testing.T) {
//	// Skip this test until we've implemented the new streaming logic
//	t.Skip()
//
//	if !testing.Short() {
//		t.Skipf("skipping test %s, requires redis & pushpin to be running and will only be run if the -short flag is passed", t.Name())
//	}
//
//	const (
//		apiKey1     = "apikey1"
//		apiKey2     = "apikey2"
//		apiKey3     = "apikey3"
//		apiKey1Hash = "d4f79b313f8106f5af108ad96ff516222dbfd5a0ab52f4308e4b1ad1d740de60"
//		apiKey2Hash = "15fac8fa1c99022568b008b9df07b04b45354ac5ca4740041d904cd3cf2b39e3"
//		apiKey3Hash = "35ab1e0411c4cc6ecaaa676a4c7fef259798799ed40ad09fb07adae902bd0c7a"
//
//		envID = "1234"
//	)
//
//	rc := redis.NewClient(&redis.Options{
//		Addr: "localhost:6379",
//	})
//
//	// Make sure we start and finish with a fresh cache
//	cache := cache.NewKeyValCache(rc, cache.WithMarshalFunc(json.Marshal), cache.WithUnmarshalFunc(json.Unmarshal))
//
//	authRepo := repository.NewAuthRepo(cache)
//
//	// setup HTTPServer & service with auth bypassed
//	server := setupHTTPServer(
//		t,
//		true,
//		setupWithCache(cache),
//		setupWithAuthRepo(authRepo),
//	)
//	testServer := httptest.NewUnstartedServer(server)
//
//	// Configure the test server to listen on port 8000
//	// which is where pushpin proxies request to
//	l, err := net.Listen("tcp", "127.0.0.1:8000")
//	if err != nil {
//		t.Fatal(err)
//	}
//	testServer.Listener = l
//	testServer.Start()
//	defer testServer.Close()
//
//	logger := log.NewNoOpLogger()
//
//	sdkEvents := []sdkstream.Event{
//		{
//			Environment: envID,
//			SSEEvent: &sse.Event{
//				ID:   []byte("1"),
//				Data: []byte("hello world"),
//			},
//		},
//		{
//			Environment: envID,
//			SSEEvent: &sse.Event{
//				ID:   []byte("2"),
//				Data: []byte("foo bar"),
//			},
//		},
//		{
//			Environment: envID,
//			SSEEvent: &sse.Event{
//				ID:   []byte("3"),
//				Data: []byte("fizz buzz"),
//			},
//		},
//		// Send a message with an EOF string in the data so as we have some
//		// way to signal in our test that this is the last event and we're done
//		{
//			Environment: envID,
//			SSEEvent: &sse.Event{
//				ID:   []byte("4"),
//				Data: []byte("EOF"),
//			},
//		},
//	}
//
//	expectedSSEEvents := []*sse.Event{
//		{
//			Event: []byte("*"),
//			Data:  []byte("hello world"),
//		},
//		{
//			Event: []byte("*"),
//			Data:  []byte("foo bar"),
//		},
//		{
//			Event: []byte("*"),
//			Data:  []byte("fizz buzz"),
//		},
//	}
//
//	testCases := map[string]struct {
//		apiKeys            []string
//		topics             []string
//		numClients         int
//		sdkEvents          []sdkstream.Event
//		expectedEvents     []*sse.Event
//		expectedStatusCode int
//	}{
//		"Given I have one client that makes a stream request and three events come in via the sdk": {
//			apiKeys:            []string{apiKey1},
//			topics:             []string{envID},
//			numClients:         1,
//			sdkEvents:          sdkEvents,
//			expectedEvents:     expectedSSEEvents,
//			expectedStatusCode: http.StatusOK,
//		},
//		"Given I have three clients making the same stream request and three events comes in via the sdk": {
//			apiKeys:            []string{apiKey1},
//			topics:             []string{envID},
//			numClients:         3,
//			sdkEvents:          sdkEvents,
//			expectedEvents:     expectedSSEEvents,
//			expectedStatusCode: http.StatusOK,
//		},
//		"Given I have three clients making stream requests with different apiKeys and three events comes in via the sdk": {
//			apiKeys:            []string{apiKey1, apiKey2, apiKey3},
//			topics:             []string{envID},
//			numClients:         3,
//			sdkEvents:          sdkEvents,
//			expectedEvents:     expectedSSEEvents,
//			expectedStatusCode: http.StatusOK,
//		},
//	}
//
//	for desc, tc := range testCases {
//		tc := tc
//		t.Run(desc, func(t *testing.T) {
//			ctx, cancel := context.WithCancel(context.Background())
//			defer cancel()
//
//			gpc := gripcontrol.NewGripPubControl([]map[string]interface{}{
//				{
//					"control_uri": "http://localhost:5561",
//				},
//			})
//
//			streamWorker := stream.NewWorker(logger, gpc, prometheus.NewRegistry())
//
//			requests := []*http.Request{}
//			for _, apiKey := range tc.apiKeys {
//				req, err := http.NewRequest(http.MethodGet, "http://localhost:7000/stream", nil)
//				if err != nil {
//					t.Errorf("(%s) failed to create request: %s", desc, err)
//				}
//				req.Header.Add("API-Key", apiKey)
//				requests = append(requests, req)
//			}
//
//			responseBodies := map[string]io.Reader{}
//
//			for _, req := range requests {
//				for i := 1; i <= tc.numClients; i++ {
//					resp, err := testServer.Client().Do(req)
//					if err != nil {
//						t.Errorf("(%s) failed making stream response: %s", desc, err)
//					}
//					defer resp.Body.Close()
//
//					t.Logf("Then the status code with be %d", tc.expectedStatusCode)
//					assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)
//
//					key := fmt.Sprintf("client-%d", i)
//					responseBodies[key] = resp.Body
//				}
//			}
//
//			t.Log("And when the EventListener receives an event from the SDK")
//			// Now that we've got an open stream with the server we can mimic
//			// events from the embedded SDK coming in
//			for _, apiKey := range tc.apiKeys {
//				for _, sdkEvent := range tc.sdkEvents {
//					sdkEvent.APIKey = apiKey
//					if err := streamWorker.Pub(ctx, sdkEvent); err != nil {
//						t.Errorf("(%s) eventListener failed to publish mocked sse event from sdk: %s", desc, err)
//					}
//				}
//			}
//
//			for key, body := range responseBodies {
//				// And then we should expect to see these events being written to
//				// the response body
//				sseReader := sse.NewEventStreamReader(body)
//				actualEvents := []*sse.Event{}
//
//				done := false
//				for !done {
//					b, err := sseReader.ReadEvent()
//					if err != nil {
//						t.Errorf("(%s) failed reading sse event stream: %s", desc, err)
//					}
//
//					actualSSEEvent, err := parseRawSSEEvent(b)
//					if err != nil {
//						t.Errorf("(%s) failed parsing sse event: %s", desc, err)
//					}
//
//					if reflect.DeepEqual(actualSSEEvent.Data, []byte("EOF")) {
//						done = true
//						continue
//					}
//
//					actualEvents = append(actualEvents, actualSSEEvent)
//				}
//				cancel()
//
//				t.Logf("Then %s will receive the event(s)", key)
//				assert.ElementsMatch(t, tc.expectedEvents, actualEvents)
//
//			}
//		})
//	}
//}

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

type mockRepo struct {
	getFn func(context context.Context, key domain.AuthAPIKey) (string, bool, error)
}

func (m mockRepo) Get(ctx context.Context, key domain.AuthAPIKey) (string, bool, error) {
	return m.getFn(ctx, key)
}
