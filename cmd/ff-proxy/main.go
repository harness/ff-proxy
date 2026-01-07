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

	_ "net/http/pprof" //nolint:gosec

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/cenkalti/backoff.v1"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/redis/go-redis/v9"

	"github.com/harness-community/go-gripcontrol"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/harness/ff-proxy/v2/build"
	clientservice "github.com/harness/ff-proxy/v2/clients/client_service"
	metricsservice "github.com/harness/ff-proxy/v2/clients/metrics_service"
	"github.com/harness/ff-proxy/v2/health"
	"github.com/harness/ff-proxy/v2/stream"
	"github.com/harness/ff-proxy/v2/token"

	"cloud.google.com/go/profiler"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/config"
	"github.com/harness/ff-proxy/v2/hash"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/harness/ff-proxy/v2/middleware"
	proxyservice "github.com/harness/ff-proxy/v2/proxy-service"
	redisclient "github.com/harness/ff-proxy/v2/redis"
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
	legacyAuthSecrets     legacySecrets
	metricPostDuration    int
	heartbeatInterval     int
	generateOfflineConfig bool
	readReplica           bool
	forwardTargets        bool

	// Cache Config
	offline       bool
	configDir     string
	redisAddress  string
	redisPassword string
	redisUsername string
	redisDB       int

	// Redis Config Options
	redisMaxRetries                  int
	redisMinRetryBackoffMilliseconds int
	redisMaxRetryBackoffMilliseconds int

	redisDialTimeoutSeconds  int
	redisReadTimeoutSeconds  int
	redisWriteTimeoutSeconds int

	redisPoolSize           int
	redisPoolSizeLiteral    int
	redisPoolTimeoutSeconds int

	redisMinIdleConns           int
	redisMaxIdleConns           int
	redisMaxActiveConns         int
	redisConnMaxIdleTimeMinutes int
	redisConnMaxLifetimeMinutes int

	// Redis TLS/mTLS Config
	redisTLSEnabled            bool
	redisTLSMode               string
	redisTLSCACertPath         string
	redisTLSClientCertPath     string
	redisTLSClientKeyPath      string
	redisTLSInsecureSkipVerify bool
	redisTLSServerName         string

	// Server Config
	port           int
	tlsEnabled     bool
	tlsCert        string
	tlsKey         string
	prometheusPort int

	// Dev/Debugging
	bypassAuth         bool
	logLevel           string
	gcpProfilerEnabled bool
	pprofEnabled       bool

	// RedisStreams
	metricsStreamMaxLen          int64
	metricsStreamReadConcurrency int

	// Beta features - will be short-lived and then become default behaviour in future releases
	andRules bool
)

// legacySecrets implements the flag.Value interface and allows us to pass a comma separated
// list of legacy secrets e.g. -legacy-secrets mysecret,mysecret2
type legacySecrets []string

func (l *legacySecrets) String() string {
	return strings.Join(*l, ",")
}

func (l *legacySecrets) Set(value string) error {
	ss := strings.Split(value, ",")
	for _, s := range ss {
		*l = append(*l, s)
	}
	return nil
}

