package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/harness/ff-proxy/stream"

	"github.com/harness/ff-proxy/token"

	"github.com/harness/ff-proxy/export"

	"github.com/fanout/go-gripcontrol"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-retryablehttp"

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

type sdkClientMap struct {
	*sync.RWMutex
	m map[string]*harness.CfClient
}

func newSDKClientMap() *sdkClientMap {
	return &sdkClientMap{
		RWMutex: &sync.RWMutex{},
		m:       map[string]*harness.CfClient{},
	}
}

func (s *sdkClientMap) set(key string, value *harness.CfClient) {
	s.Lock()
	defer s.Unlock()
	s.m[key] = value
}

func (s *sdkClientMap) copy() map[string]*harness.CfClient {
	s.RLock()
	defer s.RUnlock()
	return s.m
}

// keys implements the flag.Value interface and allows us to pass a comma seperated
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
	sdkClients            *sdkClientMap
	sdkCache              cache.Cache
	pprofEnabled          bool
	flagPollInterval      int
	flagStreamEnabled     bool
	generateOfflineConfig bool
	configDir             string
	port                  int
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
	flag.IntVar(&targetPollDuration, targetPollDurationFlag, 60, "How often in seconds the proxy polls feature flags for Target changes. Set to 0 to disable.")
	flag.IntVar(&metricPostDuration, metricPostDurationFlag, 60, "How often in seconds the proxy posts metrics to Harness. Set to 0 to disable.")
	flag.IntVar(&heartbeatInterval, heartbeatIntervalFlag, 60, "How often in seconds the proxy polls pings it's health function")
	flag.BoolVar(&pprofEnabled, pprofEnabledFlag, false, "enables pprof on port 6060")
	flag.IntVar(&flagPollInterval, flagPollIntervalFlag, 1, "how often in minutes the proxy should poll for flag updates (if stream not connected)")
	flag.BoolVar(&flagStreamEnabled, flagStreamEnabledFlag, true, "should the proxy connect to Harness in streaming mode to get flag changes")
	flag.BoolVar(&generateOfflineConfig, generateOfflineConfigFlag, false, "if true the proxy will produce offline config in the /config directory then terminate")
	flag.StringVar(&configDir, configDirFlag, "/config", "specify a custom path to search for the offline config directory. Defaults to /config")
	flag.IntVar(&port, portFlag, 8000, "port the relay proxy service is exposed on, default's to 8000")
	sdkClients = newSDKClientMap()

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
	})

	flag.Parse()
}

