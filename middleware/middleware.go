package middleware

import (
	"errors"
	"net/http"

	"github.com/golang-jwt/jwt"
	"github.com/harness/ff-proxy/domain"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// NewEchoLoggingMiddleware returns a new echo middleware that logs requests and
// their response
func NewEchoLoggingMiddleware() echo.MiddlewareFunc {
	return middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "component=LoggingMiddleware time=${time_rfc3339} reqID=${id} method=${method}, path=${path}, status=${status}, took=${latency_human}\n",
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
