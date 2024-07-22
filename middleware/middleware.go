package middleware

import (
	"context"
	"errors"
	"fmt"
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

type requestContextKey string

func (r requestContextKey) String() string {
	return string(r)
}

const (
	tokenClaims requestContextKey = "tokenClaims"
	metricsPath                   = "/metrics"
)

// keyLookUp checks if the key exists in cache
type keyLookUp interface {
	Get(context context.Context, key domain.AuthAPIKey) (string, bool, error)
}

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
func NewEchoAuthMiddleware(logger log.Logger, authRepo keyLookUp, secret []byte, bypassAuth bool) echo.MiddlewareFunc {
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

			if claims, ok := token.Claims.(*domain.Claims); ok && token.Valid && isKeyInCache(c.Request().Context(), logger, authRepo, claims) {
				c.Set(tokenClaims.String(), claims)
				return nil, nil
			}
			return nil, errors.New("invalid token")
		},
		Skipper: func(c echo.Context) bool {
			if bypassAuth {
				return true
			}

			urlPath := c.Request().URL.Path
			prometheusRequest := urlPath == metricsPath && c.Request().Method == http.MethodGet

			return urlPath == "/client/auth" || urlPath == "/health" || prometheusRequest
		},
		ErrorHandlerWithContext: func(err error, c echo.Context) error {
			return c.JSON(http.StatusUnauthorized, err)
		},
	})
}

func isKeyInCache(ctx context.Context, logger log.Logger, repo keyLookUp, claims *domain.Claims) bool {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	key := claims.APIKey
	_, exists, err := repo.Get(ctx, domain.AuthAPIKey(key))
	if err != nil {
		logger.Error("auth middleware failed to lookup key in cache", "err", err)
	}
	return exists
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

// AllowQuerySemicolons is a middleware that re-writes ';' to '&' in URL query params
// See golang.org/issue/25192
func AllowQuerySemicolons() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()

			// Check if the raw query contains a semicolon
			if strings.Contains(req.URL.RawQuery, ";") {
				newURL := *req.URL
				newURL.RawQuery = strings.ReplaceAll(req.URL.RawQuery, ";", "&")

				// Clone the request to avoid modifying the original one
				r2 := req.Clone(c.Request().Context())
				r2.URL = &newURL

				// Set the modified request in the context
				c.SetRequest(r2)
			}

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
			[]string{"url", "envID", "code"},
		),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ff_proxy_http_requests_duration",
			Help:    "Records the request duration for an endpoint",
			Buckets: prometheus.DefBuckets,
		},
			[]string{"url"},
		),
		contentLength: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ff_http_requests_content_length_histogram",
			Help:    "Records the value of the Content-Length header for an http request",
			Buckets: prometheus.ExponentialBuckets(100, 2, 10),
		}, []string{}),
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
			envID := c.Param("environment_uuid")

			// Don't want to track request count or duration to the health or prometheus /metrics endpoints
			if strings.Contains(path, "/health") || path != "/metrics" {
				p.requestCount.WithLabelValues(path, envID, statusCode).Inc()
				p.requestDuration.WithLabelValues(path).Observe(duration.Seconds())

			}

			// We only care about tracking content length for POST requests
			if req.Method == http.MethodPost {
				p.contentLength.WithLabelValues().Observe(float64(req.ContentLength))
			}

			return err
		}
	}
}

// NewCorsMiddleware returns a cors middleware
func NewCorsMiddleware() echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodOptions, http.MethodPost},
		AllowHeaders: []string{"*", "Authorization"},
	})
}

func ValidateEnvironment(bypassAuth bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if skipValidateEnv(c, bypassAuth) {
				return next(c)
			}

			claims, ok := c.Get(tokenClaims.String()).(*domain.Claims)
			if !ok {
				return errors.New("missing token claims")
			}

			envID := c.Param("environment_uuid")
			if claims.Environment != envID {
				return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Environment ID [%s] mismatch with requested %s", claims.Environment, c.Param("environment_uuid")))
			}

			return next(c)
		}
	}
}

func skipValidateEnv(c echo.Context, bypassAuth bool) bool {
	if bypassAuth {
		return true
	}

	urlPath := c.Request().URL.Path

	switch urlPath {
	case domain.AuthRoute:
		return true
	case domain.StreamRoute:
		return true
	case domain.HealthRoute:
		return true
	default:
		// Skip for prometheus requests
		if urlPath == metricsPath && c.Request().Method == http.MethodGet {
			return true
		}
		return false
	}
}
