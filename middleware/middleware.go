package middleware

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

// NewEchoLoggingMiddleware returns a new echo middleware that logs requests and
// their response
func NewEchoLoggingMiddleware(l log.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			l.Info("request", "component", "LoggingMiddleware", "method", v.Method, "path", v.RoutePath, "status", v.Status, "took", v.Latency.String(), "reqID", v.RequestID)
			return nil
		},
		LogLatency:   true,
		LogMethod:    true,
		LogRoutePath: true,
		LogRequestID: true,
		LogStatus:    true,
	})
}

// NewEchoAuthMiddleware returns an echo middleware that checks if auth headers
// are valid
func NewEchoAuthMiddleware(secret []byte, bypassAuth bool) echo.MiddlewareFunc {
	return middleware.JWTWithConfig(middleware.JWTConfig{
		AuthScheme:  "Bearer",
		TokenLookup: "header:Authorization",
		ParseTokenFunc: func(auth string, c echo.Context) (interface{}, error) {
			if auth == "" {
				return nil, errors.New("token was empty")
			}

			token, err := jwt.ParseWithClaims(auth, &domain.Claims{}, func(t *jwt.Token) (interface{}, error) {
				return secret, nil
			})
			if err != nil {
				return nil, err
			}
			// ASZ: todo validate if the environment from claims is still in redis.
			if _, ok := token.Claims.(*domain.Claims); ok && token.Valid {
				return nil, nil
			}
			return nil, errors.New("invalid token")
		},
		Skipper: func(c echo.Context) bool {
			if bypassAuth {
				return true
			}

			urlPath := c.Request().URL.Path
			prometheusRequest := urlPath == "/metrics" && c.Request().Method == http.MethodGet

			return urlPath == "/client/auth" || urlPath == "/health" || prometheusRequest
		},
		ErrorHandlerWithContext: func(err error, c echo.Context) error {
			return c.JSON(http.StatusUnauthorized, err)
		},
	})
}

// NewEchoRequestIDMiddleware returns an echo middleware that either uses a
// provided requestID from the header or generates one and adds it to the request
// context.
func NewEchoRequestIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			resp := c.Response()

			reqID := req.Header.Get(echo.HeaderXRequestID)
			if reqID == "" {
				requestUUID, _ := uuid.NewRandom()
				reqID = requestUUID.String()
			}

			req = req.WithContext(context.WithValue(req.Context(), log.RequestIDKey, reqID))
			c.SetRequest(req)

			resp.Header().Set(echo.HeaderXRequestID, reqID)
			return next(c)
		}
	}
}

type prometheusMiddleware struct {
	requestCount    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	contentLength   *prometheus.HistogramVec
}

// NewPrometheusMiddleware creates a middleware that uses prometheus to track request rate, duration & the size
// of request bodies
func NewPrometheusMiddleware(reg prometheus.Registerer) echo.MiddlewareFunc {
	p := &prometheusMiddleware{
		requestCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ff_proxy_http_requests_total",
			Help: "Records the number of requests to an endpoint",
		},
			[]string{"url", "envID", "code", "method"},
		),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ff_proxy_http_requests_duration",
			Help:    "Records the request duration for an endpoint",
			Buckets: []float64{0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.75, 1},
		},
			[]string{"url", "envID"},
		),
		contentLength: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "ff_http_requests_content_length_histogram",
			Help: "Records the value of the Content-Length header for an http request",
			Buckets: []float64{
				100,
				250,
				500,
				1000,
				5000,
				10000,
				25000,
				50000,
				100000,  // 0.1 MiB
				250000,  // 0.25 MiB
				500000,  // 0.5MiB
				1000000, // 1 MiB
				2500000, // 2.5 MiB
				5000000, // 5 MiB
			},
		}, []string{"url", "envID"}),
	}

	reg.MustRegister(p.requestCount, p.requestDuration, p.contentLength)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// We don't care about tracking metrics for these endpoints
			urlPath := c.Request().URL.Path
			if urlPath == "/health" || urlPath == "/prometheus/metrics" {
				return next(c)
			}

			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}
			duration := time.Since(start)

			req := c.Request()
			res := c.Response()

			path := c.Path()
			statusCode := strconv.Itoa(res.Status)
			method := req.Method

			envID := c.Param("environment_uuid")

			// Don't want to track request count or duration to the health or prometheus /metrics endpoints
			if strings.Contains(path, "/health") || path != "/metrics" {
				p.requestCount.WithLabelValues(path, envID, statusCode, method).Inc()
				p.requestDuration.WithLabelValues(path, envID).Observe(duration.Seconds())

			}

			// We only care about tracking content length for POST requests
			if req.Method == http.MethodPost {
				p.contentLength.WithLabelValues(path, envID).Observe(float64(req.ContentLength))
			}

			return err
		}
	}
}
