package redis

import (
	"fmt"
	"strings"

	"github.com/harness/ff-proxy/v2/files"
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

	MaxRetries                  int
	MinRetryBackoffMilliseconds int
	MaxRetryBackoffMilliseconds int
	DialTimeoutSeconds          int
	ReadTimeoutSeconds          int
	WriteTimeoutSeconds         int
	PoolSize                    int
	PoolSizeLiteral             int
	PoolTimeoutSeconds          int
	MinIdleConns                int
	MaxIdleConns                int
	MaxActiveConns              int
	ConnMaxIdleTimeMinutes      int
	ConnMaxLifetimeMinutes      int
}

func (c *Config) AuthMode() string {
	if c.TLSEnabled && c.TLSMode == "mtls" {
		return "mtls"
	}
	return "password"
}

func (c *Config) Validate() error {
	authMode := c.AuthMode()

	var validators []validator

	switch authMode {
	case "password":
		return nil
	case "mtls":
		validators = append(validators, newTLSModeValidator(c))
		validators = append(validators, newMTLSCertificateValidator(c))
	default:
		validators = append(validators, newTLSModeValidator(c))
	}

	return validateAll(validators)
}

func (c *Config) AutoDetectTLS() {
	if strings.HasPrefix(c.Address, "rediss://") && !c.TLSEnabled {
		c.TLSEnabled = true
		if c.TLSMode == "" {
			c.TLSMode = "tls"
		}
	}

	if c.TLSEnabled && c.TLSMode == "" {
		c.TLSMode = "tls"
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
	if v.tlsMode != "" && v.tlsMode != "mtls" {
		return fmt.Errorf("invalid TLS mode: %s (only 'mtls' is supported)", v.tlsMode)
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
	if !v.tlsEnabled || v.tlsMode != "mtls" {
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
