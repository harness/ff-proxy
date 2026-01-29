package redis

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	"github.com/harness/ff-proxy/v2/log"
)

// BuildTLSConfig creates a TLS configuration for Redis mTLS connections.
func BuildTLSConfig(config *Config, logger log.Logger) (*tls.Config, error) {
	caCertPool, err := loadCACertificate(config.MTLSCACertPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	clientCert, err := loadClientCertificateWithChain(
		config.MTLSClientCertPath,
		config.MTLSClientKeyPath,
		config.MTLSCACertPath,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	serverName := extractServerName(config.Address, config.TLSServerName)

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		RootCAs:            caCertPool,
		Certificates:       []tls.Certificate{clientCert},
		ServerName:         serverName,
		InsecureSkipVerify: config.TLSInsecureSkipVerify,
	}

	return tlsConfig, nil
}

// loadCACertificate loads the CA certificate and returns a cert pool.
func loadCACertificate(path string, logger log.Logger) (*x509.CertPool, error) {
	caCertPEM, err := os.ReadFile(path)
	if err != nil {
		logger.Error("failed to read CA certificate", "path", path, "err", err)
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertPEM) {
		logger.Error("failed to parse CA certificate")
		return nil, fmt.Errorf("failed to parse CA certificate from PEM")
	}

	return caCertPool, nil
}

// loadClientCertificateWithChain loads the client certificate and creates a chain
// that includes the CA certificate. This is required by Redis for proper client certificate verification.
func loadClientCertificateWithChain(
	clientCertPath, clientKeyPath, caCertPath string,
	logger log.Logger,
) (tls.Certificate, error) {
	clientCertPEM, err := os.ReadFile(clientCertPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to read client certificate: %w", err)
	}

	clientKeyPEM, err := os.ReadFile(clientKeyPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to read client key: %w", err)
	}

	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	certChainPEM := append(clientCertPEM, []byte("\n")...)
	certChainPEM = append(certChainPEM, caCertPEM...)

	cert, err := tls.X509KeyPair(certChainPEM, clientKeyPEM)
	if err != nil {
		logger.Error("failed to create certificate with chain, trying without chain", "err", err)
		cert, err = tls.X509KeyPair(clientCertPEM, clientKeyPEM)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("failed to load client certificate: %w", err)
		}
	}

	return cert, nil
}

// extractServerName extracts the server name from the Redis address for SNI.
func extractServerName(address, explicitServerName string) string {
	if explicitServerName != "" {
		return explicitServerName
	}

	addr := address
	if strings.HasPrefix(addr, "rediss://") {
		addr = strings.TrimPrefix(addr, "rediss://")
	} else if strings.HasPrefix(addr, "redis://") {
		addr = strings.TrimPrefix(addr, "redis://")
	}

	if idx := strings.Index(addr, ":"); idx > 0 {
		return addr[:idx]
	}

	return ""
}