// Environment Variables
const (
	// Service Config
	proxyKeyEnv              = "PROXY_KEY"
	clientServiceEnv         = "CLIENT_SERVICE"
	metricServiceEnv         = "METRIC_SERVICE"
	authSecretEnv            = "AUTH_SECRET"
	legacySecretsEnv         = "LEGACY_SECRETS"
	metricPostDurationEnv    = "METRIC_POST_DURATION"
	heartbeatIntervalEnv     = "HEARTBEAT_INTERVAL"
	generateOfflineConfigEnv = "GENERATE_OFFLINE_CONFIG"
	readReplicaEnv           = "READ_REPLICA"
	forwardTargetsEnv        = "FORWARD_TARGETS"

	// Cache Config
	offlineEnv       = "OFFLINE"
	configDirEnv     = "CONFIG_DIR"
	redisAddrEnv     = "REDIS_ADDRESS"
	redisPasswordEnv = "REDIS_PASSWORD"
	redisUsernameEnv = "REDIS_USERNAME"
	redisDBEnv       = "REDIS_DB"

	// Redis Config
	redisMaxRetriesEnv                  = "REDIS_MAX_RETRIES"
	redisMinRetryBackoffMillisecondsEnv = "REDIS_MIN_RETRY_BACKOFF_MILLIS"
	redisMaxRetryBackoffMillisecondsEnv = "REDIS_MAX_RETRY_BACKOFF_MILLIS"

	redisDialTimeoutSecondsEnv  = "REDIS_DIAL_TIMEOUT_SECONDS"
	redisReadTimeoutSecondsEnv  = "REDIS_READ_TIMEOUT_SECONDS"
	redisWriteTimeoutSecondsEnv = "REDIS_WRITE_TIMEOUT_SECONDS"

	redisPoolSizeEnv           = "REDIS_POOL_SIZE"
	redisPoolSizeLiteralEnv    = "REDIS_POOL_SIZE_LITERAL"
	redisPoolTimeoutSecondsEnv = "REDIS_POOL_TIMEOUT_SECONDS"

	redisMinIdleConnsEnv           = "REDIS_MIN_IDLE_CONNS"
	redisMaxIdleConnsEnv           = "REDIS_MAX_IDLE_CONNS"
	redisMaxActiveConnsEnv         = "REDIS_MAX_ACTIVE_CONNS"
	redisConnMaxIdleTimeMinutesEnv = "REDIS_CON_MAX_IDLE_TIME_MINUTES"
	redisConnMaxLifetimeMinutesEnv = "REDIS_CON_MAX_LIFETIME_MINUTES"

	// Redis TLS/mTLS Config
	redisTLSEnabledEnv            = "REDIS_TLS_ENABLED"
	redisTLSModeEnv               = "REDIS_TLS_MODE"
	redisTLSCACertEnv             = "REDIS_TLS_CA_CERT"
	redisTLSClientCertEnv         = "REDIS_TLS_CLIENT_CERT"
	redisTLSClientKeyEnv          = "REDIS_TLS_CLIENT_KEY"
	redisTLSInsecureSkipVerifyEnv = "REDIS_TLS_INSECURE_SKIP_VERIFY"
	redisTLSServerNameEnv         = "REDIS_TLS_SERVER_NAME"

	// Server Config
	portEnv           = "PORT"
	tlsEnabledEnv     = "TLS_ENABLED"
	tlsCertEnv        = "TLS_CERT"
	tlsKeyEnv         = "TLS_KEY"
	prometheusPortEnv = "PROMETHEUS_PORT"

	// Dev/Debugging
	bypassAuthEnv         = "BYPASS_AUTH" //nolint:gosec
	logLevelEnv           = "LOG_LEVEL"
	gcpProfilerEnabledEnv = "GCP_PROFILER_ENABLED"
	pprofEnabledEnv       = "PPROF"

	// RedisStreams
	metricsStreamMaxLenEnv          = "METRICS_STREAM_MAX_LEN"
	metricsStreamReadConcurrencyEnv = "METRIC_STREAM_READ_CONCURRENCY"

	// Beta features - will be short-lived and then become default behaviour in future releases
	andRulesEnv = "AND_RULES"
)

