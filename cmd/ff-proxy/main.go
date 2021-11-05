package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"time"

	ffproxy "github.com/harness/ff-proxy"
	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
	"github.com/harness/ff-proxy/repository"
	"github.com/harness/ff-proxy/transport"
)

var (
	debug   bool
	offline bool
	host    string
	port    int
)

func init() {
	flag.BoolVar(&debug, "debug", false, "enables debug logging")
	flag.BoolVar(&offline, "offline", false, "enables side loading of data from config dir")
	flag.StringVar(&host, "host", "localhost", "host of the proxy service")
	flag.IntVar(&port, "port", 7000, "port that the proxy service is exposed on")
	flag.Parse()
}

func main() {
	logger := log.NewLogger(os.Stderr, debug)
	logger.Info("msg", "service config", "debug", debug, "offline", offline, "host", host, "port", port)

	var (
		featureConfig map[domain.FeatureConfigKey][]domain.FeatureConfig
		targetConfig  map[domain.TargetKey][]domain.Target
		segmentConfig map[domain.SegmentKey][]domain.Segment
	)

	if offline {
		config, err := ffproxy.NewFeatureFlagConfig(ffproxy.DefaultConfig, ffproxy.DefaultConfigDir)
		if err != nil {
			logger.Error("msg", "failed to load config", "err", err)
			os.Exit(1)
		}
		featureConfig = config.FeatureConfig()
		targetConfig = config.Targets()
		segmentConfig = config.Segments()
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
	service := transport.NewLoggingService(logger, proxyservice.NewProxyService(fcr, tr, sr, featureEvaluator, logger))
	endpoints := transport.NewEndpoints(service)
	server := transport.NewHTTPServer(host, port, endpoints, logger)

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Interrupt)
		<-sigc
		logger.Info("msg", "recevied interrupt, shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("msg", "server error'd during shutdown", "err", err)
			os.Exit(1)
		}
	}()

	if err := server.Serve(); err != nil {
		logger.Error("msg", "server stopped", "err", err)
	}
}
