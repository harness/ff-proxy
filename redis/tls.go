package redis

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/harness/ff-proxy/v2/log"
)

func BuildTLSConfig(config *Config, logger log.Logger) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if config.TLSCACertPath != "" {
		caCert, err := os.ReadFile(config.TLSCACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	if config.TLSMode == "mtls" {
		if config.TLSClientCertPath == "" || config.TLSClientKeyPath == "" {
			return nil, fmt.Errorf("client certificate and key required for mTLS")
		}

		cert, err := tls.LoadX509KeyPair(config.TLSClientCertPath, config.TLSClientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	tlsConfig.InsecureSkipVerify = config.TLSInsecureSkipVerify
	if config.TLSServerName != "" {
		tlsConfig.ServerName = config.TLSServerName
	}

	return tlsConfig, nil
}