// Flags
const (
	// Service Config
	proxyKeyFlag              = "proxy-key"
	clientServiceFlag         = "client-service"
	metricServiceFlag         = "metric-service"
	authSecretFlag            = "auth-secret"
	legacySecretsFlag         = "legacy-secrets"
	metricPostDurationFlag    = "metric-post-duration"
	heartbeatIntervalFlag     = "heartbeat-interval"
	generateOfflineConfigFlag = "generate-offline-config"
	readReplicaFlag           = "readReplica"
	forwardTargetsFlag        = "forward-targets"

	// Cache Config
	configDirFlag     = "config-dir"
	offlineFlag       = "offline"
	redisAddressFlag  = "redis-address"
	redisPasswordFlag = "redis-password"
	redisUsernameFlag = "redis-username"
	redisDBFlag       = "redis-db"

	// Redis Configuration Options
	redisMaxRetriesFlag                  = "redis-max-retries"
	redisMinRetryBackoffMillisecondsFlag = "redis-min-retry-backoff-milliseconds"
	redisMaxRetryBackoffMillisecondsFlag = "redis-max-retry-backoff-milliseconds"

	redisDialTimeoutSecondsFlag  = "redis-dial-timeout-seconds"
	redisReadTimeoutSecondsFlag  = "redis-read-timeout-seconds"
	redisWriteTimeoutSecondsFlag = "redis-write-timeout-seconds"

	redisPoolSizeFlag           = "redis-pool-size"
	redisPoolSizeLiteralFlag    = "redis-pool-size-literal"
	redisPoolTimeoutSecondsFlag = "redis-pool-timeout"

	redisMinIdleConnsFlag           = "redis-min-idle-conns"
	redisMaxIdleConnsFlag           = "redis-max-idle-conns"
	redisMaxActiveConnsFlag         = "redis-max-active-conns"
	redisConnMaxIdleTimeMinutesFlag = "redis-conn-max-idle-time-minutes"
	redisConnMaxLifetimeMinutesFlag = "redis-conn-max-lifetime-minutes"

	// Redis TLS/mTLS Config Flags
	redisTLSEnabledFlag            = "redis-tls-enabled"
	redisTLSModeFlag               = "redis-tls-mode"
	redisTLSCACertFlag             = "redis-tls-ca-cert"
	redisTLSClientCertFlag         = "redis-tls-client-cert"
	redisTLSClientKeyFlag          = "redis-tls-client-key"
	redisTLSInsecureSkipVerifyFlag = "redis-tls-insecure-skip-verify"
	redisTLSServerNameFlag         = "redis-tls-server-name"

	// Server Config
	portFlag           = "port"
	tlsEnabledFlag     = "tls-enabled"
	tlsCertFlag        = "tls-cert"
	tlsKeyFlag         = "tls-key"
	prometheusPortFlag = "prometheus-port"

	// Dev/Debugging
	bypassAuthFlag         = "bypass-auth"
	logLevelFlag           = "log-level"
	pprofEnabledFlag       = "pprof"
	gcpProfilerEnabledFlag = "gcp-profiler-enabled"

	// RedisStreams
	metricsStreamMaxLenFlag         = "metrics-stream-max-len"
	metricStreamReadConcurrencyFlag = "metrics-stream-read-concurrency"

	// Beta features - will be short-lived and then become default behaviour in future releases
	andRulesFlag = "and-rules"
)