func initFF(ctx context.Context, cache gosdkCache.Cache, baseURL, eventURL, envID, envIdent, projectIdent, sdkKey string, l log.Logger, eventListener sdkStream.EventStreamListener) {

	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 5
	retryClient.Logger = l.With("component", "RetryClient", "environment", envID)

	l = l.With("component", "SDK", "apiKey", sdkKey, "environmentID", envID, "environment_identifier", envIdent, "project_identifier", projectIdent)
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
	)

	sdkClients.set(envID, client)
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
	if pprofEnabled {
		go func() {
			// #nosec
			if err := http.ListenAndServe(":6060", nil); err != nil {
				stdlog.Printf("failed to start pprof server: %s \n", err)
			}
		}()
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

	// Setup logger
	logger, err := log.NewStructuredLogger(debug)
	if err != nil {
		fmt.Println("we have no logger")
		os.Exit(1)
	}

	// Setup cancelation
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-sigc
		cancel()
	}()

	logger.Info("service config", "pprof", pprofEnabled, "debug", debug, "bypass-auth", bypassAuth, "offline", offline, "port", port, "admin-service", adminService, "account-identifier", accountIdentifier, "org-identifier", orgIdentifier, "sdk-base-url", sdkBaseURL, "sdk-events-url", sdkEventsURL, "redis-addr", redisAddress, "redis-db", redisDB, "api-keys", fmt.Sprintf("%v", apiKeys), "target-poll-duration", fmt.Sprintf("%ds", targetPollDuration), "heartbeat-interval", fmt.Sprintf("%ds", heartbeatInterval), "flag-stream-enabled", flagStreamEnabled, "flag-poll-interval", fmt.Sprintf("%dm", flagPollInterval), "config-dir", configDir)

	adminService, err := services.NewAdminService(logger, adminService, adminServiceToken)
	if err != nil {
		logger.Error("failed to create admin client", "err", err)
		os.Exit(1)
	}

	// Create cache
	// if we're just generating the offline config we should only use in memory mode for now
	// when we move to a pattern of allowing periodic config dumps to disk we can remove this requirement
	if redisAddress != "" && !generateOfflineConfig {
		client := redis.NewClient(&redis.Options{
			Addr:     redisAddress,
			Password: redisPassword,
			DB:       redisDB,
		})
		logger.Info("connecting to redis", "address", redisAddress)
		sdkCache = cache.NewRedisCache(client)
		err := sdkCache.HealthCheck(ctx)
		if err != nil {
			logger.Error("failed to connect to redis", "err", err)
			os.Exit(1)
		}
	} else {
		logger.Info("initialising default memcache")
		sdkCache = cache.NewMemCache()
	}

	apiKeyHasher := hash.NewSha256()

	var (
		featureConfig map[domain.FeatureFlagKey][]domain.FeatureFlag
		targetConfig  map[domain.TargetKey][]domain.Target
		segmentConfig map[domain.SegmentKey][]domain.Segment
		authConfig    map[domain.AuthAPIKey]string
	)

	var remoteConfig config.RemoteConfig

	var streamEnabled bool

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
		// attempt to connect to pushpin stream - if we can't streaming will be disabled
		err = streamHealthCheck()
		if err != nil {
			logger.Error("failed to connect to pushpin streaming service - streaming mode not available for sdks connecting to Relay Proxy", "err", err)
		} else {
			streamEnabled = true
			logger.Info("starting stream service...")
			eventListener = stream.NewStreamWorker(logger, gpc)
		}

		logger.Info("retrieving config from ff-server...")
		remoteConfig, err = config.NewRemoteConfig(
			ctx,
			accountIdentifier,
			orgIdentifier,
			apiKeys,
			hash.NewSha256(),
			adminService,
			config.WithLogger(logger),
			config.WithConcurrency(20),
			config.WithFetchTargets(targetPollDuration != 0), // don't fetch targets if poll duration is 0
		)
		if err != nil {
			logger.Error("error(s) encountered fetching config from FeatureFlags, startup will continue but the Proxy may be missing required config", "errors", err)
		} else {
			logger.Info("successfully retrieved config from FeatureFlags")
		}

		authConfig = remoteConfig.AuthConfig()
		targetConfig = remoteConfig.TargetConfig()

		envIDToProjectEnvironmentInfo := remoteConfig.ProjectEnvironmentInfo()

		// start an sdk instance for each api key
		for _, apiKey := range apiKeys {
			apiKeyHash := apiKeyHasher.Hash(apiKey)

			// find corresponding environmentID for apiKey
			envID, ok := authConfig[domain.AuthAPIKey(apiKeyHash)]
			if !ok {
				logger.Error("API key not found, skipping", "api-key", apiKey)
				continue
			}

			projEnvInfo := envIDToProjectEnvironmentInfo[authConfig[domain.AuthAPIKey(apiKeyHash)]]

			cacheWrapper := cache.NewWrapper(sdkCache, envID, logger)
			go initFF(ctx, cacheWrapper, sdkBaseURL, sdkEventsURL, envID, projEnvInfo.EnvironmentIdentifier, projEnvInfo.ProjectIdentifier, apiKey, logger, eventListener)
		}
	}

	// Create repos
	tr, err := repository.NewTargetRepo(sdkCache, targetConfig)
	if err != nil {
		logger.Error("failed to create target repo", "err", err)
		os.Exit(1)
	}

	fcr, err := repository.NewFeatureFlagRepo(sdkCache, featureConfig)
	if err != nil {
		logger.Error("failed to create feature config repo", "err", err)
		os.Exit(1)
	}

	sr, err := repository.NewSegmentRepo(sdkCache, segmentConfig)
	if err != nil {
		logger.Error("failed to create segment repo", "err", err)
		os.Exit(1)
	}

	authRepo, err := repository.NewAuthRepo(sdkCache, authConfig)
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

	featureEvaluator := proxyservice.NewFeatureEvaluator()

	metricsEnabled := metricPostDuration != 0 && !offline
	metricService, err := services.NewMetricService(logger, metricService, accountIdentifier, adminServiceToken, metricsEnabled)
	if err != nil {
		logger.Error("failed to create client for the feature flags metric service", "err", err)
		os.Exit(1)
	}

	clientService, err := services.NewClientService(logger, clientService)
	if err != nil {
		logger.Error("failed to create client for the feature flags client service", "err", err)
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
		EnvHealthFn:      envHealthCheck,
		AuthFn:           tokenSource.GenerateToken,
		Evaluator:        featureEvaluator,
		ClientService:    clientService,
		MetricService:    metricService,
		Offline:          offline,
		Hasher:           apiKeyHasher,
		StreamingEnabled: streamEnabled,
	})

	// Configure endpoints and server
	endpoints := transport.NewEndpoints(service)
	server := transport.NewHTTPServer(port, endpoints, logger)
	server.Use(
		middleware.NewEchoRequestIDMiddleware(),
		middleware.NewEchoLoggingMiddleware(),
		middleware.NewEchoAuthMiddleware([]byte(authSecret), bypassAuth),
	)

	go func() {
		<-ctx.Done()
		logger.Info("recevied interrupt, shutting down server...")

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
				for targetConfig := range remoteConfig.PollTargets(ctx, ticker.C) {
					for key, values := range targetConfig {
						tr.DeltaAdd(ctx, key, values...)
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
						for _, client := range sdkClients.copy() {
							clusterIdentifier = client.GetClusterIdentifier()
							break
						}
						logger.Info("sending metrics")
						metricService.SendMetrics(ctx, clusterIdentifier)
					}
				}
			}()
		} else {
			logger.Info("sending metrics disabled")
		}
	}

	go func() {
		ticker := time.NewTicker(time.Duration(heartbeatInterval) * time.Second)
		logger.Info(fmt.Sprintf("polling heartbeat every %d seconds", heartbeatInterval))
		heartbeat(ctx, ticker.C, fmt.Sprintf("http://localhost:%d", port), logger)
	}()

	if err := server.Serve(); err != nil {
		logger.Error("server stopped", "err", err)
	}
}

