package redis

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/harness/ff-proxy/v2/files"
)

const (
	AuthModePassword = "password"
	AuthModeMTLS     = "mtls"
)

type Config struct {
	Address  string
	Username string
	Password string
	DB       int

	// mTLS configuration - all three required for mTLS
	MTLSCACertPath        string
	MTLSClientCertPath    string
	MTLSClientKeyPath     string
	TLSInsecureSkipVerify bool
	TLSServerName         string

	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	PoolSize        int
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
	mtlsCACertPath, mtlsClientCertPath, mtlsClientKeyPath string,
	tlsInsecureSkipVerify bool,
	tlsServerName string,
	maxRetries, minRetryBackoffMilliseconds, maxRetryBackoffMilliseconds int,
	dialTimeoutSeconds, readTimeoutSeconds, writeTimeoutSeconds int,
	poolSize, poolSizeLiteral, poolTimeoutSeconds int,
	minIdleConns, maxIdleConns, maxActiveConns int,
	connMaxIdleTimeMinutes, connMaxLifetimeMinutes int,
) *Config {
	finalPoolSize := calculateFinalPoolSize(poolSize, poolSizeLiteral)

	minRetryBackoff := time.Duration(minRetryBackoffMilliseconds) * time.Millisecond
	maxRetryBackoff := time.Duration(maxRetryBackoffMilliseconds) * time.Millisecond
	dialTimeout := time.Duration(dialTimeoutSeconds) * time.Second
	readTimeout := time.Duration(readTimeoutSeconds) * time.Second
	writeTimeout := time.Duration(writeTimeoutSeconds) * time.Second
	poolTimeout := time.Duration(poolTimeoutSeconds) * time.Second
	connMaxIdleTime := time.Duration(connMaxIdleTimeMinutes) * time.Minute
	connMaxLifetime := time.Duration(connMaxLifetimeMinutes) * time.Minute

	if poolTimeout < readTimeout {
		poolTimeout = readTimeout + time.Second
	}

	return &Config{
		Address:  address,
		Username: username,
		Password: password,
		DB:       db,

		MTLSCACertPath:        mtlsCACertPath,
		MTLSClientCertPath:    mtlsClientCertPath,
		MTLSClientKeyPath:     mtlsClientKeyPath,
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

func calculateFinalPoolSize(poolSize, poolSizeLiteral int) int {
	if poolSizeLiteral > 0 {
		return poolSizeLiteral
	}
	return poolSize * runtime.NumCPU()
}

func (c *Config) AuthMode() string {
	// mTLS is enabled if all three certificate paths are provided
	// Validation ensures all three are set if any are set
	if c.MTLSCACertPath != "" && c.MTLSClientCertPath != "" && c.MTLSClientKeyPath != "" {
		return AuthModeMTLS
	}
	return AuthModePassword
}

func (c *Config) Validate() error {
	// Check if any mTLS certificate path is set
	hasAnyMTLSCert := c.MTLSCACertPath != "" || c.MTLSClientCertPath != "" || c.MTLSClientKeyPath != ""

	if hasAnyMTLSCert {
		// If any is set, all three must be set
		var missing []string
		if c.MTLSCACertPath == "" {
			missing = append(missing, "REDIS_MTLS_CA_CERT")
		}
		if c.MTLSClientCertPath == "" {
			missing = append(missing, "REDIS_MTLS_CLIENT_CERT")
		}
		if c.MTLSClientKeyPath == "" {
			missing = append(missing, "REDIS_MTLS_CLIENT_KEY")
		}

		if len(missing) > 0 {
			return fmt.Errorf("incomplete mTLS configuration: all three certificate paths are required when any are set. Missing: %s", strings.Join(missing, ", "))
		}

		// All three are set, validate they exist and are readable
		validators := []validator{newMTLSCertificateValidator(c)}
		return validateAll(validators)
	}

	// No mTLS certificates set, password auth is fine
	return nil
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

type mtlsCertificateValidator struct {
	caCertPath     string
	clientCertPath string
	clientKeyPath  string
}

func newMTLSCertificateValidator(config *Config) validator {
	return &mtlsCertificateValidator{
		caCertPath:     config.MTLSCACertPath,
		clientCertPath: config.MTLSClientCertPath,
		clientKeyPath:  config.MTLSClientKeyPath,
	}
}

func (v *mtlsCertificateValidator) validate() error {
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
