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

	"github.com/harness/ff-proxy/build"
	"github.com/harness/ff-proxy/export"
	"github.com/harness/ff-proxy/health"
	"github.com/harness/ff-proxy/stream"
	"github.com/harness/ff-proxy/token"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/fanout/go-gripcontrol"
	"github.com/hashicorp/go-retryablehttp"

	"cloud.google.com/go/profiler"

	_ "net/http/pprof" //#nosec

	"github.com/go-redis/redis/v8"

	gosdkCache "github.com/harness/ff-golang-server-sdk/cache"
	harness "github.com/harness/ff-golang-server-sdk/client"
	"github.com/harness/ff-golang-server-sdk/logger"
	sdkStream "github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/config"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/hash"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/middleware"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
	"github.com/harness/ff-proxy/repository"
	"github.com/harness/ff-proxy/services"
	"github.com/harness/ff-proxy/transport"
)

// keys implements the flag.Value interface and allows us to pass a comma separated
// list of api keys e.g. -api-keys 123,456,789
type keys []string

func (i *keys) String() string {
	return strings.Join(*i, ",")
}

func (i *keys) Set(value string) error {
	ss := strings.Split(value, ",")
	for _, s := range ss {
		*i = append(*i, s)
	}
	return nil
}

func (i *keys) PrintMasked() string {
	var maskedKeys []string
	for _, key := range *i {
		maskedKeys = append(maskedKeys, token.MaskRight(key))
	}
	return strings.Join(maskedKeys, ",")
}

var (
	debug                 bool
	bypassAuth            bool
	offline               bool
	accountIdentifier     string
	orgIdentifier         string
	adminService          string
	adminServiceToken     string
	clientService         string
	metricService         string
	authSecret            string
	sdkBaseURL            string
	sdkEventsURL          string
	redisAddress          string
	redisPassword         string
	redisDB               int
	apiKeys               keys
	targetPollDuration    int
	metricPostDuration    int
	heartbeatInterval     int
	sdkClients            *domain.SDKClientMap
	sdkCache              cache.Cache
	pprofEnabled          bool
	flagPollInterval      int
	flagStreamEnabled     bool
	generateOfflineConfig bool
	configDir             string
	port                  int
	tlsEnabled            bool
	tlsCert               string
	tlsKey                string
	gcpProfilerEnabled    bool
)

const (
	bypassAuthEnv            = "BYPASS_AUTH"
	debugEnv                 = "DEBUG"
	offlineEnv               = "OFFLINE"
	accountIdentifierEnv     = "ACCOUNT_IDENTIFIER"
	orgIdentifierEnv         = "ORG_IDENTIFIER"
	adminServiceEnv          = "ADMIN_SERVICE"
	adminServiceTokenEnv     = "ADMIN_SERVICE_TOKEN"
	clientServiceEnv         = "CLIENT_SERVICE"
	metricServiceEnv         = "METRIC_SERVICE"
	authSecretEnv            = "AUTH_SECRET"
	sdkBaseURLEnv            = "SDK_BASE_URL"
	sdkEventsURLEnv          = "SDK_EVENTS_URL"
	redisAddrEnv             = "REDIS_ADDRESS"
	redisPasswordEnv         = "REDIS_PASSWORD"
	redisDBEnv               = "REDIS_DB"
	apiKeysEnv               = "API_KEYS"
	targetPollDurationEnv    = "TARGET_POLL_DURATION"
	metricPostDurationEnv    = "METRIC_POST_DURATION"
	heartbeatIntervalEnv     = "HEARTBEAT_INTERVAL"
	flagPollIntervalEnv      = "FLAG_POLL_INTERVAL"
	flagStreamEnabledEnv     = "FLAG_STREAM_ENABLED"
	generateOfflineConfigEnv = "GENERATE_OFFLINE_CONFIG"
	configDirEnv             = "CONFIG_DIR"
	pprofEnabledEnv          = "PPROF"
	portEnv                  = "PORT"
	tlsEnabledEnv            = "TLS_ENABLED"
	tlsCertEnv               = "TLS_CERT"
	tlsKeyEnv                = "TLS_KEY"
	gcpProfilerEnabledEnv    = "GCP_PROFILER_ENABLED"

	bypassAuthFlag            = "bypass-auth"
	debugFlag                 = "debug"
	offlineFlag               = "offline"
	accountIdentifierFlag     = "account-identifier"
	orgIdentifierFlag         = "org-identifier"
	adminServiceFlag          = "admin-service"
	adminServiceTokenFlag     = "admin-service-token"
	clientServiceFlag         = "client-service"
	metricServiceFlag         = "metric-service"
	authSecretFlag            = "auth-secret"
	sdkBaseURLFlag            = "sdk-base-url"
	sdkEventsURLFlag          = "sdk-events-url"
	redisAddressFlag          = "redis-address"
	redisPasswordFlag         = "redis-password"
	redisDBFlag               = "redis-db"
	apiKeysFlag               = "api-keys"
	targetPollDurationFlag    = "target-poll-duration"
	metricPostDurationFlag    = "metric-post-duration"
	heartbeatIntervalFlag     = "heartbeat-interval"
	pprofEnabledFlag          = "pprof"
	flagStreamEnabledFlag     = "flag-stream-enabled"
	generateOfflineConfigFlag = "generate-offline-config"
	configDirFlag             = "config-dir"
	flagPollIntervalFlag      = "flag-poll-interval"
	portFlag                  = "port"
	tlsEnabledFlag            = "tls-enabled"
	tlsCertFlag               = "tls-cert"
	tlsKeyFlag                = "tls-key"
	gcpProfilerEnabledFlag    = "gcp-profiler-enabled"
)