// nolint:gochecknoinits
func init() {
	// Service Config
	flag.StringVar(&proxyKey, proxyKeyFlag, "", "The ProxyKey you want to configure your Proxy to use")
	flag.StringVar(&clientService, clientServiceFlag, "https://config.ff.harness.io/api/1.0", "the url of the ff client service")
	flag.StringVar(&metricService, metricServiceFlag, "https://events.ff.harness.io/api/1.0", "the url of the ff metric service")
	flag.StringVar(&authSecret, authSecretFlag, "secret", "the secret used for signing auth tokens")
	flag.Var(&legacyAuthSecrets, legacySecretsFlag, "legacy secrets used to decode auth tokens. If rotating a secret you can place the old auth-secret in here so old tokens will still remain valid while new tokens are issued using the new auth secret")
	flag.IntVar(&metricPostDuration, metricPostDurationFlag, 60, "How often in seconds the proxy posts metrics to Harness. Set to 0 to disable.")
	flag.IntVar(&heartbeatInterval, heartbeatIntervalFlag, 60, "How often in seconds the proxy polls pings it's health function. Set to 0 to disable.")
	flag.BoolVar(&generateOfflineConfig, generateOfflineConfigFlag, false, "if true the proxy will produce offline config in the /config directory then terminate")
	flag.BoolVar(&readReplica, readReplicaFlag, false, "if true the Proxy will operate as a read replica that only reads from the cache and doesn't fetch new data from Harness SaaS")
	flag.BoolVar(&forwardTargets, forwardTargetsFlag, false, "determines if the Proxy forwards targets to Saas during the auth flow")

	// Cache Config
	flag.BoolVar(&offline, offlineFlag, false, "enables side loading of data from config dir")
	flag.StringVar(&configDir, configDirFlag, "/config", "specify a custom path to search for the offline config directory. Defaults to /config")
	flag.StringVar(&redisAddress, redisAddressFlag, "", "Redis host:port address")
	flag.StringVar(&redisPassword, redisPasswordFlag, "", "Optional. Redis password")
	flag.StringVar(&redisUsername, redisUsernameFlag, "", "Optional. Redis username")
	flag.IntVar(&redisDB, redisDBFlag, 0, "Database to be selected after connecting to the server.")

	// Redis Configuration Options
	flag.IntVar(&redisMaxRetries, redisMaxRetriesFlag, 3, "Maximum number of retries before giving up. Default is 3 retries.")
	flag.IntVar(&redisMinRetryBackoffMilliseconds, redisMinRetryBackoffMillisecondsFlag, 8, "Minimum backoff between each retry.Default is 8 milliseconds;")
	flag.IntVar(&redisMaxRetryBackoffMilliseconds, redisMaxRetryBackoffMillisecondsFlag, 512, "Maximum backoff between each retry. Default is 512 milliseconds")

	flag.IntVar(&redisDialTimeoutSeconds, redisDialTimeoutSecondsFlag, 5, "Dial timeout for establishing new connections. Default is 5 seconds")
	flag.IntVar(&redisReadTimeoutSeconds, redisReadTimeoutSecondsFlag, 3, " Timeout for socket reads. If reached, commands will fail with a timeout instead of blocking. Default is 3 seconds.")
	flag.IntVar(&redisWriteTimeoutSeconds, redisWriteTimeoutSecondsFlag, 3, "Timeout for socket writes. If reached, commands will fail with a timeout instead of blocking. Default is 3 seconds.")

	flag.IntVar(&redisPoolSize, redisPoolSizeFlag, 10, "Legacy setting that has been kept for backwards compatibility, you may want to use REDIS_POOL_SIZE_LITERAL going forward. This sets the redis connection pool size, to this value multiplied by the number of CPU available. E.g if this value is 10 and you've 2 CPU the connection pool size will be 20")
	flag.IntVar(&redisPoolSizeLiteral, redisPoolSizeLiteralFlag, 0, "sets the maximum number of socket connections to the literal value passed e.g. setting this to 10 means the connection pool size will be 10. If not specified, this will be set to the default value of 10 per CPU")
	flag.IntVar(&redisPoolTimeoutSeconds, redisPoolTimeoutSecondsFlag, 4, "Amount of time client waits for connection if all connections are busy before returning an error.Default is 4 seconds (default ReadTimeout + 1 second)")

	flag.IntVar(&redisMinIdleConns, redisMinIdleConnsFlag, 0, "Minimum number of idle connections which is useful when establishing new connection is slow.")
	flag.IntVar(&redisMaxIdleConns, redisMaxIdleConnsFlag, 0, "MaxIdleConns is the maximum number of idle connections. The idle connections are not closed by default. Default: 0")
	flag.IntVar(&redisMaxActiveConns, redisMaxActiveConnsFlag, 0, "MaxActiveConns is the maximum number of connections allocated by the pool at a given time. When zero, there is no limit on the number of connections in the pool. If the pool is full, the next call to Get() will block until a connection is released.")
	flag.IntVar(&redisConnMaxIdleTimeMinutes, redisConnMaxIdleTimeMinutesFlag, 30, "The maximum amount of time a connection may be idle. Should be less than server's timeout. Expired connections may be closed lazily before reuse. If d <= 0, connections are not closed due to a connection's idle time. -1 disables idle timeout check. Default: 30 minutes")
	flag.IntVar(&redisConnMaxLifetimeMinutes, redisConnMaxLifetimeMinutesFlag, 0, "The maximum amount of time a connection may be reused. Expired connections may be closed lazily before reuse. If <= 0, connections are not closed due to a connection's age. Default: 0")

	// Redis TLS/mTLS Config
	flag.BoolVar(&redisTLSEnabled, redisTLSEnabledFlag, false, "enable TLS/mTLS for Redis")
	flag.StringVar(&redisTLSMode, redisTLSModeFlag, "", "TLS mode: 'tls' or 'mtls'")
	flag.StringVar(&redisTLSCACertPath, redisTLSCACertFlag, "", "path to CA certificate file")
	flag.StringVar(&redisTLSClientCertPath, redisTLSClientCertFlag, "", "path to client certificate file (mTLS)")
	flag.StringVar(&redisTLSClientKeyPath, redisTLSClientKeyFlag, "", "path to client private key file (mTLS)")
	flag.BoolVar(&redisTLSInsecureSkipVerify, redisTLSInsecureSkipVerifyFlag, false, "skip server certificate verification")
	flag.StringVar(&redisTLSServerName, redisTLSServerNameFlag, "", "server name for TLS SNI")

	// Server Config
	flag.IntVar(&port, portFlag, 8000, "port the relay proxy service is exposed on, default's to 8000")
	flag.BoolVar(&tlsEnabled, tlsEnabledFlag, false, "if true the proxy will use the tlsCert and tlsKey to run with https enabled")
	flag.StringVar(&tlsCert, tlsCertFlag, "", "Path to tls cert file. Required if tls enabled is true.")
	flag.StringVar(&tlsKey, tlsKeyFlag, "", "Path to tls key file. Required if tls enabled is true.")
	flag.IntVar(&prometheusPort, prometheusPortFlag, 8000, "port that the prometheus metrics are exposed on, defaults to 8000")

	// Dev/Debugging
	flag.BoolVar(&bypassAuth, bypassAuthFlag, false, "bypasses authentication")
	flag.StringVar(&logLevel, logLevelFlag, "INFO", "sets the logging level, valid options are INFO, DEBUG & ERROR")
	flag.BoolVar(&pprofEnabled, pprofEnabledFlag, false, "enables pprof on port 6060")
	flag.BoolVar(&gcpProfilerEnabled, gcpProfilerEnabledFlag, false, "Enables gcp cloud profiler")

	// RedisStreams
	flag.Int64Var(&metricsStreamMaxLen, metricsStreamMaxLenFlag, 1000, "Sets the max length of the redis stream that replicas use to send metrics to the Primary")
	flag.IntVar(&metricsStreamReadConcurrency, metricStreamReadConcurrencyFlag, 10, "Controls the number of threads running in the Primary that listen for metrics data being sent by replicas")

	// Beta features - will be short-lived and then become default behaviour in future releases
	flag.BoolVar(&andRules, andRulesFlag, false, "if true the proxy will enable the AND rule functionality for target groups")

	loadFlagsFromEnv(map[string]string{
		bypassAuthEnv:                   bypassAuthFlag,
		logLevelEnv:                     logLevelFlag,
		offlineEnv:                      offlineFlag,
		clientServiceEnv:                clientServiceFlag,
		metricServiceEnv:                metricServiceFlag,
		authSecretEnv:                   authSecretFlag,
		legacySecretsEnv:                legacySecretsFlag,
		redisAddrEnv:                    redisAddressFlag,
		redisPasswordEnv:                redisPasswordFlag,
		redisUsernameEnv:                redisUsernameFlag,
		redisDBEnv:                      redisDBFlag,
		redisPoolSizeEnv:                redisPoolSizeFlag,
		metricPostDurationEnv:           metricPostDurationFlag,
		heartbeatIntervalEnv:            heartbeatIntervalFlag,
		pprofEnabledEnv:                 pprofEnabledFlag,
		generateOfflineConfigEnv:        generateOfflineConfigFlag,
		configDirEnv:                    configDirFlag,
		portEnv:                         portFlag,
		tlsEnabledEnv:                   tlsEnabledFlag,
		andRulesEnv:                     andRulesFlag,
		tlsCertEnv:                      tlsCertFlag,
		tlsKeyEnv:                       tlsKeyFlag,
		prometheusPortEnv:               prometheusPortFlag,
		gcpProfilerEnabledEnv:           gcpProfilerEnabledFlag,
		proxyKeyEnv:                     proxyKeyFlag,
		readReplicaEnv:                  readReplicaFlag,
		metricsStreamMaxLenEnv:          metricsStreamMaxLenFlag,
		metricsStreamReadConcurrencyEnv: metricStreamReadConcurrencyFlag,
		forwardTargetsEnv:               forwardTargetsFlag,

		redisMaxRetriesEnv:                  redisMaxRetriesFlag,
		redisMinRetryBackoffMillisecondsEnv: redisMinRetryBackoffMillisecondsFlag,
		redisMaxRetryBackoffMillisecondsEnv: redisMaxRetryBackoffMillisecondsFlag,

		redisDialTimeoutSecondsEnv:  redisDialTimeoutSecondsFlag,
		redisReadTimeoutSecondsEnv:  redisReadTimeoutSecondsFlag,
		redisWriteTimeoutSecondsEnv: redisWriteTimeoutSecondsFlag,

		redisPoolSizeLiteralEnv:    redisPoolSizeLiteralFlag,
		redisPoolTimeoutSecondsEnv: redisPoolTimeoutSecondsFlag,

		redisMinIdleConnsEnv:           redisMinIdleConnsFlag,
		redisMaxIdleConnsEnv:           redisMaxIdleConnsFlag,
		redisMaxActiveConnsEnv:         redisMaxActiveConnsFlag,
		redisConnMaxIdleTimeMinutesEnv: redisConnMaxIdleTimeMinutesFlag,
		redisConnMaxLifetimeMinutesEnv: redisConnMaxLifetimeMinutesFlag,

		redisTLSEnabledEnv:            redisTLSEnabledFlag,
		redisTLSModeEnv:               redisTLSModeFlag,
		redisTLSCACertEnv:             redisTLSCACertFlag,
		redisTLSClientCertEnv:         redisTLSClientCertFlag,
		redisTLSClientKeyEnv:          redisTLSClientKeyFlag,
		redisTLSInsecureSkipVerifyEnv: redisTLSInsecureSkipVerifyFlag,
		redisTLSServerNameEnv:         redisTLSServerNameFlag,
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

		profilerDebug := false
		if strings.ToUpper(logLevel) == "DEBUG" {
			profilerDebug = true
		}

		err := profiler.Start(
			profiler.Config{
				Service:        serviceName,
				ServiceVersion: build.Version,
				DebugLogging:   profilerDebug,
			})
		if err != nil {
			logger.Info("unable to start gcp profiler", "err", err)
		}
	}

	requiredFlags := map[string]interface{}{
		redisAddrEnv:  redisAddress,
		authSecretEnv: authSecret,
	}
	if !readReplica {
		requiredFlags[proxyKeyEnv] = proxyKey
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

	logger.Info("service config",
		"version", build.Version,
		"pprof", pprofEnabled,
		"log-level", logLevel,
		"bypass-auth", bypassAuth,
		"offline", offline,
		"port", port,
		"redis-addr", redisAddress,
		"redis-db", redisDB,
		"redis-tls-enabled", redisTLSEnabled,
		"redis-tls-mode", redisTLSMode,
		"heartbeat-interval", fmt.Sprintf("%ds", heartbeatInterval),
		"config-dir", configDir,
		"tls-enabled", tlsEnabled,
		"tls-cert", tlsCert,
		"tls-key", tlsKey,
		"read-replica", readReplica,
		"client-service", clientService,
		"metrics-service", metricService,
		"prometheus-port", prometheusPort,
		"and-rules", andRules,
	)

	var redisClient redis.UniversalClient
	var hashCache *cache.HashCache

	if redisAddress != "" && !generateOfflineConfig { //nolint:nestif
		redisConfig := buildRedisConfig()
		var err error
		redisClient, err = redisclient.NewClient(redisConfig, logger)
		if err != nil {
			logger.Error("failed to create redis client", "err", err)
			os.Exit(1)
		}

		mcMetrics := cache.NewMemoizeMetrics("proxy", promReg)
		mcCache := cache.NewMemoizeCache(redisClient, 1*time.Minute, 2*time.Minute, mcMetrics)
		sdkCache = cache.NewMetricsCache("redis", promReg, mcCache)
		hashCache = cache.NewHashCache(cache.NewKeyValCache(redisClient), 10*time.Minute, 12*time.Minute)

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

	const (
		streamHealthKey = "ffproxy_saas_stream_health"
	)

	var (
		messageHandler domain.MessageHandler

		gpc = gripcontrol.NewGripPubControl([]map[string]interface{}{
			{
				"control_uri": "http://localhost:5561",
			},
		})

		keyvalCache      = cache.NewKeyValCache(redisClient)
		sHealth          = stream.NewHealth(logger, streamHealthKey, keyvalCache, readReplica)
		streamHealth     = stream.NewStreamHealthMetrics(sHealth, promReg)
		connectedStreams = domain.NewSafeMap()

		getConnectedStreams = func() map[string]interface{} {
			return connectedStreams.Get()
		}

		pushpinStream domain.Stream = stream.NewPushpin(gpc)
		redisStream   domain.Stream = stream.NewRedisStream(redisClient)
	)

	// If we're running as the primary we kick off a routine to make sure that cached status matches
	// the in memory status.
	// If we're running as replicas we kick off a routine to make sure the in memory status matches the
	// cached status
	// nolint:nestif
	if !readReplica {
		h, ok := sHealth.(stream.PrimaryHealth)
		if !ok {
			logger.Error("got unexpected type for streamHealth", "expected", "stream.PrimaryHealth", "got", fmt.Sprintf("%T", h))
		} else {
			go h.VerifyStreamStatus(ctx, 60*time.Second)
		}
	} else {
		go getStreamStatusForReplica(ctx, keyvalCache, logger, streamHealth, streamHealthKey)
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
		sseStreamTopic     = "proxy:sse_events"
		controlEventsTopic = "proxy:primary_to_replica_control_events"
	)

	// Wait for pushpin to be available before continuing FFM-12445
	if !pushpinHealthy(ctx, logger, pushpin) {
		logger.Error("failed to connect to pushpin, pushpin streaming was not available")
		os.Exit(1)
	}

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
		stream.WithOnDisconnect(stream.ReadReplicaSSEStreamOnDisconnect(logger, sseStreamTopic)),
		stream.WithBackoff(backoff.NewConstantBackOff(1*time.Minute)),
	)

	primaryToReplicaControlStream := stream.NewStream(
		logger,
		controlEventsTopic,
		redisStream,
		domain.NewReadReplicaMessageHandler(logger, streamHealth, getConnectedStreams, pushpin),
		stream.WithOnDisconnect(stream.ReadReplicaSSEStreamOnDisconnect(logger, controlEventsTopic)),
		stream.WithBackoff(backoff.NewConstantBackOff(1*time.Minute)),
	)

	if !readReplica {
		s := stream.NewStatusWorker(streamHealth, primaryToReplicaControlStream, logger)
		go s.Start(ctx)
	}

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

		// Set the accountID in the context, this way it can be included in headers
		// for any requests the Proxy makes to Saas
		ctx = context.WithValue(ctx, domain.ContextKeyAccountID, conf.AccountID())
	}

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

		pollingStatus := stream.NewPollingStatusMetric(promReg)

		streamURL := fmt.Sprintf("%s/stream?cluster=%s", clientService, conf.ClusterIdentifier())
		sseClient := stream.NewSSEClient(
			logger,
			streamURL,
			proxyKey,
			conf.Token(),
			conf.AccountID(),
			stream.SaasStreamOnConnect(logger, streamHealth, reloadConfig, primaryToReplicaControlStream, pollingStatus),
			stream.SaasStreamOnDisconnect(logger, streamHealth, pushpin, primaryToReplicaControlStream, getConnectedStreams, reloadConfig, pollingStatus),
		)

		saasStream := stream.NewStream(
			logger,
			"*",
			stream.NewPrometheusStream("ff_proxy_saas_to_primary_sse_consumer", sseClient, promReg),
			messageHandler,
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
	proxyHealth := health.NewProxyHealth(logger, configStatus, streamHealth.Status, cacheHealthCheck)
	proxyHealth.PollCacheHealth(ctx, 1*time.Minute)

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
			streamStatus, err := streamHealth.Status(ctx)
			if err != nil {
				logger.Error("failed to check status of saas -> proxy stream health", "err", err)
				return false
			}
			return streamStatus.State == domain.StreamStateConnected
		},
		SDKStreamConnected: func(envID string) {
			connectedStreams.Set(envID, "")
		},
		ForwardTargets:  forwardTargets,
		AndRulesEnabled: andRules,
	})

	// Configure endpoints and server
	legacySecretsBytes := make([][]byte, 0)
	for _, s := range legacyAuthSecrets {
		legacySecretsBytes = append(legacySecretsBytes, []byte(s))
	}
	endpoints := transport.NewEndpoints(service)
	server := transport.NewHTTPServer(port, endpoints, logger, tlsEnabled, tlsCert, tlsKey)
	server.Use(
		middleware.NewPrometheusMiddleware(promReg),
		middleware.AllowQuerySemicolons(),
		middleware.NewCorsMiddleware(),
		middleware.NewEchoRequestIDMiddleware(),
		middleware.NewEchoLoggingMiddleware(logger),
		middleware.NewEchoAuthMiddleware(logger, authRepo, []byte(authSecret), legacySecretsBytes, bypassAuth, promReg),
		middleware.ValidateEnvironment(bypassAuth),
	)

	// We want to be able to expose prometheus metrics on a different server than the
	// main Proxy server but also need to maintain backwards compatability. By default,
	// the prometheusPort is set to the same value as the main Proxy server port
	// which means the default behaviour is for Prometheus metrics to be exposed on the
	// main Proxy server
	//
	// But if users configure `prometheusPort` to be different from `port`, then we'll start up
	// a dedicated server for exposing prometheus metrics
	if prometheusPort == port {
		prometheusHandler := promhttp.HandlerFor(promReg, promhttp.HandlerOpts{Registry: promReg})
		if err := server.WithCustomHandler(http.MethodGet, "/metrics", prometheusHandler); err != nil {
			logger.Error("failed to register prometheus handler on Proxy Server", "err", err)
		}
	} else {
		runPrometheusServer(ctx, prometheusPort, promReg, logger)
	}

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

