package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"

	sdkCache "github.com/harness/ff-golang-server-sdk/cache"
	harness "github.com/harness/ff-golang-server-sdk/client"
	ffproxy "github.com/harness/ff-proxy"
	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/config"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/middleware"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
	"github.com/harness/ff-proxy/repository"
	"github.com/harness/ff-proxy/services"
	"github.com/harness/ff-proxy/transport"
	"github.com/sirupsen/logrus"
	"github.com/wings-software/ff-server/pkg/hash"
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
	sdkBaseURL       string
	sdkEventsURL     string
	apiKeys          keys
)

// keys implements the flag.Value interface and allows us to pass in multiple api keys in the program arguments
// e.g. -apiKey key1 -apiKey key2 -apiKey key3
type keys []string

func (i *keys) String() string {
	return strings.Join(*i, ",")
}

func (i *keys) Set(value string) error {
	*i = append(*i, strings.TrimSpace(value))
	return nil
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
	flag.StringVar(&sdkBaseURL, "sdkBaseUrl", "https://config.feature-flags.qa.harness.io/api/1.0", "url for the sdk to connect to")
	flag.StringVar(&sdkEventsURL, "sdkEventsUrl", "https://event.feature-flags.qa.harness.io/api/1.0", "url for the sdk to send metrics to")
	flag.Var(&apiKeys, "apiKey", "API keys to connect with ff-server for each environment")

	flag.Parse()
}

func initFF(ctx context.Context, cache sdkCache.Cache, baseURL, eventURL, sdkKey string) {
	logger := log.NewLogger(os.Stderr, debug)

	client, err := harness.NewCfClient(sdkKey,
		harness.WithURL(baseURL),
		harness.WithEventsURL(eventURL),
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

	<-ctx.Done()
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

	adminClient, err := services.NewAdminClient(logger, adminService, serviceToken)
	if err != nil {
		logger.Error("msg", "failed to create admin client", "err", err)
		os.Exit(1)
	}

	var (
		featureConfig map[domain.FeatureFlagKey][]domain.FeatureFlag
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
		featureConfig = config.FeatureFlag()
		targetConfig = config.Targets()
		segmentConfig = config.Segments()
		authConfig = config.AuthConfig()

		logger.Info("msg", "retrieved offline config")
	} else {
		logger.Info("msg", "retrieving config from ff-server...")
		config := config.NewRemoteConfig(
			ctx,
			accountIdentifer,
			orgIdentifier,
			apiKeys,
			hash.NewSha256(),
			adminClient,
			config.WithLogger(logger),
			config.WithConcurrency(20),
		)

		authConfig = config.AuthConfig()
		targetConfig = config.TargetConfig()
		logger.Info("msg", "successfully retrieved config from ff-server")
	}

	// Create cache
	memCache := cache.NewMemCache()
	apiKeyHasher := hash.NewSha256()

	// start an sdk instance for each api key
	for _, apiKey := range apiKeys {
		apiKeyHash := apiKeyHasher.Hash(apiKey)

		// find corresponding environmentID for apiKey
		envID, ok := authConfig[domain.AuthAPIKey(apiKeyHash)]
		if !ok {
			logger.Error("API key not found, skipping: %v", apiKey)
			continue
		}

		cacheWrapper := cache.NewWrapper(&memCache, envID, logrus.New())
		go initFF(ctx, cacheWrapper, sdkBaseURL, sdkEventsURL, apiKey)
	}

	// Create repos
	tr, err := repository.NewTargetRepo(memCache, targetConfig)
	if err != nil {
		logger.Error("msg", "failed to create target repo", "err", err)
		os.Exit(1)
	}

	fcr, err := repository.NewFeatureFlagRepo(memCache, featureConfig)
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
	tokenSource := ffproxy.NewTokenSource(logger, authRepo, apiKeyHasher, []byte(authSecret))

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
