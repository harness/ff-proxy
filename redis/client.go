package redis

import (
	"fmt"
	"strings"

	"github.com/harness/ff-proxy/v2/log"
	"github.com/redis/go-redis/v9"
)

func NewClient(config *Config, logger log.Logger) (redis.UniversalClient, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid redis config: %w", err)
	}

	authMode := config.AuthMode()

	var opts *redis.UniversalOptions
	var err error

	switch authMode {
	case AuthModePassword:
		opts, err = buildOptionsWithPasswordAuth(config, logger)
	case AuthModeMTLS:
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

	return &redis.UniversalOptions{
		Addrs:           addrs,
		DB:              config.DB,
		Username:        config.Username,
		Password:        config.Password,
		PoolSize:        config.PoolSize,
		TLSConfig:       parsed.TLSConfig,
		MaxRetries:      config.MaxRetries,
		MinRetryBackoff: config.MinRetryBackoff,
		MaxRetryBackoff: config.MaxRetryBackoff,
		DialTimeout:     config.DialTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		PoolTimeout:     config.PoolTimeout,
		MinIdleConns:    config.MinIdleConns,
		MaxIdleConns:    config.MaxIdleConns,
		MaxActiveConns:  config.MaxActiveConns,
		ConnMaxIdleTime: config.ConnMaxIdleTime,
		ConnMaxLifetime: config.ConnMaxLifetime,
	}, nil
}

func buildOptionsWithMTLSAuth(config *Config, logger log.Logger) (*redis.UniversalOptions, error) {
	addrs := parseAddresses(config.Address)

	tlsConfig, err := BuildTLSConfig(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build mTLS config: %w", err)
	}

	return &redis.UniversalOptions{
		Addrs:           addrs,
		DB:              config.DB,
		Username:        config.Username,
		Password:        config.Password,
		PoolSize:        config.PoolSize,
		TLSConfig:       tlsConfig,
		MaxRetries:      config.MaxRetries,
		MinRetryBackoff: config.MinRetryBackoff,
		MaxRetryBackoff: config.MaxRetryBackoff,
		DialTimeout:     config.DialTimeout,
		ReadTimeout:     config.ReadTimeout,
		WriteTimeout:    config.WriteTimeout,
		PoolTimeout:     config.PoolTimeout,
		MinIdleConns:    config.MinIdleConns,
		MaxIdleConns:    config.MaxIdleConns,
		MaxActiveConns:  config.MaxActiveConns,
		ConnMaxIdleTime: config.ConnMaxIdleTime,
		ConnMaxLifetime: config.ConnMaxLifetime,
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

func removeRedisScheme(addr string) string {
	return strings.TrimPrefix(strings.TrimPrefix(addr, "redis://"), "rediss://")
}
