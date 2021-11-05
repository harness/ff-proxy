package transport

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/harness/ff-proxy/log"

	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
)

// HTTPServer is an http server that handles http requests
type HTTPServer struct {
	router *mux.Router
	server *http.Server
	log    log.Logger
}

// NewHTTPServer registers the passed endpoints against routes and returns an
// HTTPServer that's ready to use
func NewHTTPServer(host string, port int, e *Endpoints, l log.Logger) *HTTPServer {
	l = log.With(l, "component", "HTTPServer")

	router := mux.NewRouter()
	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", host, port),
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

//ServeHTTP makes HTTPServer implement the http.Handler interface
func (h *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.router.ServeHTTP(w, r)
}

// Serve listens on the HTTPServers addr and handles requests
func (h *HTTPServer) Serve() error {
	h.log.Info("msg", "starting http server", "addr", h.server.Addr)
	return h.server.ListenAndServe()
}

// Shutdown gracefully shutsdown the server
func (h *HTTPServer) Shutdown(ctx context.Context) error {
	h.log.Info("msg", "shutting down http server...", "addr", h.server.Addr)
	return h.server.Shutdown(ctx)
}

// Use applies the passed MiddewareFuncs to all endpoints on the HTTPServer
func (h *HTTPServer) Use(mw ...mux.MiddlewareFunc) {
	h.router.Use(mw...)
}

func (h *HTTPServer) registerEndpoints(e *Endpoints) {
	options := []httptransport.ServerOption{
		httptransport.ServerErrorHandler(errorHandler{logger: h.log}),
		httptransport.ServerErrorEncoder(encodeError),
		httptransport.ServerBefore(httptransport.PopulateRequestContext),
	}

	streamOptions := []StreamHandlerOption{
		StreamHandlerErrorHandler(errorHandler{logger: h.log}),
		StreamHandlerErrorEncoder(encodeError),
		StreamHandlerBefore(httptransport.PopulateRequestContext),
	}

	h.router.Methods(http.MethodPost).Path("/client/auth").Handler(httptransport.NewServer(
		e.PostAuthenticate,
		decodeAuthRequest,
		encodeResponse,
		options...,
	))

	h.router.Methods(http.MethodGet).Path("/client/env/{environmentUUID}/feature-configs").Handler(httptransport.NewServer(
		e.GetFeatureConfigs,
		decodeGetFeatureConfigsRequest,
		encodeResponse,
		options...,
	))

	h.router.Methods(http.MethodGet).Path("/client/env/{environmentUUID}/feature-configs/{identifier}").Handler(httptransport.NewServer(
		e.GetFeatureConfigsByIdentifier,
		decodeGetFeatureConfigsByIdentifierRequest,
		encodeResponse,
		options...,
	))

	h.router.Methods(http.MethodGet).Path("/client/env/{environmentUUID}/target-segments").Handler(httptransport.NewServer(
		e.GetTargetSegments,
		decodeGetTargetSegmentsRequest,
		encodeResponse,
		options...,
	))

	h.router.Methods(http.MethodGet).Path("/client/env/{environmentUUID}/target-segments/{identifier}").Handler(httptransport.NewServer(
		e.GetTargetSegmentsByIdentifier,
		decodeGetTargetSegmentsByIdentifierRequest,
		encodeResponse,
		options...,
	))

	h.router.Methods(http.MethodGet).Path("/client/env/{environmentUUID}/target/{target}/evaluations").Handler(httptransport.NewServer(
		e.GetEvaluations,
		decodeGetEvaluationsRequest,
		encodeResponse,
		options...,
	))

	h.router.Methods(http.MethodGet).Path("/client/env/{environmentUUID}/target/{target}/evaluations/{feature}").Handler(httptransport.NewServer(
		e.GetEvaluationsByFeature,
		decodeGetEvaluationsByFeatureRequest,
		encodeResponse,
		options...,
	))

	h.router.Methods(http.MethodGet).Path("/stream").Handler(NewStreamHandler(
		e.GetStream,
		decodeGetStreamRequest,
		encodeStreamResponse,
		streamOptions...,
	))

	h.router.Methods(http.MethodPost).Path("/metrics/{environmentUUID}").Handler(httptransport.NewServer(
		e.PostMetrics,
		decodeMetricsRequest,
		encodeResponse,
		options...,
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

// errorHandler handles logging transport errors
type errorHandler struct {
	logger log.Logger
}

// Handle makes errorHandler implement the httptransport.ErrorHandler interface
func (e errorHandler) Handle(ctx context.Context, err error) {
	method := ctx.Value(httptransport.ContextKeyRequestMethod)
	path := ctx.Value(httptransport.ContextKeyRequestPath)
	e.logger.Error("method", method, "path", path, "err", err)
}
