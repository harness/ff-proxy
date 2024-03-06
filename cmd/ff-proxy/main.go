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

	// RedisStreams
	metricsStreamMaxLen          int64
	metricsStreamReadConcurrency int
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

	// RedisStreams
	metricsStreamMaxLenEnv          = "METRICS_STREAM_MAX_LEN"
	metricsStreamReadConcurrencyEnv = "METRIC_STREAM_READ_CONCURRENCY"
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

	// RedisStreams
	metricsStreamMaxLenFlag         = "metrics-stream-max-len"
	metricStreamReadConcurrencyFlag = "metrics-stream-read-concurrency"
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

	// RedisStreams
	flag.Int64Var(&metricsStreamMaxLen, metricsStreamMaxLenFlag, 1000, "Sets the max length of the redis stream that replicas use to send metrics to the Primary")
	flag.IntVar(&metricsStreamReadConcurrency, metricStreamReadConcurrencyFlag, 10, "Controls the number of threads running in the Primary that listen for metrics data being sent by replicas")

	loadFlagsFromEnv(map[string]string{
		bypassAuthEnv:                   bypassAuthFlag,
		logLevelEnv:                     logLevelFlag,
		offlineEnv:                      offlineFlag,
		clientServiceEnv:                clientServiceFlag,
		metricServiceEnv:                metricServiceFlag,
		authSecretEnv:                   authSecretFlag,
		redisAddrEnv:                    redisAddressFlag,
		redisPasswordEnv:                redisPasswordFlag,
		redisDBEnv:                      redisDBFlag,
		metricPostDurationEnv:           metricPostDurationFlag,
		heartbeatIntervalEnv:            heartbeatIntervalFlag,
		pprofEnabledEnv:                 pprofEnabledFlag,
		generateOfflineConfigEnv:        generateOfflineConfigFlag,
		configDirEnv:                    configDirFlag,
		portEnv:                         portFlag,
		tlsEnabledEnv:                   tlsEnabledFlag,
		tlsCertEnv:                      tlsCertFlag,
		tlsKeyEnv:                       tlsKeyFlag,
		gcpProfilerEnabledEnv:           gcpProfilerEnabledFlag,
		proxyKeyEnv:                     proxyKeyFlag,
		readReplicaEnv:                  readReplicaFlag,
		metricsStreamMaxLenEnv:          metricsStreamMaxLenFlag,
		metricsStreamReadConcurrencyEnv: metricStreamReadConcurrencyFlag,
	})

	flag.Parse()
}