func pushpinHealthy(ctx context.Context, logger log.Logger, pushpin stream.Pushpin) bool {
	const (
		maxAttempts   = 10
		sleepInterval = 5 * time.Second
	)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logger.Info("checking pushpin health", "attempt", attempt, "maxAttempts", maxAttempts)

		if pushpin.Healthy(ctx) {
			logger.Info("pushpin is healthy")
			return true
		}

		if attempt < maxAttempts {
			timer := time.NewTimer(sleepInterval)
			logger.Info("pushpin not healthy, retrying after delay", "delay", sleepInterval)

			select {
			case <-timer.C:
				// continue to the next health check attempt
				continue
			case <-ctx.Done():
				logger.Error("pushpin health check aborted due to context cancellation", "error", ctx.Err())
				timer.Stop() // Stop the timer to avoid leaks if it hasn't elapsed
				return false
			}
		}
	}

	logger.Error("pushpin failed health check after maximum attempts")
	return false
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

func buildRedisConfig() *redisclient.Config {
	return &redisclient.Config{
		Address:  redisAddress,
		Username: redisUsername,
		Password: redisPassword,
		DB:       redisDB,

		TLSEnabled:            redisTLSEnabled,
		TLSMode:               redisTLSMode,
		TLSCACertPath:         redisTLSCACertPath,
		TLSClientCertPath:     redisTLSClientCertPath,
		TLSClientKeyPath:      redisTLSClientKeyPath,
		TLSInsecureSkipVerify: redisTLSInsecureSkipVerify,
		TLSServerName:         redisTLSServerName,

		MaxRetries:                  redisMaxRetries,
		MinRetryBackoffMilliseconds: redisMinRetryBackoffMilliseconds,
		MaxRetryBackoffMilliseconds: redisMaxRetryBackoffMilliseconds,
		DialTimeoutSeconds:          redisDialTimeoutSeconds,
		ReadTimeoutSeconds:          redisReadTimeoutSeconds,
		WriteTimeoutSeconds:         redisWriteTimeoutSeconds,
		PoolSize:                    redisPoolSize,
		PoolSizeLiteral:             redisPoolSizeLiteral,
		PoolTimeoutSeconds:          redisPoolTimeoutSeconds,
		MinIdleConns:                redisMinIdleConns,
		MaxIdleConns:                redisMaxIdleConns,
		MaxActiveConns:              redisMaxActiveConns,
		ConnMaxIdleTimeMinutes:      redisConnMaxIdleTimeMinutes,
		ConnMaxLifetimeMinutes:      redisConnMaxLifetimeMinutes,
	}
}