func init() {
	flag.BoolVar(&bypassAuth, bypassAuthFlag, false, "bypasses authentication")
	// TODO - FFM-1812 - we should update this to be loglevel
	flag.BoolVar(&debug, debugFlag, false, "enables debug logging")
	flag.BoolVar(&offline, offlineFlag, false, "enables side loading of data from config dir")
	flag.StringVar(&accountIdentifier, accountIdentifierFlag, "", "account identifier to load remote config for")
	flag.StringVar(&orgIdentifier, orgIdentifierFlag, "", "org identifier to load remote config for")
	flag.StringVar(&adminService, adminServiceFlag, "https://app.harness.io/gateway/cf", "the url of the ff admin service")
	flag.StringVar(&adminServiceToken, adminServiceTokenFlag, "", "token to use with the ff service")
	flag.StringVar(&clientService, clientServiceFlag, "https://config.ff.harness.io/api/1.0", "the url of the ff client service")
	flag.StringVar(&metricService, metricServiceFlag, "https://events.ff.harness.io/api/1.0", "the url of the ff metric service")
	flag.StringVar(&authSecret, authSecretFlag, "secret", "the secret used for signing auth tokens")
	flag.StringVar(&sdkBaseURL, sdkBaseURLFlag, "https://config.ff.harness.io/api/1.0", "url for the sdk to connect to")
	flag.StringVar(&sdkEventsURL, sdkEventsURLFlag, "https://events.ff.harness.io/api/1.0", "url for the sdk to send metrics to")
	flag.StringVar(&redisAddress, redisAddressFlag, "", "Redis host:port address")
	flag.StringVar(&redisPassword, redisPasswordFlag, "", "Optional. Redis password")
	flag.IntVar(&redisDB, redisDBFlag, 0, "Database to be selected after connecting to the server.")
	flag.Var(&apiKeys, apiKeysFlag, "API keys to connect with ff-server for each environment")
	flag.IntVar(&targetPollDuration, targetPollDurationFlag, 0, "How often in seconds the proxy polls feature flags for Target changes. Set to 0 to disable.")
	flag.IntVar(&metricPostDuration, metricPostDurationFlag, 60, "How often in seconds the proxy posts metrics to Harness. Set to 0 to disable.")
	flag.IntVar(&heartbeatInterval, heartbeatIntervalFlag, 60, "How often in seconds the proxy polls pings it's health function. Set to 0 to disable.")
	flag.BoolVar(&pprofEnabled, pprofEnabledFlag, false, "enables pprof on port 6060")
	flag.IntVar(&flagPollInterval, flagPollIntervalFlag, 60, "how often in seconds the proxy should poll for flag updates (if stream not connected)")
	flag.BoolVar(&flagStreamEnabled, flagStreamEnabledFlag, true, "should the proxy connect to Harness in streaming mode to get flag changes")
	flag.BoolVar(&generateOfflineConfig, generateOfflineConfigFlag, false, "if true the proxy will produce offline config in the /config directory then terminate")
	flag.StringVar(&configDir, configDirFlag, "/config", "specify a custom path to search for the offline config directory. Defaults to /config")
	flag.IntVar(&port, portFlag, 8000, "port the relay proxy service is exposed on, default's to 8000")
	flag.BoolVar(&tlsEnabled, tlsEnabledFlag, false, "if true the proxy will use the tlsCert and tlsKey to run with https enabled")
	flag.StringVar(&tlsCert, tlsCertFlag, "", "Path to tls cert file. Required if tls enabled is true.")
	flag.StringVar(&tlsKey, tlsKeyFlag, "", "Path to tls key file. Required if tls enabled is true.")
	flag.BoolVar(&gcpProfilerEnabled, gcpProfilerEnabledFlag, false, "Enables gcp cloud profiler")

	sdkClients = domain.NewSDKClientMap()

	loadFlagsFromEnv(map[string]string{
		bypassAuthEnv:            bypassAuthFlag,
		debugEnv:                 debugFlag,
		offlineEnv:               offlineFlag,
		accountIdentifierEnv:     accountIdentifierFlag,
		orgIdentifierEnv:         orgIdentifierFlag,
		adminServiceEnv:          adminServiceFlag,
		adminServiceTokenEnv:     adminServiceTokenFlag,
		clientServiceEnv:         clientServiceFlag,
		metricServiceEnv:         metricServiceFlag,
		authSecretEnv:            authSecretFlag,
		sdkBaseURLEnv:            sdkBaseURLFlag,
		sdkEventsURLEnv:          sdkEventsURLFlag,
		redisAddrEnv:             redisAddressFlag,
		redisPasswordEnv:         redisPasswordFlag,
		redisDBEnv:               redisDBFlag,
		apiKeysEnv:               apiKeysFlag,
		targetPollDurationEnv:    targetPollDurationFlag,
		metricPostDurationEnv:    metricPostDurationFlag,
		heartbeatIntervalEnv:     heartbeatIntervalFlag,
		pprofEnabledEnv:          pprofEnabledFlag,
		flagStreamEnabledEnv:     flagStreamEnabledFlag,
		generateOfflineConfigEnv: generateOfflineConfigFlag,
		configDirEnv:             configDirFlag,
		flagPollIntervalEnv:      flagPollIntervalFlag,
		portEnv:                  portFlag,
		tlsEnabledEnv:            tlsEnabledFlag,
		tlsCertEnv:               tlsCertFlag,
		tlsKeyEnv:                tlsKeyFlag,
		gcpProfilerEnabledEnv:    gcpProfilerEnabledFlag,
	})

	flag.Parse()
}

