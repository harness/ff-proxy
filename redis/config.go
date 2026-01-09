package redis

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/harness/ff-proxy/v2/files"
)

// Auth mode constants
const (
	AuthModePassword = "password"
	AuthModeMTLS     = "mtls"
	TLSModeTLS       = "tls"
	TLSModeMTLS      = "mtls"
)

type Config struct {
	Address  string
	Username string
	Password string
	DB       int

	TLSEnabled            bool
	TLSMode               string
	TLSCACertPath         string
	TLSClientCertPath     string
	TLSClientKeyPath      string
	TLSInsecureSkipVerify bool
	TLSServerName         string

	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolSize        int // Final calculated pool size, ready to use
	PoolTimeout     time.Duration
	MinIdleConns    int
	MaxIdleConns    int
	MaxActiveConns  int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration
}

// NewConfig creates a new Redis configuration with time.Duration fields properly converted
func NewConfig(
	address, username, password string,
	db int,
	tlsEnabled bool,
	tlsMode, tlsCACertPath, tlsClientCertPath, tlsClientKeyPath string,
	tlsInsecureSkipVerify bool,
	tlsServerName string,
	maxRetries, minRetryBackoffMilliseconds, maxRetryBackoffMilliseconds int,
	dialTimeoutSeconds, readTimeoutSeconds, writeTimeoutSeconds int,
	poolSize, poolSizeLiteral, poolTimeoutSeconds int,
	minIdleConns, maxIdleConns, maxActiveConns int,
	connMaxIdleTimeMinutes, connMaxLifetimeMinutes int,
) *Config {
	// Calculate the final pool size immediately
	// For backwards compatibility, by default we use poolSize multiplied by the number of CPUs.
	// However, if poolSizeLiteral is set, we use it instead.
	finalPoolSize := poolSize * numCPU()
	if poolSizeLiteral > 0 {
		finalPoolSize = poolSizeLiteral
	}

	// Convert timeout values to time.Duration
	minRetryBackoff := time.Duration(minRetryBackoffMilliseconds) * time.Millisecond
	maxRetryBackoff := time.Duration(maxRetryBackoffMilliseconds) * time.Millisecond
	dialTimeout := time.Duration(dialTimeoutSeconds) * time.Second
	readTimeout := time.Duration(readTimeoutSeconds) * time.Second
	writeTimeout := time.Duration(writeTimeoutSeconds) * time.Second
	poolTimeout := time.Duration(poolTimeoutSeconds) * time.Second
	connMaxIdleTime := time.Duration(connMaxIdleTimeMinutes) * time.Minute
	connMaxLifetime := time.Duration(connMaxLifetimeMinutes) * time.Minute

	// Adjust pool timeout if needed to prevent timeout issues
	// Pool timeout should be at least readTimeout + 1 second
	if poolTimeout < readTimeout {
		poolTimeout = readTimeout + time.Second
	}

	return &Config{
		Address:  address,
		Username: username,
		Password: password,
		DB:       db,

		TLSEnabled:            tlsEnabled,
		TLSMode:               tlsMode,
		TLSCACertPath:         tlsCACertPath,
		TLSClientCertPath:     tlsClientCertPath,
		TLSClientKeyPath:      tlsClientKeyPath,
		TLSInsecureSkipVerify: tlsInsecureSkipVerify,
		TLSServerName:         tlsServerName,

		MaxRetries:      maxRetries,
		MinRetryBackoff: minRetryBackoff,
		MaxRetryBackoff: maxRetryBackoff,
		DialTimeout:     dialTimeout,
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		PoolSize:        finalPoolSize,
		PoolTimeout:     poolTimeout,
		MinIdleConns:    minIdleConns,
		MaxIdleConns:    maxIdleConns,
		MaxActiveConns:  maxActiveConns,
		ConnMaxIdleTime: connMaxIdleTime,
		ConnMaxLifetime: connMaxLifetime,
	}
}

// numCPU returns the number of CPUs, extracted to a function for testability
func numCPU() int {
	return runtime.NumCPU()
}

func (c *Config) AuthMode() string {
	if c.TLSEnabled && c.TLSMode == TLSModeMTLS {
		return AuthModeMTLS
	}
	return AuthModePassword
}

func (c *Config) Validate() error {
	authMode := c.AuthMode()

	var validators []validator

	switch authMode {
	case AuthModePassword:
		return nil
	case AuthModeMTLS:
		validators = append(validators, newTLSModeValidator(c))
		validators = append(validators, newMTLSCertificateValidator(c))
	default:
		validators = append(validators, newTLSModeValidator(c))
	}

	return validateAll(validators)
}

func (c *Config) AutoDetectTLS() {
	if strings.HasPrefix(c.Address, "rediss://") {
		c.TLSEnabled = true
	}

	if c.TLSEnabled && c.TLSMode == "" {
		c.TLSMode = TLSModeTLS
	}
}

type validator interface {
	validate() error
}

type validationError struct {
	errors []string
}

func (e *validationError) Error() string {
	return fmt.Sprintf("redis config validation failed: %s", strings.Join(e.errors, "; "))
}

func validateAll(validators []validator) error {
	var errs []string

	for _, v := range validators {
		if err := v.validate(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return &validationError{errors: errs}
	}

	return nil
}

type tlsModeValidator struct {
	tlsMode string
}

func newTLSModeValidator(config *Config) validator {
	return &tlsModeValidator{tlsMode: config.TLSMode}
}

func (v *tlsModeValidator) validate() error {
	if v.tlsMode != "" && v.tlsMode != TLSModeTLS && v.tlsMode != TLSModeMTLS {
		return fmt.Errorf("invalid TLS mode: %s (only 'tls' and 'mtls' are supported)", v.tlsMode)
	}
	return nil
}

type mtlsCertificateValidator struct {
	tlsEnabled     bool
	tlsMode        string
	caCertPath     string
	clientCertPath string
	clientKeyPath  string
}

func newMTLSCertificateValidator(config *Config) validator {
	return &mtlsCertificateValidator{
		tlsEnabled:     config.TLSEnabled,
		tlsMode:        config.TLSMode,
		caCertPath:     config.TLSCACertPath,
		clientCertPath: config.TLSClientCertPath,
		clientKeyPath:  config.TLSClientKeyPath,
	}
}

func (v *mtlsCertificateValidator) validate() error {
	if !v.tlsEnabled || v.tlsMode != TLSModeMTLS {
		return nil
	}

	if v.caCertPath == "" {
		return fmt.Errorf("CA certificate path required for mTLS")
	}

	if err := files.Exists(v.caCertPath); err != nil {
		return fmt.Errorf("CA certificate not found: %s", v.caCertPath)
	}

	if err := files.Readable(v.caCertPath); err != nil {
		return fmt.Errorf("CA certificate not readable: %s", v.caCertPath)
	}

	if v.clientCertPath == "" {
		return fmt.Errorf("client certificate path required for mTLS")
	}

	if err := files.Exists(v.clientCertPath); err != nil {
		return fmt.Errorf("client certificate not found: %s", v.clientCertPath)
	}

	if err := files.Readable(v.clientCertPath); err != nil {
		return fmt.Errorf("client certificate not readable: %s", v.clientCertPath)
	}

	if v.clientKeyPath == "" {
		return fmt.Errorf("client key path required for mTLS")
	}

	if err := files.Exists(v.clientKeyPath); err != nil {
		return fmt.Errorf("client key not found: %s", v.clientKeyPath)
	}

	if err := files.Readable(v.clientKeyPath); err != nil {
		return fmt.Errorf("client key not readable: %s", v.clientKeyPath)
	}

	return nil
}
