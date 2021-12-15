package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/go-redis/redis/v8"

	sdkCache "github.com/harness/ff-golang-server-sdk/cache"
	harness "github.com/harness/ff-golang-server-sdk/client"
	"github.com/harness/ff-golang-server-sdk/logger"
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
	"github.com/wings-software/ff-server/pkg/hash"
)

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
	debug              bool
	bypassAuth         bool
	offline            bool
	port               int
	accountIdentifier  string
	orgIdentifier      string
	adminService       string
	adminServiceToken  string
	clientService      string
	authSecret         string
	sdkBaseURL         string
	sdkEventsURL       string
	redisAddress       string
	redisPassword      string
	redisDB            int
	apiKeys            keys
	targetPollDuration int
)

const (
	bypassAuthEnv         = "BYPASS_AUTH"
	debugEnv              = "DEBUG"
	offlineEnv            = "OFFLINE"
	portEnv               = "PORT"
	accountIdentifierEnv  = "ACCOUNT_IDENTIFIER"
	orgIdentifierEnv      = "ORG_IDENTIFIER"
	adminServiceEnv       = "ADMIN_SERVICE"
	adminServiceTokenEnv  = "ADMIN_SERVICE_TOKEN"
	clientServiceEnv      = "CLIENT_SERVICE"
	authSecretEnv         = "AUTH_SECRET"
	sdkBaseURLEnv         = "SDK_BASE_URL"
	sdkEventsURLEnv       = "SDK_EVENTS_URL"
	redisAddrEnv          = "REDIS_ADDRESS"
	redisPasswordEnv      = "REDIS_PASSWORD"
	redisDBEnv            = "REDIS_DB"
	apiKeysEnv            = "API_KEYS"
	targetPollDurationEnv = "TARGET_POLL_DURATION"

	bypassAuthFlag         = "bypass-auth"
	debugFlag              = "debug"
	offlineFlag            = "offline"
	portFlag               = "port"
	accountIdentifierFlag  = "account-identifier"
	orgIdentifierFlag      = "org-identifier"
	adminServiceFlag       = "admin-service"
	adminServiceTokenFlag  = "admin-service-token"
	clientServiceFlag      = "client-service"
	authSecretFlag         = "auth-secret"
	sdkBaseURLFlag         = "sdk-base-url"
	sdkEventsURLFlag       = "sdk-events-url"
	redisAddressFlag       = "redis-address"
	redisPasswordFlag      = "redis-password"
	redisDBFlag            = "redis-db"
	apiKeysFlag            = "api-keys"
	targetPollDurationFlag = "target-poll-duration"
)

func init() {
	flag.BoolVar(&bypassAuth, bypassAuthFlag, false, "bypasses authentication")
	// TODO - FFM-1812 - we should update this to be loglevel
	flag.BoolVar(&debug, debugFlag, false, "enables debug logging")
	flag.BoolVar(&offline, offlineFlag, false, "enables side loading of data from config dir")
	flag.IntVar(&port, portFlag, 7000, "port that the proxy service is exposed on")
	flag.StringVar(&accountIdentifier, accountIdentifierFlag, "", "account identifier to load remote config for")
	flag.StringVar(&orgIdentifier, orgIdentifierFlag, "", "org identifier to load remote config for")
	flag.StringVar(&adminService, adminServiceFlag, "https://harness.io/gateway/cf", "the url of the ff admin service")
	flag.StringVar(&adminServiceToken, adminServiceTokenFlag, "", "token to use with the ff service")
	flag.StringVar(&clientService, clientServiceFlag, "https://config.ff.harness.io/", "the url of the ff client service")
	flag.StringVar(&authSecret, authSecretFlag, "", "the secret used for signing auth tokens")
	flag.StringVar(&sdkBaseURL, sdkBaseURLFlag, "https://config.ff.harness.io/", "url for the sdk to connect to")
	flag.StringVar(&sdkEventsURL, sdkEventsURLFlag, "https://events.ff.harness.io/", "url for the sdk to send metrics to")
	flag.StringVar(&redisAddress, redisAddressFlag, "", "Redis host:port address")
	flag.StringVar(&redisPassword, redisPasswordFlag, "", "Optional. Redis password")
	flag.IntVar(&redisDB, redisDBFlag, 0, "Database to be selected after connecting to the server.")
	flag.Var(&apiKeys, apiKeysFlag, "API keys to connect with ff-server for each environment")
	flag.IntVar(&targetPollDuration, targetPollDurationFlag, 60, "How often in seconds the proxy polls feature flags for Target changes")

	loadFlagsFromEnv(map[string]string{
		bypassAuthEnv:         bypassAuthFlag,
		debugEnv:              debugFlag,
		offlineEnv:            offlineFlag,
		portEnv:               portFlag,
		accountIdentifierEnv:  accountIdentifierFlag,
		orgIdentifierEnv:      orgIdentifierFlag,
		adminServiceEnv:       adminServiceFlag,
		adminServiceTokenEnv:  adminServiceTokenFlag,
		clientServiceEnv:      clientServiceFlag,
		authSecretEnv:         authSecretFlag,
		sdkBaseURLEnv:         sdkBaseURLFlag,
		sdkEventsURLEnv:       sdkEventsURLFlag,
		redisAddrEnv:          redisAddressFlag,
		redisPasswordEnv:      redisPasswordFlag,
		redisDBEnv:            redisDBFlag,
		apiKeysEnv:            apiKeysFlag,
		targetPollDurationEnv: targetPollDurationFlag,
	})

	flag.Parse()
}

