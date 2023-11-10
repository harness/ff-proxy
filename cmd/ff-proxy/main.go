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

	"gopkg.in/cenkalti/backoff.v1"

	"github.com/harness/ff-proxy/v2/domain"

	"github.com/fanout/go-gripcontrol"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/harness/ff-proxy/v2/build"
	clientservice "github.com/harness/ff-proxy/v2/clients/client_service"
	metricsservice "github.com/harness/ff-proxy/v2/clients/metrics_service"
	"github.com/harness/ff-proxy/v2/export"
	"github.com/harness/ff-proxy/v2/health"
	"github.com/harness/ff-proxy/v2/stream"
	"github.com/harness/ff-proxy/v2/token"

	"cloud.google.com/go/profiler"

	"github.com/go-redis/redis/v8"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/config"
	"github.com/harness/ff-proxy/v2/hash"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/harness/ff-proxy/v2/middleware"
	proxyservice "github.com/harness/ff-proxy/v2/proxy-service"
	"github.com/harness/ff-proxy/v2/repository"
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
	readReplica           bool

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
	readReplicaEnv           = "READ_REPLICA"

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
	bypassAuthEnv         = "BYPASS_AUTH" //nolint:gosec
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
	readReplicaFlag           = "readReplica"

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

// nolint:gochecknoinits
func init() {
	// Service Config
	flag.StringVar(&proxyKey, proxyKeyFlag, "", "The ProxyKey you want to configure your Proxy to use")
	flag.StringVar(&clientService, clientServiceFlag, "https://config.ff.harness.io/api/1.0", "the url of the ff client service")
	flag.StringVar(&metricService, metricServiceFlag, "https://events.ff.harness.io/api/1.0", "the url of the ff metric service")
	flag.StringVar(&authSecret, authSecretFlag, "secret", "the secret used for signing auth tokens")
	flag.IntVar(&metricPostDuration, metricPostDurationFlag, 60, "How often in seconds the proxy posts metrics to Harness. Set to 0 to disable.")
	flag.IntVar(&heartbeatInterval, heartbeatIntervalFlag, 60, "How often in seconds the proxy polls pings it's health function. Set to 0 to disable.")
	flag.BoolVar(&generateOfflineConfig, generateOfflineConfigFlag, false, "if true the proxy will produce offline config in the /config directory then terminate")
	flag.BoolVar(&readReplica, readReplicaFlag, false, "if true the Proxy will operate as a read replica that only reads from the cache and doesn't fetch new data from Harness SaaS")

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
		readReplicaEnv:           readReplicaFlag,
	})

	flag.Parse()
}

