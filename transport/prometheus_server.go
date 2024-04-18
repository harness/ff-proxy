package transport

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/harness/ff-proxy/v2/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusServer is a HTTPServer that's used for exposing Prometheus metrics
type PrometheusServer struct {
	log    log.Logger
	server *http.Server
}

// NewPrometheusServer creates a PrometheusServer
func NewPrometheusServer(port int, promReg prometheusRegister, logger log.Logger) *PrometheusServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(promReg, promhttp.HandlerOpts{Registry: promReg}))

	return &PrometheusServer{
		log: logger,
		server: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           mux,
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       1 * time.Minute,
		},
	}
}

// Serve listens on the PrometheusServers addr and handles requests
func (p *PrometheusServer) Serve() error {
	p.log.Info("starting prometheus server", "addr", p.server.Addr)

	return p.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (p *PrometheusServer) Shutdown(ctx context.Context) error {
	p.log.Info("shutting down prometheus server", "addr", p.server.Addr)
	return p.server.Shutdown(ctx)
}