func initFF(ctx context.Context, cache gosdkCache.Cache, baseURL, eventURL, envID, envIdent, projectIdent, sdkKey string, l log.Logger, eventListener sdkStream.EventStreamListener) {

	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 5
	retryClient.Logger = l.With("component", "RetryClient", "environment", envID)

	l = l.With("component", "SDK", "apiKey", token.MaskRight(sdkKey), "environmentID", envID, "environment_identifier", envIdent, "project_identifier", projectIdent)
	structuredLogger, ok := l.(log.StructuredLogger)
	if !ok {
		l.Error("unexpected logger", "expected", "log.StructuredLogger", "got", fmt.Sprintf("%T", structuredLogger))
	}

	sdklogger := logger.NewZapLoggerFromSugar(structuredLogger.Sugar())

	client, err := harness.NewCfClient(sdkKey,
		harness.WithLogger(sdklogger),
		harness.WithURL(baseURL),
		harness.WithHTTPClient(retryClient.StandardClient()),
		harness.WithEventsURL(eventURL),
		harness.WithStreamEnabled(flagStreamEnabled),
		harness.WithPullInterval(uint(flagPollInterval)),
		harness.WithCache(cache),
		harness.WithStoreEnabled(false), // store should be disabled until we implement a wrapper to handle multiple envs
		harness.WithEventStreamListener(eventListener),
		harness.WithProxyMode(true),
	)

	sdkClients.Set(envID, client)
	defer func() {
		if err := client.Close(); err != nil {
			l.Error("error while closing client", "err", err)
		}
	}()

	if err != nil {
		l.Error("could not connect to CF servers", "err", err)
	}

	<-ctx.Done()
}

