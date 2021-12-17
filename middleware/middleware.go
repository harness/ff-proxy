package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/harness/ff-proxy/domain"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// NewEchoLoggingMiddleware returns a new echo middleware that logs requests and
// their response
func NewEchoLoggingMiddleware() echo.MiddlewareFunc {
	return middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "{\"level\":\"info\",\"ts\":\"${time_rfc3339}\",\"method\":\"${method}\",\"path\":\"${path}\",\"status\":\"${status}\",\"took\":\"${latency_human}\",\"component\":\"LoggingMiddleware\",\"reqID\":\"${id}\"}\n",
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

			if _, ok := token.Claims.(*domain.Claims); ok && token.Valid {
				return nil, nil
			}
			return nil, errors.New("invalid token")
		},
		Skipper: func(c echo.Context) bool {
			if bypassAuth {
				return true
			}

			return c.Request().URL.Path == "/client/auth"
		},
		ErrorHandlerWithContext: func(err error, c echo.Context) error {
			return c.JSON(http.StatusUnauthorized, err)
		},
	})
}

type contextKey string

const requestIDKey contextKey = "requestID"

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

			req = req.WithContext(context.WithValue(req.Context(), requestIDKey, reqID))
			c.SetRequest(req)

			resp.Header().Set(echo.HeaderXRequestID, reqID)
			return next(c)
		}
	}
}

// GetRequestID extracts the requestID value from the context if it exists.
func GetRequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}
