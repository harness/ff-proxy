package redis

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/harness/ff-proxy/v2/log"
	"github.com/redis/go-redis/v9"
)

func NewClient(config *Config, logger log.Logger) (redis.UniversalClient, error) {
	config.AutoDetectTLS()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid redis config: %w", err)
	}

	authMode := config.AuthMode()

	var opts *redis.UniversalOptions
	var err error

	switch authMode {
	case "password":
		opts, err = buildOptionsWithPasswordAuth(config, logger)
	case "mtls":
		opts, err = buildOptionsWithMTLSAuth(config, logger)
	default:
		return nil, fmt.Errorf("unsupported auth mode: %s", authMode)
	}

	if err != nil {
		return nil, err
	}

	logger.Info("connecting to redis",
		"address", config.Address,
		"db", config.DB,
		"authMode", authMode,
		"tlsEnabled", config.TLSEnabled,
		"poolSize", opts.PoolSize,
	)

	return redis.NewUniversalClient(opts), nil
}

func buildOptionsWithPasswordAuth(config *Config, logger log.Logger) (*redis.UniversalOptions, error) {
	addrs := parseAddresses(config.Address)
	parsed, err := parseRedisURL(config.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis address: %w", err)
	}

	timeouts := calculateTimeouts(config)
	poolSize := calculatePoolSize(config)
	adjustPoolTimeoutIfNeeded(&timeouts, logger)

	return &redis.UniversalOptions{
		Addrs:           addrs,
		DB:              config.DB,
		Username:        config.Username,
		Password:        config.Password,
		PoolSize:        poolSize,
		TLSConfig:       parsed.TLSConfig,
		MaxRetries:      config.MaxRetries,
		MinRetryBackoff: timeouts.minRetryBackoff,
		MaxRetryBackoff: timeouts.maxRetryBackoff,
		DialTimeout:     timeouts.dialTimeout,
		ReadTimeout:     timeouts.readTimeout,
		WriteTimeout:    timeouts.writeTimeout,
		PoolTimeout:     timeouts.poolTimeout,
		MinIdleConns:    config.MinIdleConns,
		MaxIdleConns:    config.MaxIdleConns,
		MaxActiveConns:  config.MaxActiveConns,
		ConnMaxIdleTime: timeouts.maxIdleTime,
		ConnMaxLifetime: timeouts.connMaxLifetime,
	}, nil
}

func buildOptionsWithMTLSAuth(config *Config, logger log.Logger) (*redis.UniversalOptions, error) {
	addrs := parseAddresses(config.Address)

	tlsConfig, err := BuildTLSConfig(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build mTLS config: %w", err)
	}

	timeouts := calculateTimeouts(config)
	poolSize := calculatePoolSize(config)
	adjustPoolTimeoutIfNeeded(&timeouts, logger)

	return &redis.UniversalOptions{
		Addrs:           addrs,
		DB:              config.DB,
		Username:        config.Username,
		Password:        config.Password,
		PoolSize:        poolSize,
		TLSConfig:       tlsConfig,
		MaxRetries:      config.MaxRetries,
		MinRetryBackoff: timeouts.minRetryBackoff,
		MaxRetryBackoff: timeouts.maxRetryBackoff,
		DialTimeout:     timeouts.dialTimeout,
		ReadTimeout:     timeouts.readTimeout,
		WriteTimeout:    timeouts.writeTimeout,
		PoolTimeout:     timeouts.poolTimeout,
		MinIdleConns:    config.MinIdleConns,
		MaxIdleConns:    config.MaxIdleConns,
		MaxActiveConns:  config.MaxActiveConns,
		ConnMaxIdleTime: timeouts.maxIdleTime,
		ConnMaxLifetime: timeouts.connMaxLifetime,
	}, nil
}

func parseAddresses(address string) []string {
	splitAddr := strings.Split(address, ",")
	for i, split := range splitAddr {
		splitAddr[i] = removeRedisScheme(split)
	}
	return splitAddr
}

func parseRedisURL(address string) (*redis.Options, error) {
	connectionString := address
	if !strings.HasPrefix(address, "redis://") && !strings.HasPrefix(address, "rediss://") {
		connectionString = fmt.Sprintf("redis://%s", address)
	}

	parsed, err := redis.ParseURL(connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis address: %w", err)
	}

	return parsed, nil
}

type timeoutConfig struct {
	minRetryBackoff time.Duration
	maxRetryBackoff time.Duration
	dialTimeout     time.Duration
	readTimeout     time.Duration
	writeTimeout    time.Duration
	poolTimeout     time.Duration
	maxIdleTime     time.Duration
	connMaxLifetime time.Duration
}

func calculateTimeouts(config *Config) timeoutConfig {
	return timeoutConfig{
		minRetryBackoff: time.Duration(config.MinRetryBackoffMilliseconds) * time.Millisecond,
		maxRetryBackoff: time.Duration(config.MaxRetryBackoffMilliseconds) * time.Millisecond,
		dialTimeout:     time.Duration(config.DialTimeoutSeconds) * time.Second,
		readTimeout:     time.Duration(config.ReadTimeoutSeconds) * time.Second,
		writeTimeout:    time.Duration(config.WriteTimeoutSeconds) * time.Second,
		poolTimeout:     time.Duration(config.PoolTimeoutSeconds) * time.Second,
		maxIdleTime:     time.Duration(config.ConnMaxIdleTimeMinutes) * time.Minute,
		connMaxLifetime: time.Duration(config.ConnMaxLifetimeMinutes) * time.Minute,
	}
}

func adjustPoolTimeoutIfNeeded(timeouts *timeoutConfig, logger log.Logger) {
	if timeouts.poolTimeout < timeouts.readTimeout {
		timeouts.poolTimeout = timeouts.readTimeout + time.Second
		logger.Warn("redis pool timeout adjusted", "readTimeout", timeouts.readTimeout, "poolTimeout", timeouts.poolTimeout)
	}
}

func calculatePoolSize(config *Config) int {
	poolSize := config.PoolSize * runtime.NumCPU()
	if config.PoolSizeLiteral > 0 {
		poolSize = config.PoolSizeLiteral
	}
	return poolSize
}

func removeRedisScheme(addr string) string {
	return strings.TrimPrefix(strings.TrimPrefix(addr, "redis://"), "rediss://")
}