func main() {
	// Setup logger
	logger, err := log.NewStructuredLogger(debug)
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
			accountIdentifierEnv: accountIdentifier,
			orgIdentifierEnv:     orgIdentifier,
			adminServiceTokenEnv: adminServiceToken,
			apiKeysEnv:           apiKeys,
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

	logger.Info("service config", "pprof", pprofEnabled, "debug", debug, "bypass-auth", bypassAuth, "offline", offline, "port", port, "admin-service", adminService, "account-identifier", accountIdentifier, "org-identifier", orgIdentifier, "sdk-base-url", sdkBaseURL, "sdk-events-url", sdkEventsURL, "redis-addr", redisAddress, "redis-db", redisDB, "api-keys", apiKeys.PrintMasked(), "target-poll-duration", fmt.Sprintf("%ds", targetPollDuration), "heartbeat-interval", fmt.Sprintf("%ds", heartbeatInterval), "flag-stream-enabled", flagStreamEnabled, "flag-poll-interval", fmt.Sprintf("%dm", flagPollInterval), "config-dir", configDir, "tls-enabled", tlsEnabled, "tls-cert", tlsCert, "tls-key", tlsKey)

	adminService, err := services.NewAdminService(logger, adminService, adminServiceToken)
	if err != nil {
		logger.Error("failed to create admin client", "err", err)
		os.Exit(1)
	}

	clientService, err := services.NewClientService(logger, clientService)
	if err != nil {
		logger.Error("failed to create client for the feature flags client service", "err", err)
		os.Exit(1)
	}

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
		sdkCache = cache.NewMetricsCache("redis", promReg, cache.NewMemoizeCache(client, 10*time.Minute, 1*time.Minute, 2*time.Minute, nil))
		//sdkCache = cache.NewMetricsCache("redis", promReg, cache.NewKeyValCache(client))
		err = sdkCache.HealthCheck(ctx)
		if err != nil {
			logger.Error("failed to connect to redis", "err", err)
			os.Exit(1)
		}
	} else {
		logger.Info("initialising default memcache")
		sdkCache = cache.NewMetricsCache("in_mem", promReg, cache.NewMemCache())
	}

	apiKeyHasher := hash.NewSha256()

	var (
		featureConfig map[domain.FeatureFlagKey]interface{}
		targetConfig  map[domain.TargetKey]interface{}
		segmentConfig map[domain.SegmentKey]interface{}
		authConfig    map[domain.AuthAPIKey]string
		approvedEnvs  = map[string]struct{}{}
	)

	var (
		remoteConfig  config.RemoteConfig
		streamEnabled bool
		environments  []string
	)

	// Load either local config from files or remote config from ff-server
	if offline && !generateOfflineConfig {
		fs := os.DirFS(configDir)
		config, err := config.NewLocalConfig(fs)
		if err != nil {
			logger.Error("failed to load config", "err", err)
			os.Exit(1)
		}
		featureConfig = config.FeatureFlag()
		targetConfig = config.Targets()
		segmentConfig = config.Segments()
		authConfig = config.AuthConfig()

		logger.Info("retrieved offline config")
	} else {
		gpc := gripcontrol.NewGripPubControl([]map[string]interface{}{
			{
				"control_uri": "http://localhost:5561",
			},
		})

		var eventListener sdkStream.EventStreamListener

		// if we're not connecting to get streaming updates we shouldn't allow downstream sdks to connect to us in streaming mode
		// because we won't receive any to forward on. This will force sdks to poll the proxy to get their updates
		if flagStreamEnabled {
			// attempt to connect to pushpin stream - if we can't streaming will be disabled
			err = health.StreamHealthCheck()
			if err != nil {
				logger.Error("failed to connect to pushpin streaming service - streaming mode not available for sdks connecting to Relay Proxy", "err", err)
			} else {
				streamEnabled = true
				logger.Info("starting stream service...")
				eventListener = stream.NewStreamWorker(logger, gpc, promReg)
			}
		} else {
			logger.Info("starting sdks in polling mode. streaming disabled for connected sdks")
			streamEnabled = false
		}

		logger.Info("retrieving config from ff-server...")
		remoteConfig, err = config.NewRemoteConfig(
			ctx,
			accountIdentifier,
			orgIdentifier,
			apiKeys,
			adminService,
			clientService,
			config.WithLogger(logger),
			config.WithFetchTargets(targetPollDuration != 0), // don't fetch targets if poll duration is 0
		)
		if err != nil {
			logger.Error("error(s) encountered fetching config from FeatureFlags, startup will continue but the Proxy may be missing required config", "errors", err)
		} else {
			logger.Info("successfully retrieved config from FeatureFlags")
		}

		authConfig = remoteConfig.AuthConfig()
		targetConfig = remoteConfig.TargetConfig()
		envInfo := remoteConfig.EnvInfo()

		// If all provided api keys auth'd successfully then restrict this proxy instance to only serve requests from those envs.
		// The reason we're still lenient here is that if a network issue causes an api key not to auth we still want to
		// fallback to serving cached data where possible. This logic can be extended/improved in future for other use cases
		// but this will lockdown requests a bit better while still giving us high availability for now. This could be coupled
		// with a new config option to exit if any keys fail for users who want to restrict fully to whats provided in the startup config
		if len(envInfo) == len(apiKeys) {
			for env := range envInfo {
				environments = append(environments, env)
				approvedEnvs[env] = struct{}{}
			}
			logger.Info("serving requests for configured environments", "environments", approvedEnvs)
		}
		logger.Info(fmt.Sprintf("successfully fetched config for %d environment(s)", len(envInfo)))
		// start an sdk instance for each valid api key
		for _, env := range envInfo {
			cacheWrapper := cache.NewWrapper(sdkCache, env.EnvironmentID, logger)
			go initFF(ctx, cacheWrapper, sdkBaseURL, sdkEventsURL, env.EnvironmentID, env.EnvironmentIdentifier, env.ProjectIdentifier, env.APIKey, logger, eventListener)
		}
	}

	// Create repos
	tr, err := repository.NewTargetRepo(sdkCache, repository.WithTargetConfig(targetConfig))
	if err != nil {
		logger.Error("failed to create target repo", "err", err)
		os.Exit(1)
	}

	fcr, err := repository.NewFeatureFlagRepo(sdkCache, repository.WithFeatureConfig(featureConfig))
	if err != nil {
		logger.Error("failed to create feature config repo", "err", err)
		os.Exit(1)
	}

	sr, err := repository.NewSegmentRepo(sdkCache, repository.WithSegmentConfig(segmentConfig))
	if err != nil {
		logger.Error("failed to create segment repo", "err", err)
		os.Exit(1)
	}

	authRepo, err := repository.NewAuthRepo(sdkCache, authConfig, approvedEnvs)
	if err != nil {
		logger.Error("failed to create auth config repo", "err", err)
		os.Exit(1)
	}

	if generateOfflineConfig {
		if sdksInitialised() {
			exportService := export.NewService(logger, fcr, tr, sr, authRepo, authConfig, configDir)
			err = exportService.Persist(ctx)
			if err != nil {
				logger.Error("offline config export failed err: %s", err)
				os.Exit(1)
			}
			os.Exit(0)
		} else {
			logger.Error("SDKs didnt initialise correctly. Failed to generate offline config")
			os.Exit(1)
		}
	}

	tokenSource := token.NewTokenSource(logger, authRepo, apiKeyHasher, []byte(authSecret))

	metricsEnabled := metricPostDuration != 0 && !offline
	metricService, err := services.NewMetricService(logger, metricService, accountIdentifier, remoteConfig.Tokens(), metricsEnabled, promReg)
	if err != nil {
		logger.Error("failed to create client for the feature flags metric service", "err", err)
		os.Exit(1)
	}

	// Setup service and middleware
	service := proxyservice.NewService(proxyservice.Config{
		Logger:           log.NewContextualLogger(logger, log.ExtractRequestValuesFromContext),
		FeatureRepo:      fcr,
		TargetRepo:       tr,
		SegmentRepo:      sr,
		AuthRepo:         authRepo,
		CacheHealthFn:    cacheHealthCheck,
		EnvHealthFn:      health.NewEnvironmentHealthTracker(ctx, environments, sdkClients, 30*time.Second),
		AuthFn:           tokenSource.GenerateToken,
		ClientService:    clientService,
		MetricService:    metricService,
		Offline:          offline,
		Hasher:           apiKeyHasher,
		StreamingEnabled: streamEnabled,
		SDKClients:       sdkClients,
	})

	// Configure endpoints and server
	endpoints := transport.NewEndpoints(service)
	server := transport.NewHTTPServer(port, endpoints, logger, tlsEnabled, tlsCert, tlsKey, promReg)
	server.Use(
		middleware.NewEchoRequestIDMiddleware(),
		middleware.NewEchoLoggingMiddleware(),
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

	if !offline {
		// start target polling ticker
		// don't poll for targets if duration is 0
		if targetPollDuration != 0 {
			go func() {
				ticker := time.NewTicker(time.Duration(targetPollDuration) * time.Second)
				defer ticker.Stop()

				logger.Info(fmt.Sprintf("polling for new targets every %d seconds", targetPollDuration))

				for {
					select {
					case <-ctx.Done():
						logger.Info("stopping poll targets ticker")
						return
					case <-ticker.C:
						// poll for all targets for each configured environment
						pollTargetConfig := make(map[string][]domain.Target)
						for _, env := range remoteConfig.EnvInfo() {
							targets, err := config.GetTargets(ctx, accountIdentifier, orgIdentifier, env.ProjectIdentifier, env.EnvironmentIdentifier, adminService)
							if err != nil {
								logger.Error("failed to poll targets for environment %s: %s", env.EnvironmentID, err)
							}
							pollTargetConfig[env.EnvironmentID] = targets
						}

						for env, values := range pollTargetConfig {
							tr.DeltaAdd(ctx, env, values...)
						}
					}
				}
			}()
		}

		// start metric sending ticker
		if metricPostDuration != 0 {
			go func() {
				logger.Info(fmt.Sprintf("sending metrics every %d seconds", metricPostDuration))
				ticker := time.NewTicker(time.Duration(metricPostDuration) * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-ctx.Done():
						logger.Info("stopping metrics ticker")
						return
					case <-ticker.C:
						// default to prod cluster
						clusterIdentifier := "1"
						// grab which cluster we're connected to from sdk
						for _, client := range sdkClients.Copy() {
							clusterIdentifier = client.GetClusterIdentifier()
							break
						}
						logger.Debug("sending metrics")
						metricService.SendMetrics(ctx, clusterIdentifier)
					}
				}
			}()
		} else {
			logger.Info("sending metrics disabled")
		}
	}

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
		case keys:
			if len(v.(keys)) == 0 {
				unset = append(unset, k)
			}
		}
	}

	if len(unset) > 0 {
		stdlog.Fatalf("The following configuration values are required: %v ", unset)
	}
}

// checks that all connected sdks initialised successfully
func sdksInitialised() bool {
	// wait for all specified sdks to be started
	var sdksStarted bool
	for i := 0; i < 20; i++ {
		if len(apiKeys) == len(sdkClients.Copy()) {
			sdksStarted = true
			break
		}
		time.Sleep(time.Second * 1)
	}
	if !sdksStarted {
		return false
	}

	// check that all the sdks have fetched flags/segments
	for _, sdk := range sdkClients.Copy() {
		init, _ := sdk.IsInitialized()
		if !init {
			return false
		}
	}
	return true
}
