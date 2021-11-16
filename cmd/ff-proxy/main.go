package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"github.com/sirupsen/logrus"
	"time"
	"github.com/wings-software/ff-server/pkg/hash"
	sdkCache "github.com/harness/ff-golang-server-sdk/cache"
	harness "github.com/harness/ff-golang-server-sdk/client"

	ffproxy "github.com/harness/ff-proxy"
	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/config"
	"github.com/harness/ff-proxy/domain"
	admingen "github.com/harness/ff-proxy/gen/admin"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/middleware"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
	"github.com/harness/ff-proxy/repository"
	"github.com/harness/ff-proxy/transport"
	
)

var (
	debug            bool
	bypassAuth       bool
	offline          bool
	host             string
	port             int
	accountIdentifer string
	orgIdentifier    string
	adminService     string
	serviceToken     string
	authSecret       string
	sdkBaseUrl   string
	sdkEventsUrl string
	environments []environment
)

type environment struct {
	sdkKey string
	environmentID string
}

func init() {
	flag.BoolVar(&bypassAuth, "bypass-auth", false, "bypasses authentication")
	flag.BoolVar(&debug, "debug", false, "enables debug logging")
	flag.BoolVar(&offline, "offline", false, "enables side loading of data from config dir")
	flag.StringVar(&host, "host", "localhost", "host of the proxy service")
	flag.IntVar(&port, "port", 7000, "port that the proxy service is exposed on")
	flag.StringVar(&accountIdentifer, "account-identifier", "zEaak-FLS425IEO7OLzMUg", "account identifier to load remote config for")
	flag.StringVar(&orgIdentifier, "org-identifier", "featureflagorg", "org identifier to load remote config for")
	flag.StringVar(&adminService, "admin-service", "https://qa.harness.io/gateway/cf", "the url of the admin service")
	flag.StringVar(&serviceToken, "service-token", "", "token to use with the ff service")
	flag.StringVar(&authSecret, "auth-secret", "secret", "the secret used for signing auth tokens")
	flag.StringVar(&sdkBaseUrl, "sdkBaseUrl", "https://config.feature-flags.uat.harness.io/api/1.0", "url for the sdk to connect to")
	flag.StringVar(&sdkEventsUrl, "sdkEventsUrl", "https://event.feature-flags.uat.harness.io/api/1.0", "url for the sdk to send metrics to")
	// TODO - read from config file
	// configure environments to connect to via sdks - these will populate the proxy cache
	environments = []environment{
		{
			sdkKey:        "9a2ab61e-adab-4634-ae53-fcc132cc9b25", // https://uat.harness.io/ng/#/account/AQ8xhfNCRtGIUjq5bSM8Fg/cf/orgs/FF_SDK_Tests/projects/FF_SDK_Test_Project/feature-flags?activeEnvironment=test_env
			environmentID: "4b72e8d2-4e6a-4bc0-abbc-e561c2e49531",
		},
		{
			sdkKey:        "b06c3bf4-9a5b-4cca-9b7b-e2b9063ea2d1", // https://uat.harness.io/ng/#/account/AQ8xhfNCRtGIUjq5bSM8Fg/cf/orgs/FF_SDK_Tests/projects/FF_SDK_Test_Project/feature-flags?activeEnvironment=learning_env
			environmentID: "d4487d24-a468-4ebd-a625-9235f877a3a0",
		},
	}


	flag.Parse()
}

func initFF(cache sdkCache.Cache, baseUrl, eventUrl, sdkKey string) {
	logger := log.NewLogger(os.Stderr, debug)

	client, err := harness.NewCfClient(sdkKey,
		harness.WithURL(baseUrl),
		harness.WithEventsURL(eventUrl),
		harness.WithStreamEnabled(true),
		harness.WithCache(cache),
		harness.WithStoreEnabled(false), // store should be disabled until we implement a wrapper to handle multiple envs
	)
	defer func() {
		if err := client.Close(); err != nil {
			logger.Error("error while closing client err: %v", err)
		}
	}()

	if err != nil {
		logger.Error("could not connect to CF servers %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(10 * time.Second)
			}
		}
	}()
	time.Sleep(5 * time.Minute)
	cancel()
}

