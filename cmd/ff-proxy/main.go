package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"

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
	offline          bool
	host             string
	port             int
	accountIdentifer string
	orgIdentifier    string
	adminService     string
	serviceToken     string
)

func init() {
	flag.BoolVar(&debug, "debug", false, "enables debug logging")
	flag.BoolVar(&offline, "offline", false, "enables side loading of data from config dir")
	flag.StringVar(&host, "host", "localhost", "host of the proxy service")
	flag.IntVar(&port, "port", 7000, "port that the proxy service is exposed on")
	flag.StringVar(&accountIdentifer, "account-identifier", "px7xd_BFRCi-pfWPYXVjvw", "account identifier to load remote config for")
	flag.StringVar(&orgIdentifier, "org-identifier", "default", "org identifier to load remote config for")
	flag.StringVar(&adminService, "admin-service", "https://qa.harness.io/gateway/cf", "the url of the admin service")
	flag.StringVar(&serviceToken, "service-token", "", "token to use with the ff service")
	flag.Parse()
}

func main() {
	// Setup logger
	logger := log.NewLogger(os.Stderr, debug)
	logger.Info("msg", "service config", "debug", debug, "offline", offline, "host", host, "port", port, "admin-service", adminService, "account-identifier", accountIdentifer, "org-identifier", orgIdentifier)

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
		// Just print for now, we'll pass this to the service later
		fmt.Println("online authConfig", authConfig)
	}

	// Create cache and repos
	memCache := cache.NewMemCache()
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

	featureEvaluator := proxyservice.NewFeatureEvaluator()

	// Setup service and middleware
	var service transport.ProxyService
	service = proxyservice.NewProxyService(fcr, tr, sr, featureEvaluator, logger)
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
