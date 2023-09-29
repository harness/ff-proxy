package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/harness/ff-proxy/v2/build"
	"github.com/harness/ff-proxy/v2/export"
	"github.com/harness/ff-proxy/v2/health"
	"github.com/harness/ff-proxy/v2/token"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	_ "net/http/pprof" //#nosec

	"cloud.google.com/go/profiler"

	"github.com/go-redis/redis/v8"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/config"
	"github.com/harness/ff-proxy/v2/hash"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/harness/ff-proxy/v2/middleware"
	proxyservice "github.com/harness/ff-proxy/v2/proxy-service"
	"github.com/harness/ff-proxy/v2/repository"
	"github.com/harness/ff-proxy/v2/services"
	"github.com/harness/ff-proxy/v2/transport"
)

var sdkCache cache.Cache

var (
	// Service Config
	proxyKey              string
	clientService         string
	metricService         string
	authSecret            string
	metricPostDuration    int
	heartbeatInterval     int
	generateOfflineConfig bool

	// Cache Config
	offline       bool
	configDir     string
	redisAddress  string
	redisPassword string
	redisDB       int

	// Server Config
	port       int
	tlsEnabled bool
	tlsCert    string
	tlsKey     string

	// Dev/Debugging
	bypassAuth         bool
	logLevel           string
	gcpProfilerEnabled bool
	pprofEnabled       bool
)

// Environment Variables
const (
	// Service Config
	proxyKeyEnv              = "PROXY_KEY"
	clientServiceEnv         = "CLIENT_SERVICE"
	metricServiceEnv         = "METRIC_SERVICE"
	authSecretEnv            = "AUTH_SECRET"
	metricPostDurationEnv    = "METRIC_POST_DURATION"
	heartbeatIntervalEnv     = "HEARTBEAT_INTERVAL"
	generateOfflineConfigEnv = "GENERATE_OFFLINE_CONFIG"

	// Cache Config
	offlineEnv       = "OFFLINE"
	configDirEnv     = "CONFIG_DIR"
	redisAddrEnv     = "REDIS_ADDRESS"
	redisPasswordEnv = "REDIS_PASSWORD"
	redisDBEnv       = "REDIS_DB"

	// Server Config
	portEnv       = "PORT"
	tlsEnabledEnv = "TLS_ENABLED"
	tlsCertEnv    = "TLS_CERT"
	tlsKeyEnv     = "TLS_KEY"

	// Dev/Debugging
	bypassAuthEnv         = "BYPASS_AUTH"
	logLevelEnv           = "LOG_LEVEL"
	gcpProfilerEnabledEnv = "GCP_PROFILER_ENABLED"
	pprofEnabledEnv       = "PPROF"
)

// Flags
const (
	// Service Config
	proxyKeyFlag              = "proxy-key"
	clientServiceFlag         = "client-service"
	metricServiceFlag         = "metric-service"
	authSecretFlag            = "auth-secret"
	metricPostDurationFlag    = "metric-post-duration"
	heartbeatIntervalFlag     = "heartbeat-interval"
	generateOfflineConfigFlag = "generate-offline-config"

	// Cache Config
	configDirFlag     = "config-dir"
	offlineFlag       = "offline"
	redisAddressFlag  = "redis-address"
	redisPasswordFlag = "redis-password"
	redisDBFlag       = "redis-db"

	// Server Config
	portFlag       = "port"
	tlsEnabledFlag = "tls-enabled"
	tlsCertFlag    = "tls-cert"
	tlsKeyFlag     = "tls-key"

	// Dev/Debugging
	bypassAuthFlag         = "bypass-auth"
	logLevelFlag           = "log-level"
	pprofEnabledFlag       = "pprof"
	gcpProfilerEnabledFlag = "gcp-profiler-enabled"
)