func main() {
	// Setup logger
	logger := log.NewLogger(os.Stderr, debug)
	logger.Info("msg", "service config", "debug", debug, "bypass-auth", bypassAuth, "offline", offline, "host", host, "port", port, "admin-service", adminService, "account-identifier", accountIdentifer, "org-identifier", orgIdentifier)

	// Setup cancelation
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigc
		cancel()
	}()

	// Create a new admin client with a HTTP client that injects the adminServiceToken
	// into the auth header
	adminClient, err := admingen.NewClientWithResponses(
		adminService,
		admingen.WithHTTPClient(doer{c: http.DefaultClient, token: serviceToken}),
	)
	if err != nil {
		logger.Error("msg", "failed to create admin client", "err", err)
		os.Exit(1)
	}

	var (
		featureConfig map[domain.FeatureConfigKey][]domain.FeatureConfig
		targetConfig  map[domain.TargetKey][]domain.Target
		segmentConfig map[domain.SegmentKey][]domain.Segment
		authConfig    map[domain.AuthAPIKey]string
	)

	// Load either local config from files or remote config from ff-server
	if offline {
		config, err := config.NewLocalConfig(ffproxy.DefaultConfig, ffproxy.DefaultConfigDir)
		if err != nil {
			logger.Error("msg", "failed to load config", "err", err)
			os.Exit(1)
		}
		featureConfig = config.FeatureConfig()
		targetConfig = config.Targets()
		segmentConfig = config.Segments()
		authConfig = config.AuthConfig()

		logger.Info("msg", "retrieved offline config")
	} else {
		logger.Info("msg", "retrieving config from ff-server...")
		config := config.NewRemoteConfig(
			accountIdentifer,
			orgIdentifier,
			adminClient,
			config.WithLogger(logger),
			config.WithConcurrency(20),
		)

		authConfig, err = config.AuthConfig(ctx)
		if err != nil {
			logger.Error("msg", "failed to load auth config", "err", err)
			os.Exit(1)
		}
		logger.Info("msg", "successfully retrieved config from ff-server")
	}

	// Create cache
	memCache := cache.NewMemCache()

	// start an sdk instance per environment
	for _, env := range environments {
		// TODO - we should try and pass in the logger ^ to the sdk's - currently the interfaces don't line up
		cacheWrapper := cache.NewWrapper(&memCache, env.environmentID, logrus.New())
		go initFF(cacheWrapper, sdkBaseUrl, sdkEventsUrl, env.sdkKey)
	}

	// Create repos
	tr, err := repository.NewTargetRepo(memCache, targetConfig)
	if err != nil {
		logger.Error("msg", "failed to create target repo", "err", err)
		os.Exit(1)
	}

	fcr, err := repository.NewFeatureConfigRepo(memCache, featureConfig)
	if err != nil {
		logger.Error("msg", "failed to create feature config repo", "err", err)
		os.Exit(1)
	}

	sr, err := repository.NewSegmentRepo(memCache, segmentConfig)
	if err != nil {
		logger.Error("msg", "failed to create segment repo", "err", err)
		os.Exit(1)
	}

	authRepo := repository.NewAuthRepo(authConfig)
	tokenSource := ffproxy.NewTokenSource(logger, authRepo, hash.NewSha256(), []byte(authSecret))

	featureEvaluator := proxyservice.NewFeatureEvaluator()

	// Setup service and middleware
	var service proxyservice.ProxyService
	service = proxyservice.NewService(fcr, tr, sr, tokenSource.GenerateToken, featureEvaluator, logger)
	service = middleware.NewAuthMiddleware(tokenSource.ValidateToken, bypassAuth, service)
	service = middleware.NewLoggingMiddleware(logger, debug, service)

	// Configure endpoints and server
	endpoints := transport.NewEndpoints(service)
	server := transport.NewHTTPServer(host, port, endpoints, logger)

	go func() {
		<-ctx.Done()
		logger.Info("msg", "recevied interrupt, shutting down server...")

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("msg", "server error'd during shutdown", "err", err)
			os.Exit(1)
		}
	}()

	if err := server.Serve(); err != nil {
		logger.Error("msg", "server stopped", "err", err)
	}
}

// doer is a simple http client that gets passed to the generated admin client
// and injects the service token into the header before any requests are made
type doer struct {
	c     *http.Client
	token string
}

func (d doer) Do(r *http.Request) (*http.Response, error) {
	r.Header.Add("api-key", fmt.Sprintf("Bearer %s", d.token))
	return d.c.Do(r)
}