// checks the health of the connected cache instance
func cacheHealthCheck(ctx context.Context) error {
	return sdkCache.HealthCheck(ctx)
}

// heartbeat kicks off a goroutine that polls the /health endpoint at intervals
// determined by how frequently events are sent on the tick channel.
func heartbeat(ctx context.Context, tick <-chan time.Time, listenAddr string, logger log.StructuredLogger) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Info("stopping heartbeat")
				return
			case <-tick:
				resp, err := http.Get(fmt.Sprintf("%s/health", listenAddr))
				if err != nil {
					logger.Error(fmt.Sprintf("heartbeat request failed: %d", resp.StatusCode))
				}

				if resp.StatusCode == http.StatusOK {
					logger.Info(fmt.Sprintf("heartbeat healthy: status code: %d", resp.StatusCode))
					resp.Body.Close()
					continue
				}

				b, err := io.ReadAll(resp.Body)
				if err != nil {
					resp.Body.Close()
					logger.Error(fmt.Sprintf("failed to read response body from %s", resp.Request.URL.String()))
					logger.Error(fmt.Sprintf("heartbeat unhealthy: status code: %d", resp.StatusCode))
					continue
				}
				resp.Body.Close()

				logger.Error(fmt.Sprintf("heartbeat unhealthy: status code: %d, body: %s", resp.StatusCode, b))
			}
		}
	}()
}

// checks the health of all connected environments
// returns an error, if any for each
func envHealthCheck(ctx context.Context) map[string]error {
	envHealth := map[string]error{}

	// if the user chooses to run connected sdks in polling mode then we can skip the stream health checks
	if !flagStreamEnabled {
		return envHealth
	}

	for env, sdk := range sdkClients.copy() {
		// get SDK health details
		var err error
		streamConnected := sdk.IsStreamConnected()
		if !streamConnected {
			err = fmt.Errorf("environment %s unhealthy, stream not connected", env)
		}
		envHealth[env] = err
	}
	return envHealth
}

// streamHealthCheck checks if the GripControl stream is available - this is required to enable streaming mode
func streamHealthCheck() error {
	var multiErr error

	for i := 0; i < 2; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:5561"))
		if err != nil || resp == nil {
			multiErr = multierror.Append(fmt.Errorf("gpc request failed streaming will be disabled: %s", err), multiErr)
		} else if resp != nil && resp.StatusCode != http.StatusOK {
			multiErr = multierror.Append(fmt.Errorf("gpc request failed streaming will be disabled: %d", resp.StatusCode), multiErr)
		} else {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return multiErr
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
		stdlog.Fatalf("The following configuaration values are required: %v ", unset)
	}
}

// checks that all connected sdks initialised successfully
func sdksInitialised() bool {
	// wait for all specified sdks to be started
	var sdksStarted bool
	for i := 0; i < 20; i++ {
		if len(apiKeys) == len(sdkClients.copy()) {
			sdksStarted = true
			break
		}
		time.Sleep(time.Second * 1)
	}
	if !sdksStarted {
		return false
	}

	// check that all the sdks have fetched flags/segments
	for _, sdk := range sdkClients.copy() {
		init, _ := sdk.IsInitialized()
		if !init {
			return false
		}
	}
	return true
}
