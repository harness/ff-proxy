package transport

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
)

var proxyRoutes = domain.NewImmutableSet(map[string]struct{}{
	domain.AuthRoute:                     {},
	domain.HealthRoute:                   {},
	domain.FeatureConfigsRoute:           {},
	domain.FeatureConfigsIdentifierRoute: {},
	domain.SegmentsRoute:                 {},
	domain.SegmentsIdentifierRoute:       {},
	domain.EvaluationsRoute:              {},
	domain.EvaluationsFlagRoute:          {},
	domain.StreamRoute:                   {},
	domain.MetricsRoute:                  {},
})

type prometheusRegister interface {
	prometheus.Registerer
	prometheus.Gatherer
}

// HTTPServer is an http server that handles http requests
type HTTPServer struct {
	router     *echo.Echo
	server     *http.Server
	log        log.Logger
	tlsEnabled bool
	tlsCert    string
	tlsKey     string
}

// NewHTTPServer registers the passed endpoints against routes and returns an
// HTTPServer that's ready to use
func NewHTTPServer(port int, e *Endpoints, l log.Logger, tlsEnabled bool, tlsCert string, tlsKey string) *HTTPServer {
	l = l.With("component", "HTTPServer")

	router := echo.New()
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       1 * time.Minute,
	}

	h := &HTTPServer{
		router:     router,
		server:     server,
		log:        l,
		tlsEnabled: tlsEnabled,
		tlsCert:    tlsCert,
		tlsKey:     tlsKey,
	}
	h.registerEndpoints(e)
	return h
}

// ServeHTTP makes HTTPServer implement the http.Handler interface
func (h *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

// Serve listens on the HTTPServers addr and handles requests
func (h *HTTPServer) Serve() error {
	if h.tlsEnabled {
		h.log.Info("starting https server", "addr", h.server.Addr, "tlsCert", h.tlsCert, "tlsKey", h.tlsKey)
		return h.server.ListenAndServeTLS(h.tlsCert, h.tlsKey)
	}
	h.log.Info("starting http server", "addr", h.server.Addr)
	return h.server.ListenAndServe()
}

// Shutdown gracefully shutsdown the server
func (h *HTTPServer) Shutdown(ctx context.Context) error {
	h.log.Info("shutting down http server", "addr", h.server.Addr)
	return h.server.Shutdown(ctx)
}

// Use applies the passed MiddewareFuncs to all endpoints on the HTTPServer
func (h *HTTPServer) Use(mw ...echo.MiddlewareFunc) {
	for _, m := range mw {
		h.router.Use(m)
	}
}

func (h *HTTPServer) registerEndpoints(e *Endpoints) {
	h.router.POST(domain.AuthRoute, NewUnaryHandler(
		e.PostAuthenticate,
		decodeAuthRequest,
		encodeResponse,
		encodeEchoError,
		h.log,
	))

	h.router.GET(domain.HealthRoute, NewUnaryHandler(
		e.Health,
		decodeHealthRequest,
		encodeResponse,
		encodeEchoError,
		h.log,
	))

	h.router.GET(domain.FeatureConfigsRoute, NewUnaryHandler(
		e.GetFeatureConfigs,
		decodeGetFeatureConfigsRequest,
		encodeResponse,
		encodeEchoError,
		h.log,
	))

	h.router.GET(domain.FeatureConfigsIdentifierRoute, NewUnaryHandler(
		e.GetFeatureConfigsByIdentifier,
		decodeGetFeatureConfigsByIdentifierRequest,
		encodeResponse,
		encodeEchoError,
		h.log,
	))

	h.router.GET(domain.SegmentsRoute, NewUnaryHandler(
		e.GetTargetSegments,
		decodeGetTargetSegmentsRequest,
		encodeResponse,
		encodeEchoError,
		h.log,
	))

	h.router.GET(domain.SegmentsIdentifierRoute, NewUnaryHandler(
		e.GetTargetSegmentsByIdentifier,
		decodeGetTargetSegmentsByIdentifierRequest,
		encodeResponse,
		encodeEchoError,
		h.log,
	))

	h.router.GET(domain.EvaluationsRoute, NewUnaryHandler(
		e.GetEvaluations,
		decodeGetEvaluationsRequest,
		encodeResponse,
		encodeEchoError,
		h.log,
	))

	h.router.GET(domain.EvaluationsFlagRoute, NewUnaryHandler(
		e.GetEvaluationsByFeature,
		decodeGetEvaluationsByFeatureRequest,
		encodeResponse,
		encodeEchoError,
		h.log,
	))

	h.router.GET(domain.StreamRoute, NewUnaryHandler(
		e.GetStream,
		decodeGetStreamRequest,
		encodeStreamResponse,
		encodeEchoError,
		h.log,
	))

	h.router.POST(domain.MetricsRoute, NewUnaryHandler(
		e.PostMetrics,
		decodeMetricsRequest,
		encodeResponse,
		encodeEchoError,
		h.log,
	))
}

// WithCustomHandler lets you register a custom handler with the HTTPServer
// It will error if you try to register a handler for a route that's already defined.
func (h *HTTPServer) WithCustomHandler(method string, route string, handler http.Handler) error {
	// Don't allow new handlers on routes that are already defined
	if proxyRoutes.Has(route) {
		return fmt.Errorf("route=%s is reserved for the Proxy", route)
	}

	switch method {
	case http.MethodGet:
		h.router.GET(route, echo.WrapHandler(handler))
		return nil

	case http.MethodPost:
		h.router.POST(route, echo.WrapHandler(handler))
		return nil
	}
	return fmt.Errorf("http method %s not supported, update WithCustomHandler to add support", method)
}