func runPrometheusServer(ctx context.Context, port int, promReg *prometheus.Registry, logger log.Logger) {
	promServer := transport.NewPrometheusServer(port, promReg, logger)

	go func() {
		if err := promServer.Serve(); err != nil {
			logger.Error("prometheus server stopped unexpectedly", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()

		logger.Info("received interrupt, shutting down prometheus server...")

		if err := promServer.Shutdown(ctx); err != nil {
			logger.Error("prometheus server error'd during shutdown", "err", err)
			os.Exit(1)
		}
	}()
}

func getStreamStatusForReplica(ctx context.Context, c cache.Cache, log log.Logger, h stream.Health, key string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	status := domain.StreamStatus{}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Info("getting cached stream status as a part of the startup flow")

			if err := c.Get(ctx, key, &status); err != nil {
				log.Error("failed to get stream status from cache, backing off and retrying in 5 seconds", "err", err)
				continue
			}

			if status.State == domain.StreamStateInitializing {
				log.Info("cached stream status is still initializing... backing off and fetching it again in 5 seconds")
				continue
			}

			if status.State == domain.StreamStateConnected {
				if err := h.SetHealthy(ctx); err != nil {
					log.Error("failed to set healthy stream status in read replica", "err", err)
				}
				log.Info("successfully retrieved cached status and set it in memory", "state", status.State, "since", status.Since)
				return
			}

			if status.State == domain.StreamStateDisconnected {
				if err := h.SetUnhealthy(ctx); err != nil {
					log.Error("failed to set unhealthy status in read replica", "err", err)
				}
				log.Info("successfully retrieved cached status and set it in memory", "state", status.State, "since", status.Since)
				return
			}
		}
	}
}