func initFF(ctx context.Context, cache sdkCache.Cache, baseURL, eventURL, envID, envIdent, projectIdent, sdkKey string, l log.Logger) {

	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 5
	retryClient.Logger = l.With("component", "RetryClient", "environment", envID)

	l = l.With("component", "SDK", "environmentID", envID, "environment_identifier", envIdent, "project_identifier", projectIdent)
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
		harness.WithStreamEnabled(true),
		harness.WithCache(cache),
		harness.WithStoreEnabled(false), // store should be disabled until we implement a wrapper to handle multiple envs
	)
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
	requiredFlags := map[string]interface{}{}
	if offline {
		requiredFlags = map[string]interface{}{
			authSecretEnv: authSecret,
		}
	} else {
		requiredFlags = map[string]interface{}{
			accountIdentifierEnv: accountIdentifier,
			orgIdentifierEnv:     orgIdentifier,
			adminServiceTokenEnv: adminServiceToken,
			authSecretEnv:        authSecret,
			apiKeysEnv:           apiKeysFlag,
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

	// Setup logger
	logger, err := log.NewStructuredLogger(debug)
	if err != nil {
		fmt.Println("we have no logger")
		os.Exit(1)
	}

	logger.Info("service config", "debug", debug, "bypass-auth", bypassAuth, "offline", offline, "port", port, "admin-service", adminService, "account-identifier", accountIdentifier, "org-identifier", orgIdentifier, "sdk-base-url", sdkBaseURL, "sdk-events-url", sdkEventsURL, "redis-addr", redisAddress, "redis-db", redisDB, "api-keys", fmt.Sprintf("%v", apiKeys), "target-poll-duration", fmt.Sprintf("%ds", targetPollDuration))

	adminService, err := services.NewAdminService(logger, adminService, adminServiceToken)
	if err != nil {
		logger.Error("failed to create admin client", "err", err)
		os.Exit(1)
	}

	// Create cache
	var sdkCache cache.Cache
	if redisAddress != "" {
		client := redis.NewClient(&redis.Options{
			Addr:     redisAddress,
			Password: redisPassword,
			DB:       redisDB,
		})
		logger.Info("connecting to redis", "address", redisAddress)
		sdkCache = cache.NewRedisCache(client)
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

	// Load either local config from files or remote config from ff-server
	if offline {
		config, err := config.NewLocalConfig(ffproxy.DefaultConfig, ffproxy.DefaultConfigDir)
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
		logger.Info("retrieving config from ff-server...")
		remoteConfig = config.NewRemoteConfig(
			ctx,
			accountIdentifier,
			orgIdentifier,
			apiKeys,
			hash.NewSha256(),
			adminService,
			config.WithLogger(logger),
			config.WithConcurrency(20),
		)

		authConfig = remoteConfig.AuthConfig()
		targetConfig = remoteConfig.TargetConfig()
		logger.Info("successfully retrieved config from ff-server")

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
			go initFF(ctx, cacheWrapper, sdkBaseURL, sdkEventsURL, envID, projEnvInfo.EnvironmentIdentifier, projEnvInfo.ProjectIdentifier, apiKey, logger)
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

	tokenSource := ffproxy.NewTokenSource(logger, authRepo, apiKeyHasher, []byte(authSecret))

	featureEvaluator := proxyservice.NewFeatureEvaluator()

	clientService, err := services.NewClientService(logger, clientService)
	if err != nil {
		logger.Error("failed to create client for the feature flags client service", "err", err)
		os.Exit(1)
	}

	// Setup service and middleware
	var service proxyservice.ProxyService
	service = proxyservice.NewService(fcr, tr, sr, tokenSource.GenerateToken, featureEvaluator, clientService, log.NewContextualLogger(logger, log.ExtractRequestValuesFromContext), offline)

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

	if err := server.Serve(); err != nil {
		logger.Error("server stopped", "err", err)
	}
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
		stdlog.Fatalf("The following configuaration values are required: %v ", unset)
	}
}