func init() {
	// Service Config
	flag.StringVar(&proxyKey, proxyKeyFlag, "", "The ProxyKey you want to configure your Proxy to use")
	flag.StringVar(&clientService, clientServiceFlag, "https://config.ff.harness.io/api/1.0", "the url of the ff client service")
	flag.StringVar(&metricService, metricServiceFlag, "https://events.ff.harness.io/api/1.0", "the url of the ff metric service")
	flag.StringVar(&authSecret, authSecretFlag, "secret", "the secret used for signing auth tokens")
	flag.IntVar(&metricPostDuration, metricPostDurationFlag, 60, "How often in seconds the proxy posts metrics to Harness. Set to 0 to disable.")
	flag.IntVar(&heartbeatInterval, heartbeatIntervalFlag, 60, "How often in seconds the proxy polls pings it's health function. Set to 0 to disable.")
	flag.BoolVar(&generateOfflineConfig, generateOfflineConfigFlag, false, "if true the proxy will produce offline config in the /config directory then terminate")

	// Cache Config
	flag.BoolVar(&offline, offlineFlag, false, "enables side loading of data from config dir")
	flag.StringVar(&configDir, configDirFlag, "/config", "specify a custom path to search for the offline config directory. Defaults to /config")
	flag.StringVar(&redisAddress, redisAddressFlag, "", "Redis host:port address")
	flag.StringVar(&redisPassword, redisPasswordFlag, "", "Optional. Redis password")
	flag.IntVar(&redisDB, redisDBFlag, 0, "Database to be selected after connecting to the server.")

	// Server Config
	flag.IntVar(&port, portFlag, 8000, "port the relay proxy service is exposed on, default's to 8000")
	flag.BoolVar(&tlsEnabled, tlsEnabledFlag, false, "if true the proxy will use the tlsCert and tlsKey to run with https enabled")
	flag.StringVar(&tlsCert, tlsCertFlag, "", "Path to tls cert file. Required if tls enabled is true.")
	flag.StringVar(&tlsKey, tlsKeyFlag, "", "Path to tls key file. Required if tls enabled is true.")

	// Dev/Debugging
	flag.BoolVar(&bypassAuth, bypassAuthFlag, false, "bypasses authentication")
	flag.StringVar(&logLevel, logLevelFlag, "INFO", "sets the logging level, valid options are INFO, DEBUG & ERROR")
	flag.BoolVar(&pprofEnabled, pprofEnabledFlag, false, "enables pprof on port 6060")
	flag.BoolVar(&gcpProfilerEnabled, gcpProfilerEnabledFlag, false, "Enables gcp cloud profiler")

	loadFlagsFromEnv(map[string]string{
		bypassAuthEnv:            bypassAuthFlag,
		logLevelEnv:              logLevelFlag,
		offlineEnv:               offlineFlag,
		clientServiceEnv:         clientServiceFlag,
		metricServiceEnv:         metricServiceFlag,
		authSecretEnv:            authSecretFlag,
		redisAddrEnv:             redisAddressFlag,
		redisPasswordEnv:         redisPasswordFlag,
		redisDBEnv:               redisDBFlag,
		metricPostDurationEnv:    metricPostDurationFlag,
		heartbeatIntervalEnv:     heartbeatIntervalFlag,
		pprofEnabledEnv:          pprofEnabledFlag,
		generateOfflineConfigEnv: generateOfflineConfigFlag,
		configDirEnv:             configDirFlag,
		portEnv:                  portFlag,
		tlsEnabledEnv:            tlsEnabledFlag,
		tlsCertEnv:               tlsCertFlag,
		tlsKeyEnv:                tlsKeyFlag,
		gcpProfilerEnabledEnv:    gcpProfilerEnabledFlag,
		proxyKeyEnv:              proxyKeyFlag,
	})

	flag.Parse()
}