//nolint:gocognit,cyclop,maintidx
func main() {

	// Setup logger
	logger, err := log.NewStructuredLogger(logLevel)
	if err != nil {
		fmt.Println("we have no logger")
		os.Exit(1)
	}

	if pprofEnabled {
		go func() {
			//nolint:gosec
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
	if !offline && !readReplica {
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

	logger.Info("service config", "pprof", pprofEnabled, "log-level", logLevel, "bypass-auth", bypassAuth, "offline", offline, "port", port, "redis-addr", redisAddress, "redis-db", redisDB, "heartbeat-interval", fmt.Sprintf("%ds", heartbeatInterval), "config-dir", configDir, "tls-enabled", tlsEnabled, "tls-cert", tlsCert, "tls-key", tlsKey, "read-replica", readReplica, "client-service", clientService, "metrics-service", metricService)

	// Create cache
	// if we're just generating the offline config we should only use in memory mode for now
	// when we move to a pattern of allowing periodic config dumps to disk we can remove this requirement

	var redisClient redis.UniversalClient

	if redisAddress != "" && !generateOfflineConfig { //nolint:nestif
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
		redisClient = redis.NewUniversalClient(&opts)
		logger.Info("connecting to redis", "address", redisAddress)
		sdkCache = cache.NewMetricsCache("redis", promReg, cache.NewMemoizeCache(redisClient, 1*time.Minute, 2*time.Minute, cache.NewMemoizeMetrics("proxy", promReg)))
		err = sdkCache.HealthCheck(ctx)
		if err != nil {
			logger.Error("failed to connect to redis", "err", err)
			os.Exit(1)
		}
	} else {
		logger.Info("initialising default memcache")
		sdkCache = cache.NewMetricsCache("in_mem", promReg, cache.NewMemCache())
	}

	clientSvc, err := clientservice.NewClient(logger, clientService)
	if err != nil {
		logger.Error("failed to create client for the feature flags client service", "err", err)
		os.Exit(1)
	}

	// Create repos
	targetRepo := repository.NewTargetRepo(sdkCache, logger)
	flagRepo := repository.NewFeatureFlagRepo(sdkCache)
	segmentRepo := repository.NewSegmentRepo(sdkCache)
	authRepo := repository.NewAuthRepo(sdkCache)
	inventoryRepo := repository.NewInventoryRepo(sdkCache)

	// Create config that we'll use to populate our repos
	conf, err := config.NewConfig(offline, configDir, proxyKey, clientSvc)
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	// Read replicas don't need to care about populating the repos with config
	if !readReplica {
		if err := conf.FetchAndPopulate(ctx, inventoryRepo, authRepo, flagRepo, segmentRepo); err != nil {
			logger.Error("failed to populate repos with config", "err", err)
			os.Exit(1)
		}
	}

	// If the generateOfflineConfig flag is provided then we can just export the config and exit
	if generateOfflineConfig {
		exportService := export.NewService(logger, flagRepo, targetRepo, segmentRepo, authRepo, nil, configDir)
		err = exportService.Persist(ctx)
		if err != nil {
			logger.Error("offline config export failed err: %s", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	var (
		messageHandler domain.MessageHandler

		gpc = gripcontrol.NewGripPubControl([]map[string]interface{}{
			{
				"control_uri": "http://localhost:5561",
			},
		})
		pushpinStream    = stream.NewPushpin(gpc)
		saasStreamHealth = stream.NewHealth("ffproxy_saas_stream_health", sdkCache)
		connectedStreams = domain.NewSafeMap()

		getConnectedStreams = func() map[string]interface{} {
			return connectedStreams.Get()
		}

		reloadConfig = func() error {
			return conf.FetchAndPopulate(ctx, inventoryRepo, authRepo, flagRepo, segmentRepo)
		}

		redisStream = stream.NewRedisStream(redisClient)
	)

	const (
		sseStreamTopic = "proxy:sse_events"
	)

	readReplicaSSEStream := stream.NewStream(
		logger,
		sseStreamTopic,
		redisStream,
		stream.NewForwarder(logger, pushpinStream, domain.NewReadReplicaMessageHandler()),
		stream.WithOnDisconnect(stream.ReadReplicaSSEStreamOnDisconnect(logger, pushpinStream, getConnectedStreams)),
		stream.WithBackoff(backoff.NewConstantBackOff(1*time.Minute)),
	)

	if readReplica {
		// If we're running  in read replica mode we need to subscribe to the redis stream that
		// the Writer will be forwarding events to and forward these on to pushpin so any connected
		// SDKs get the events
		readReplicaSSEStream.Subscribe(ctx)
	} else {
		// If we're running as a 'write' replica Proxy then we need our message handler to forward events
		// on to redis, pushpin and refresh the cache. These types all implement the MessageHandler interface,
		// meaning they're essentially middlewares that we can layer one after the other.
		//
		// Layering them in this order means that the first thing we'll do when we receive an SSE message
		// is attempt to refresh the cache, then if that's successful we'll forward the event on to Redis and Pushpin
		cacheRefresher := cache.NewRefresher(logger, conf, clientSvc, inventoryRepo, authRepo, flagRepo, segmentRepo)
		redisForwarder := stream.NewForwarder(logger, redisStream, cacheRefresher, stream.WithStreamName(sseStreamTopic))
		messageHandler = stream.NewForwarder(logger, pushpinStream, redisForwarder)
	}

	// If this is the 'Writer' proxy then open up a stream to Harness Saas
	// so the Proxy can be notified of changes
	if !readReplica {
		streamURL := fmt.Sprintf("%s/stream", clientService)
		sseClient := stream.NewSSEClient(logger, streamURL, proxyKey, conf.Token())

		saasStream := stream.NewStream(
			logger,
			"*",
			sseClient,
			messageHandler,
			stream.WithOnConnect(stream.SaasStreamOnConnect(logger, saasStreamHealth)),
			stream.WithOnDisconnect(
				stream.SaasStreamOnDisconnect(
					logger,
					saasStreamHealth,
					pushpinStream,
					readReplicaSSEStream,
					getConnectedStreams,
					reloadConfig,
				)),
		)
		saasStream.Subscribe(ctx)
	}

	metricSvc := createMetricsService(ctx, logger, conf, promReg, readReplica, redisClient)

	apiKeyHasher := hash.NewSha256()
	tokenSource := token.NewSource(logger, authRepo, apiKeyHasher, []byte(authSecret))

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
		HealthySaasStream: func() bool {
			b, err := saasStreamHealth.StreamHealthy(ctx)
			if err != nil {
				logger.Error("failed to check status of saas -> proxy stream health", "err", err)
				return b
			}
			return b
		},
		SDKStreamConnected: func(envID string) {
			connectedStreams.Set(envID, "")
		},
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

	protocol := "http"
	if tlsEnabled {
		protocol = "https"
	}
	health.Heartbeat(ctx, heartbeatInterval, fmt.Sprintf("%s://localhost:%d", protocol, port), logger)

	if err := server.Serve(); err != nil {
		logger.Error("server stopped", "err", err)
	}
}

// createMetricsService is a helper that creates the Client implementation we need depending on the mode the
// Proxy is running in. If the proxy is running in readReplica mode then we want to return a Client implementation
// that pushes metrics onto a redis stream. If we're not running in readReplica mode then we want to return the normal
// client that forwards requests on to Harness SaaS
func createMetricsService(ctx context.Context, logger log.Logger, conf config.Config, promReg *prometheus.Registry, readReplica bool, redisClient redis.UniversalClient) proxyservice.MetricService {
	redisStream := stream.NewRedisStream(redisClient)
	if readReplica {
		return metricsservice.NewStream(stream.NewRedisStream(redisClient))
	}

	metricsEnabled := metricPostDuration != 0 && !offline
	ms, err := metricsservice.NewClient(logger, metricService, conf.Token(), metricsEnabled, promReg, redisStream)
	if err != nil {
		logger.Error("failed to create client for the feature flags metric service", "err", err)
		os.Exit(1)
	}
	// Kick off the job that periodically posts metrics to Harness SaaS
	ms.PostMetrics(ctx, offline, metricPostDuration)
	return ms
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
		switch t := v.(type) {
		case string:
			if t == "" {
				unset = append(unset, k)
			}
		case int:
			if t == 0 {
				unset = append(unset, k)
			}
		case []string:
			if len(t) == 0 {
				unset = append(unset, k)
			}
		}
	}

	if len(unset) > 0 {
		stdlog.Fatalf("The following configuration values are required: %v ", unset)
	}
}
