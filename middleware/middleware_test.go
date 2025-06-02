package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/golang-jwt/jwt/v4"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowQuerySemicolons(t *testing.T) {
	tests := []struct {
		name     string
		rawQuery string
		expected string
	}{
		{
			name:     "No semicolons in query",
			rawQuery: "param1=value1&param2=value2",
			expected: "param1=value1&param2=value2",
		},
		{
			name:     "Semicolons in query",
			rawQuery: "param1=value1;param2=value2",
			expected: "param1=value1&param2=value2",
		},
		{
			name:     "Mixed semicolons and ampersands",
			rawQuery: "param1=value1;param2=value2&param3=value3",
			expected: "param1=value1&param2=value2&param3=value3",
		},
		{
			name:     "Multiple semicolons",
			rawQuery: "param1=value1;param2=value2;param3=value3",
			expected: "param1=value1&param2=value2&param3=value3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.rawQuery, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			h := AllowQuerySemicolons()(func(c echo.Context) error {
				return c.String(http.StatusOK, "test")
			})

			err := h(c)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, c.Request().URL.RawQuery)
		})
	}
}

func benchmarkAllowQuerySemicolons(b *testing.B, middlewareFunc echo.MiddlewareFunc, rawQuery string) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/?"+rawQuery, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := middlewareFunc(func(c echo.Context) error {
		return c.String(http.StatusOK, "test")
	})

	// Run the benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = handler(c)
	}
}

func BenchmarkAllowQuerySemicolons(b *testing.B) {
	allowQuerySemicolonsV1 := func() echo.MiddlewareFunc {
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

	allowQuerySemicolonsV2 := func() echo.MiddlewareFunc {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				req := c.Request()

				if strings.Contains(req.URL.RawQuery, ";") {
					r2 := new(http.Request)
					*r2 = *req
					r2.URL = new(url.URL)
					*r2.URL = *req.URL
					r2.URL.RawQuery = strings.ReplaceAll(req.URL.RawQuery, ";", "&")
					c.SetRequest(r2)
				}

				return next(c)
			}
		}
	}

	benchmarks := []struct {
		name       string
		middleware echo.MiddlewareFunc
		rawQuery   string
	}{
		{
			name:       "V1_NoSemicolons",
			middleware: allowQuerySemicolonsV1(),
			rawQuery:   "param1=value1&param2=value2",
		},
		{
			name:       "V1_WithSemicolons",
			middleware: allowQuerySemicolonsV1(),
			rawQuery:   "param1=value1;param2=value2",
		},
		{
			name:       "V2_NoSemicolons",
			middleware: allowQuerySemicolonsV2(),
			rawQuery:   "param1=value1&param2=value2",
		},
		{
			name:       "V2_WithSemicolons",
			middleware: allowQuerySemicolonsV2(),
			rawQuery:   "param1=value1;param2=value2",
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			benchmarkAllowQuerySemicolons(b, bm.middleware, bm.rawQuery)
		})
	}
}

func TestSkipper(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		urlPath  string
		expected bool
	}{
		{
			name:     "Auth route",
			method:   http.MethodGet,
			urlPath:  domain.AuthRoute,
			expected: true,
		},
		{
			name:     "Stream route",
			method:   http.MethodGet,
			urlPath:  domain.StreamRoute,
			expected: true,
		},
		{
			name:     "Metrics route GET",
			method:   http.MethodGet,
			urlPath:  "/metrics",
			expected: true,
		},
		{
			name:     "Metrics route POST",
			method:   http.MethodPost,
			urlPath:  "/metrics",
			expected: false,
		},
		{
			name:     "Metrics route POST with environment_uuid",
			method:   http.MethodPost,
			urlPath:  "/metrics/environment_uuid",
			expected: false,
		},
		{
			name:     "Other route",
			method:   http.MethodGet,
			urlPath:  "/other",
			expected: false,
		},
		{
			name:     "Health check request",
			method:   http.MethodGet,
			urlPath:  "/health",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(tt.method, tt.urlPath, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			result := skipValidateEnv(c, false)
			assert.Equal(t, tt.expected, result)
		})
	}
}

type mockKeyLookup struct {
	shouldExist bool
}

func (m *mockKeyLookup) Get(_ context.Context, _ domain.AuthAPIKey) (string, bool, error) {
	return "", m.shouldExist, nil
}

func TestNewEchoAuthMiddleware(t *testing.T) {
	// Create test secrets
	currentSecret := []byte("current-secret")
	legacySecret := []byte("legacy-secret")
	legacySecrets := [][]byte{legacySecret}

	// Create test claims
	validClaims := &domain.Claims{
		APIKey: "test-key",
	}

	// Generate test tokens
	validToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims).SignedString(currentSecret)
	require.NoError(t, err)
	legacyToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims).SignedString(legacySecret)
	require.NoError(t, err)
	invalidToken := "invalid-token"

	tests := []struct {
		name       string
		token      string
		keyExists  bool
		bypassAuth bool
		wantStatus int
	}{
		{
			name:       "Valid token with current secret",
			token:      validToken,
			keyExists:  true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "Valid token with legacy secret",
			token:      legacyToken,
			keyExists:  true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "Invalid token",
			token:      invalidToken,
			keyExists:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Empty token",
			token:      "",
			keyExists:  true,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Valid token but key not in cache",
			token:      validToken,
			keyExists:  false,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Bypass auth enabled",
			token:      "",
			bypassAuth: true,
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			// Create mock auth repo with desired behavior
			mockRepo := &mockKeyLookup{shouldExist: tt.keyExists}

			// Create middleware
			middleware := NewEchoAuthMiddleware(nil, mockRepo, currentSecret, legacySecrets, tt.bypassAuth, prometheus.NewRegistry())

			// Create test handler
			handler := middleware(func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			})

			// Execute
			_ = handler(c)

			// Assert response status code
			assert.Equal(t, tt.wantStatus, rec.Code)

			// For successful non-bypassed auth, verify claims are set
			if tt.wantStatus == http.StatusOK && !tt.bypassAuth {
				claims := c.Get(tokenClaims.String()).(*domain.Claims)
				assert.Equal(t, validClaims.APIKey, claims.APIKey)
			}
		})
	}
}
