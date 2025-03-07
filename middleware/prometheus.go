package middleware

import (
	"github.com/prometheus/client_golang/prometheus"
)

// prometheusAuth is used for tracking prometheus metrics around auth token validation
type prometheusAuth struct {
	currentSecretTokens prometheus.Counter
	legacySecretTokens  prometheus.Counter
}

func newPrometheusAuth(reg *prometheus.Registry) *prometheusAuth {
	p := &prometheusAuth{
		currentSecretTokens: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ff_proxy_auth_current_secret_tokens_total",
			Help: "The total number of auth tokens decoded with the current secret",
		}),
		legacySecretTokens: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ff_proxy_auth_legacy_secret_tokens_total",
			Help: "The total number of auth tokens decoded with a legacy secret",
		}),
	}

	reg.MustRegister(p.currentSecretTokens, p.legacySecretTokens)
	return p
}