//nolint:gocognit,cyclop,maintidx,gocyclo
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
		kind := "primary"
		if readReplica {
			kind = "replica"
		}
		serviceName := fmt.Sprintf("ff-proxy-v2-%s", kind)

		if e := os.Getenv("ENV"); e != "" {
			serviceName = fmt.Sprintf("%s.%s", serviceName, e)
		}

		err := profiler.Start(profiler.Config{Service: serviceName, ServiceVersion: build.Version})
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
	var hashCache *cache.HashCache

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

		mcMetrics := cache.NewMemoizeMetrics("proxy", promReg)
		mcCache := cache.NewMemoizeCache(redisClient, 1*time.Minute, 2*time.Minute, mcMetrics)
		sdkCache = cache.NewMetricsCache("redis", promReg, mcCache)
		hashCache = cache.NewHashCache(cache.NewKeyValCache(redisClient), mcMetrics, 30*time.Minute, 10*time.Minute)

		err = sdkCache.HealthCheck(ctx)
		if err != nil {
			logger.Error("failed to connect to redis", "err", err)
			os.Exit(1)
		}

	} else {
		logger.Info("initialising default memcache")
		sdkCache = cache.NewMetricsCache("in_mem", promReg, cache.NewMemCache())
	}

	clientSvc, err := clientservice.NewClient(logger, clientService, promReg)
	if err != nil {
		logger.Error("failed to create client for the feature flags client service", "err", err)
		os.Exit(1)
	}

	var (
		messageHandler domain.MessageHandler

		gpc = gripcontrol.NewGripPubControl([]map[string]interface{}{
			{
				"control_uri": "http://localhost:5561",
			},
		})
		saasStreamHealth = stream.NewHealth("ffproxy_saas_stream_health", sdkCache, logger)
		connectedStreams = domain.NewSafeMap()

		getConnectedStreams = func() map[string]interface{} {
			return connectedStreams.Get()
		}

		pushpinStream domain.Stream = stream.NewPushpin(gpc)
		redisStream   domain.Stream = stream.NewRedisStream(redisClient)
	)

	// Kick of routine that makes sure the cachedStreamStatus is up to date with the inMemoryStreamStatus
	if !readReplica {
		go saasStreamHealth.VerifyStreamStatus(ctx, 60*time.Second)
	}

	// Get the underlying type from the pushpinStream which is currently the
	// Stream interface. We can now pass the underlying Pushpin type that has
	// the Close method to our OnDisconnect handlers.
	pushpin, ok := pushpinStream.(stream.Pushpin)
	if !ok {
		logger.Error("failed to get underlying type from pushpinStream")
		os.Exit(1)
	}

	const (
		sseStreamTopic = "proxy:sse_events"
	)

	// Configure prometheus labels depending on if we're running as a replica or primary
	if readReplica {
		redisStream = stream.NewPrometheusStream("ff_proxy_replica_sse_consumer", redisStream, promReg)
		pushpinStream = stream.NewPrometheusStream("ff_proxy_replica_to_sdk_sse_producer", pushpinStream, promReg)
	} else {
		redisStream = stream.NewPrometheusStream("ff_proxy_primary_to_replica_sse_producer", redisStream, promReg)
		pushpinStream = stream.NewPrometheusStream("ff_proxy_primary_to_sdk_sse_producer", pushpinStream, promReg)
	}

	readReplicaSSEStream := stream.NewStream(
		logger,
		sseStreamTopic,
		redisStream,
		stream.NewForwarder(logger, pushpinStream, domain.NoOpMessageHandler{}),
		stream.WithOnDisconnect(stream.ReadReplicaSSEStreamOnDisconnect(logger, pushpin, getConnectedStreams)),
		stream.WithBackoff(backoff.NewConstantBackOff(1*time.Minute)),
	)

	primaryToReplicaControlStream := stream.NewStream(
		logger,
		"proxy:primary_to_replica_control_events",
		redisStream,
		domain.NewReadReplicaMessageHandler(),
		stream.WithOnDisconnect(stream.ReadReplicaSSEStreamOnDisconnect(logger, pushpin, getConnectedStreams)),
		stream.WithBackoff(backoff.NewConstantBackOff(1*time.Minute)),
	)

	// Create repos
	targetRepo := repository.NewTargetRepo(sdkCache, logger)
	flagRepo := repository.NewFeatureFlagRepo(hashCache)
	segmentRepo := repository.NewSegmentRepo(hashCache)
	authRepo := repository.NewAuthRepo(sdkCache)
	inventoryRepo := repository.NewInventoryRepo(sdkCache, logger)

	// Create config that we'll use to populate our repos
	conf, err := config.NewConfig(offline, configDir, proxyKey, clientSvc, readReplicaSSEStream)
	if err != nil {
		logger.Error("failed to load config", "err", err)

	}

	reloadConfig := func() error {
		return conf.FetchAndPopulate(ctx, inventoryRepo, authRepo, flagRepo, segmentRepo)
	}

	// If we're running as a Primary we'll need to fetch the config and populate the cache
	var configStatus domain.ConfigStatus
	if !readReplica {
		if err := conf.FetchAndPopulate(ctx, inventoryRepo, authRepo, flagRepo, segmentRepo); err != nil {
			logger.Error("failed to populate repos with config", "err", err)
			configStatus = domain.NewConfigStatus(domain.ConfigStateFailedToSync)
		} else {
			configStatus = domain.NewConfigStatus(domain.ConfigStateSynced)
		}
	}

	// If we're running as a read replica then we want to subscribe to two streams
	//
	// 1. The Redis Stream that the primary forwards SSE events on to
	//   - The replica subscribes to this stream and forwards these events on to SDKs
	//
	// 2. The Redis stream that the primary sends control messages on e.g. stream disconnects
	//   - The replica subscribes to this stream and when it gets a stream disconnect message
	//     it closes any open streams with SDKs to force them to poll for changes
	if readReplica {
		configStatus = domain.NewConfigStatus(domain.ConfigStateReadReplica)
		primaryToReplicaControlStream.Subscribe(ctx)
		readReplicaSSEStream.Subscribe(ctx)
	} else {

		// If we're running as a Primary Proxy then we do the following
		//
		// 1. Subscribe to the Saas SSE stream
		// 2. Refresh the cache when we receive an SSE event
		// 3. Forward events we receive on the Saas SSE Stream to read replica Proxy's
		// 4. Forward events from the Saas SSE stream on to connected SDKs
		cacheRefresher := cache.NewRefresher(logger, conf, clientSvc, inventoryRepo, authRepo, flagRepo, segmentRepo)
		redisForwarder := stream.NewForwarder(logger, redisStream, cacheRefresher, stream.WithStreamName(sseStreamTopic))
		messageHandler = stream.NewForwarder(logger, pushpinStream, redisForwarder)

		streamURL := fmt.Sprintf("%s/stream?cluster=%s", clientService, conf.ClusterIdentifier())
		sseClient := stream.NewSSEClient(logger, streamURL, proxyKey, conf.Token())

		saasStream := stream.NewStream(
			logger,
			"*",
			stream.NewPrometheusStream("ff_proxy_saas_to_primary_sse_consumer", sseClient, promReg),
			messageHandler,
			stream.WithOnConnect(stream.SaasStreamOnConnect(logger, saasStreamHealth, reloadConfig)),
			stream.WithOnDisconnect(
				stream.SaasStreamOnDisconnect(
					logger,
					saasStreamHealth,
					pushpin,
					primaryToReplicaControlStream, // When we disconnect we send a message on this stream to the replica to let it know the saas stream has disconnected
					getConnectedStreams,
					reloadConfig,
				),
			),
		)
		saasStream.Subscribe(ctx)
	}

	metricsEnabled := metricPostDuration != 0 && !offline
	metricStore := newMetricStore(ctx, logger, readReplica, redisClient, promReg, metricsStreamMaxLen, metricPostDuration)

	ms, err := metricsservice.NewClient(logger, metricService, conf.Token, promReg)
	if err != nil {
		logger.Error("failed to create client for the feature flags metric service", "err", err)
		os.Exit(1)
	}

	// If we're running as the primary start the worker that consumes metrics
	// sent by read replicas and sends them on to Saas. Only bother to start
	// worker if sending metrics is actually enabled.
	if !readReplica && metricsEnabled {
		metricsStreamConsumer := stream.NewPrometheusStream("ff_proxy_primary_metrics_stream_consumer", stream.NewRedisStream(redisClient), promReg)
		store, _ := metricStore.(metricsservice.Queue)
		worker := metricsservice.NewWorker(logger, store, ms, metricsStreamConsumer, metricsStreamReadConcurrency, conf.ClusterIdentifier())
		worker.Start(ctx)
	}

	apiKeyHasher := hash.NewSha256()
	tokenSource := token.NewSource(logger, authRepo, apiKeyHasher, []byte(authSecret))
	proxyHealth := health.NewProxyHealth(logger, configStatus, saasStreamHealth.StreamStatus, cacheHealthCheck)

	// Setup service and middleware
	service := proxyservice.NewService(proxyservice.Config{
		Logger:        log.NewContextualLogger(logger, log.ExtractRequestValuesFromContext),
		FeatureRepo:   flagRepo,
		TargetRepo:    targetRepo,
		SegmentRepo:   segmentRepo,
		AuthRepo:      authRepo,
		AuthFn:        tokenSource.GenerateToken,
		ClientService: clientSvc,
		MetricStore:   metricStore,
		Offline:       offline,
		Hasher:        apiKeyHasher,
		Health:        proxyHealth.Health,
		HealthySaasStream: func() bool {
			streamStatus, err := saasStreamHealth.StreamStatus(ctx)
			if err != nil {
				logger.Error("failed to check status of saas -> proxy stream health", "err", err)
				return false
			}
			return streamStatus.State == domain.StreamStateConnected
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
		middleware.NewEchoAuthMiddleware(authRepo, []byte(authSecret), bypassAuth),
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

// newMetricStore creates a MetricStore. If we are running as a read replica it returns a MetricStore that pushes
// metrics to a redis stream. If we are running as a primary it returns a MetricStore that pushed metrics to an
// in memory queue.
func newMetricStore(ctx context.Context, logger log.Logger, readReplica bool, redisClient redis.UniversalClient, promReg *prometheus.Registry, maxLen int64, metricPostDuration int) proxyservice.MetricStore {
	if readReplica {
		return metricsservice.NewStream(
			stream.NewPrometheusStream(
				"ff_proxy_replica_metrics_stream_producer",
				stream.NewRedisStream(
					redisClient,
					stream.WithMaxLen(maxLen),
				),
				promReg,
			),
		)
	}

	return metricsservice.NewQueue(ctx, logger, time.Duration(metricPostDuration)*time.Second)
}