func main() {

	// Setup logger
	logger, err := log.NewStructuredLogger(logLevel)
	if err != nil {
		fmt.Println("we have no logger")
		os.Exit(1)
	}

	if pprofEnabled {
		go func() {
			// #nosec
			if err := http.ListenAndServe(":6060", nil); err != nil {
				stdlog.Printf("failed to start pprof server: %s \n", err)
			}
		}()
	}

	if gcpProfilerEnabled {
		err := profiler.Start(profiler.Config{Service: "ff-proxy", ServiceVersion: build.Version})
		if err != nil {
			logger.Info("unable to start gcp profiler", "err", err)
		}
	}

	// we currently don't require any config to run in offline mode
	requiredFlags := map[string]interface{}{}
	if !offline {
		requiredFlags = map[string]interface{}{
			proxyKeyEnv: proxyKey,
		}
	}
	validateFlags(requiredFlags)

	// Setup cancelation
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigc
		cancel()
	}()

	promReg := prometheus.NewRegistry()
	promReg.MustRegister(collectors.NewGoCollector())

	logger.Info("service config", "pprof", pprofEnabled, "log-level", logLevel, "bypass-auth", bypassAuth, "offline", offline, "port", port, "redis-addr", redisAddress, "redis-db", redisDB, "heartbeat-interval", fmt.Sprintf("%ds", heartbeatInterval), "config-dir", configDir, "tls-enabled", tlsEnabled, "tls-cert", tlsCert, "tls-key", tlsKey)

	// Create cache
	// if we're just generating the offline config we should only use in memory mode for now
	// when we move to a pattern of allowing periodic config dumps to disk we can remove this requirement
	if redisAddress != "" && !generateOfflineConfig {
		// if address does not start with redis:// or rediss:// then default to redis://
		// if the connection string starts with rediss:// it means we'll connect with TLS enabled
		redisConnectionString := redisAddress
		if !strings.HasPrefix(redisAddress, "redis://") && !strings.HasPrefix(redisAddress, "rediss://") {
			redisConnectionString = fmt.Sprintf("redis://%s", redisAddress)
		}
		parsed, err := redis.ParseURL(redisConnectionString)
		if err != nil {
			logger.Error("failed to parse redis address url", "connection string", redisConnectionString, "err", err)
			os.Exit(1)
		}
		// TODO - going forward we can open up support for more of these query param connection string options e.g. max_retries etc
		// we would first need to test the impact that these would have if unset vs current defaults
		opts := redis.UniversalOptions{}
		opts.DB = parsed.DB
		opts.Addrs = []string{parsed.Addr}
		opts.Username = parsed.Username
		opts.Password = parsed.Password
		opts.TLSConfig = parsed.TLSConfig
		if redisPassword != "" {
			opts.Password = redisPassword
		}
		client := redis.NewUniversalClient(&opts)
		logger.Info("connecting to redis", "address", redisAddress)
		sdkCache = cache.NewMetricsCache("redis", promReg, cache.NewMemoizeCache(client, 1*time.Minute, 2*time.Minute, cache.NewMemoizeMetrics("proxy", promReg)))
		err = sdkCache.HealthCheck(ctx)
		if err != nil {
			logger.Error("failed to connect to redis", "err", err)
			os.Exit(1)
		}
	} else {
		logger.Info("initialising default memcache")
		sdkCache = cache.NewMetricsCache("in_mem", promReg, cache.NewMemCache())
	}

	clientSvc, err := services.NewClientService(logger, clientService)
	if err != nil {
		logger.Error("failed to create client for the feature flags client service", "err", err)
		os.Exit(1)
	}

	// Create repos
	targetRepo := repository.NewTargetRepo(sdkCache)
	flagRepo := repository.NewFeatureFlagRepo(sdkCache)
	segmentRepo := repository.NewSegmentRepo(sdkCache)
	authRepo := repository.NewAuthRepo(sdkCache)

	// Create config that we'll use to populate our repos
	conf, err := config.NewConfig(offline, configDir, proxyKey, clientSvc)
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	if err := conf.Populate(ctx, authRepo, flagRepo, segmentRepo); err != nil {
		logger.Error("failed to populate repos with config", "err", err)
		os.Exit(1)
	}

	metricsEnabled := metricPostDuration != 0 && !offline
	metricSvc, err := services.NewMetricService(logger, metricService, conf.Token(), metricsEnabled, promReg)
	if err != nil {
		logger.Error("failed to create client for the feature flags metric service", "err", err)
		os.Exit(1)
	}

	if generateOfflineConfig {
		exportService := export.NewService(logger, flagRepo, targetRepo, segmentRepo, authRepo, nil, configDir)
		err = exportService.Persist(ctx)
		if err != nil {
			logger.Error("offline config export failed err: %s", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	apiKeyHasher := hash.NewSha256()
	tokenSource := token.NewTokenSource(logger, authRepo, apiKeyHasher, []byte(authSecret))

	// Setup service and middleware
	service := proxyservice.NewService(proxyservice.Config{
		Logger:        log.NewContextualLogger(logger, log.ExtractRequestValuesFromContext),
		FeatureRepo:   flagRepo,
		TargetRepo:    targetRepo,
		SegmentRepo:   segmentRepo,
		AuthRepo:      authRepo,
		CacheHealthFn: cacheHealthCheck,
		AuthFn:        tokenSource.GenerateToken,
		ClientService: clientSvc,
		MetricService: metricSvc,
		Offline:       offline,
		Hasher:        apiKeyHasher,
	})

	// Configure endpoints and server
	endpoints := transport.NewEndpoints(service)
	server := transport.NewHTTPServer(port, endpoints, logger, tlsEnabled, tlsCert, tlsKey, promReg)
	server.Use(
		middleware.NewEchoRequestIDMiddleware(),
		middleware.NewEchoLoggingMiddleware(logger),
		middleware.NewEchoAuthMiddleware([]byte(authSecret), bypassAuth),
		middleware.NewPrometheusMiddleware(promReg),
	)

	go func() {
		<-ctx.Done()
		logger.Info("received interrupt, shutting down server...")

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("server error'd during shutdown", "err", err)
			os.Exit(1)
		}
	}()

	// TODO: We should move this inside services/metrics_service.go
	//if !offline {
	//	// start metric sending ticker
	//	if metricPostDuration != 0 {
	//		go func() {
	//			logger.Info(fmt.Sprintf("sending metrics every %d seconds", metricPostDuration))
	//			ticker := time.NewTicker(time.Duration(metricPostDuration) * time.Second)
	//			defer ticker.Stop()
	//
	//			for {
	//				select {
	//				case <-ctx.Done():
	//					logger.Info("stopping metrics ticker")
	//					return
	//				case <-ticker.C:
	//					// default to prod cluster
	//					clusterIdentifier := "1"
	//					logger.Debug("sending metrics")
	//					metricService.SendMetrics(ctx, clusterIdentifier)
	//				}
	//			}
	//		}()
	//	} else {
	//		logger.Info("sending metrics disabled")
	//	}
	//}

	if heartbeatInterval != 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(heartbeatInterval) * time.Second)
			logger.Info(fmt.Sprintf("polling heartbeat every %d seconds", heartbeatInterval))
			protocol := "http"
			if tlsEnabled {
				protocol = "https"
			}

			health.Heartbeat(ctx, ticker.C, fmt.Sprintf("%s://localhost:%d", protocol, port), logger)
		}()
	}

	if err := server.Serve(); err != nil {
		logger.Error("server stopped", "err", err)
	}
}

// checks the health of the connected cache instance
func cacheHealthCheck(ctx context.Context) error {
	return sdkCache.HealthCheck(ctx)
}

func loadFlagsFromEnv(envToFlag map[string]string) {
	for k, v := range envToFlag {
		val := os.Getenv(k)
		if val == "" {
			continue
		}
		os.Args = append(os.Args, fmt.Sprintf("--%s=%s", v, val))
	}
}

func validateFlags(flags map[string]interface{}) {
	unset := []string{}
	for k, v := range flags {
		switch v.(type) {
		case string:
			if v == "" {
				unset = append(unset, k)
			}
		case int:
			if v == 0 {
				unset = append(unset, k)
			}
		case []string:
			if len(v.([]string)) == 0 {
				unset = append(unset, k)
			}
		}
	}

	if len(unset) > 0 {
		stdlog.Fatalf("The following configuration values are required: %v ", unset)
	}
}
