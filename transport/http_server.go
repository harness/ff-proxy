package transport

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/harness/ff-proxy/log"
	"github.com/labstack/echo/v4"
)

// HTTPServer is an http server that handles http requests
type HTTPServer struct {
	router *echo.Echo
	server *http.Server
	log    log.Logger
}

// NewHTTPServer registers the passed endpoints against routes and returns an
// HTTPServer that's ready to use
func NewHTTPServer(port int, e *Endpoints, l log.Logger) *HTTPServer {
	l = l.With("component", "HTTPServer")

	router := echo.New()
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           cors(router),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       1 * time.Minute,
	}

	h := &HTTPServer{
		router: router,
		server: server,
		log:    l,
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
	h.router.POST("/client/auth", NewUnaryHandler(
		e.PostAuthenticate,
		decodeAuthRequest,
		encodeResponse,
		encodeEchoError,
	))

	h.router.GET("/health", NewUnaryHandler(
		e.Health,
		decodeHealthRequest,
		encodeHealthResponse,
		encodeEchoError,
	))

	h.router.GET("/client/env/:environment_uuid/feature-configs", NewUnaryHandler(
		e.GetFeatureConfigs,
		decodeGetFeatureConfigsRequest,
		encodeResponse,
		encodeEchoError,
	))

	h.router.GET("/client/env/:environment_uuid/feature-configs/:identifier", NewUnaryHandler(
		e.GetFeatureConfigsByIdentifier,
		decodeGetFeatureConfigsByIdentifierRequest,
		encodeResponse,
		encodeEchoError,
	))

	h.router.GET("/client/env/:environment_uuid/target-segments", NewUnaryHandler(
		e.GetTargetSegments,
		decodeGetTargetSegmentsRequest,
		encodeResponse,
		encodeEchoError,
	))

	h.router.GET("/client/env/:environment_uuid/target-segments/:identifier", NewUnaryHandler(
		e.GetTargetSegmentsByIdentifier,
		decodeGetTargetSegmentsByIdentifierRequest,
		encodeResponse,
		encodeEchoError,
	))

	h.router.GET("/client/env/:environment_uuid/target/:target/evaluations", NewUnaryHandler(
		e.GetEvaluations,
		decodeGetEvaluationsRequest,
		encodeResponse,
		encodeEchoError,
	))

	h.router.GET("/client/env/:environment_uuid/target/:target/evaluations/:feature", NewUnaryHandler(
		e.GetEvaluationsByFeature,
		decodeGetEvaluationsByFeatureRequest,
		encodeResponse,
		encodeEchoError,
	))

	h.router.GET("/stream", NewUnaryHandler(
		e.GetStream,
		decodeGetStreamRequest,
		encodeStreamResponse,
		encodeEchoError,
	))

	h.router.POST("/metrics/:environment_uuid", NewUnaryHandler(
		e.PostMetrics,
		decodeMetricsRequest,
		encodeResponse,
		encodeEchoError,
	))
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Methods", "GET,OPTIONS")
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type,Authorization,Accept,Origin,API-Key")

		if r.Method == http.MethodOptions {
			return
		}

		next.ServeHTTP(w, r)
	})
}
