package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/harness/ff-proxy/domain"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
	"github.com/labstack/echo/v4"
)

var (
	errBadRouting = errors.New("bad routing")
	errBadRequest = errors.New("bad request")
)

// encodeResponse is the common method to encode all the non error response types
// to the client. If we need to we can write specific encodeResponse functions
// for endpoints that require one.
func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

// encodeHealthResponse encodes a healthcheck response with status code
func encodeHealthResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(getHealthStatusCode(response))
	return json.NewEncoder(w).Encode(response)
}

func getHealthStatusCode(response interface{}) int {
	healthRes, ok := response.(domain.HealthResponse)
	if !ok {
		return http.StatusInternalServerError
	}

	for _, health := range healthRes {
		if health != "healthy" {
			return http.StatusServiceUnavailable
		}
	}
	return http.StatusOK
}

// encodeError encodes error responses returned from handlers
func encodeError(ctx context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codeFrom(err))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

func encodeEchoError(c echo.Context, err error) error {
	code := codeFrom(err)
	return c.JSON(code, map[string]interface{}{
		"error": err.Error(),
	})
}

// encodeStreamResponse sets the headers for a streaming response
func encodeStreamResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	return nil
}

// codeFrom casts a service error to an http.StatusCode
func codeFrom(err error) int {
	if errors.Is(err, errBadRequest) {
		return http.StatusBadRequest
	}

	if errors.Is(err, proxyservice.ErrNotFound) {
		return http.StatusNotFound
	}

	if errors.Is(err, proxyservice.ErrUnauthorised) {
		return http.StatusUnauthorized
	}

	if errors.Is(err, proxyservice.ErrNotImplemented) {
		return http.StatusNotImplemented
	}

	return http.StatusInternalServerError
}

// decodeAuthRequest decodes POST /client/auth requests into a domain.AuthRequest
// that can be passed to the service. It returns a wrapped bad request error if
// the request body is empty or if the apiKey is empty
func decodeAuthRequest(c echo.Context) (interface{}, error) {
	defer c.Request().Body.Close()

	req := domain.AuthRequest{}
	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, fmt.Errorf("%w: request body cannot be empty", errBadRequest)
	}

	if err := json.Unmarshal(b, &req); err != nil {
		return nil, err
	}

	if req.APIKey == "" {
		return nil, fmt.Errorf("%w: apiKey cannot be empty", errBadRequest)
	}
	return req, nil
}

// decodeHealthRequest returns an empty interface
func decodeHealthRequest(c echo.Context) (interface{}, error) {
	return nil, nil
}

// decodeGetFeatureConfigisRequest decodes GET /client/env/{environmentUUID}/feature-configs requests
// into a domain.FeatureConfigRequest that can be passed to the ProxyService
func decodeGetFeatureConfigsRequest(c echo.Context) (interface{}, error) {
	envID := c.Param("environment_uuid")
	if envID == "" {
		return nil, errBadRouting
	}

	req := domain.FeatureConfigRequest{
		EnvironmentID: envID,
	}
	return req, nil
}

// decodeGetFeatureConfigsByIdentifierRequest decodes GET /client/env/{environmentUUID}/feature-configs/{identifier} requests
// into a domain.FeatureConfigsByIdentifierRequest that can be passed to the ProxyService
func decodeGetFeatureConfigsByIdentifierRequest(c echo.Context) (interface{}, error) {
	envID := c.Param("environment_uuid")
	identifier := c.Param("identifier")

	if envID == "" || identifier == "" {
		return nil, errBadRouting
	}

	req := domain.FeatureConfigByIdentifierRequest{
		EnvironmentID: envID,
		Identifier:    identifier,
	}
	return req, nil
}

// decodeGetTargetSegmentsRequest decodes GET /client/env/{environmentUUID}/target-segments requests
// into a domain.TargetSegmentsRequest that can be passed to the ProxyService
func decodeGetTargetSegmentsRequest(c echo.Context) (interface{}, error) {
	envID := c.Param("environment_uuid")
	if envID == "" {
		return nil, errBadRouting
	}

	req := domain.TargetSegmentsRequest{
		EnvironmentID: envID,
	}
	return req, nil
}

// decodeGetTargetSegmentsByIdentifierRequest decodes GET /client/env/{environmentUUID}/target-segments/{identifier}
// requests into a domain.TargetSegmentsByIdentifierRequest that can be passed to the ProxyService
func decodeGetTargetSegmentsByIdentifierRequest(c echo.Context) (interface{}, error) {
	envID := c.Param("environment_uuid")
	identifier := c.Param("identifier")

	if envID == "" || identifier == "" {
		return nil, errBadRouting
	}

	req := domain.TargetSegmentsByIdentifierRequest{
		EnvironmentID: envID,
		Identifier:    identifier,
	}
	return req, nil
}

// decodeGetEvaluationsRequest decodes GET /client/env/{environmentUUID}/target/{target}/evaluations
// requests into a domain.EvaluationsRequest that can be passed to the ProxyService
func decodeGetEvaluationsRequest(c echo.Context) (interface{}, error) {
	envID := c.Param("environment_uuid")
	target := c.Param("target")

	if envID == "" || target == "" {
		return nil, errBadRouting
	}

	req := domain.EvaluationsRequest{
		EnvironmentID:    envID,
		TargetIdentifier: target,
	}
	return req, nil
}

// decodeGetEvaluationsByFeatureRequest decodes GET /client/env/{environmentUUID}/target/{target}/evaluations/{feature}
// requests into a domain.EvaluationsByFeatureRequest that can be passed to the ProxyService
func decodeGetEvaluationsByFeatureRequest(c echo.Context) (interface{}, error) {
	envID := c.Param("environment_uuid")
	target := c.Param("target")
	feature := c.Param("feature")

	req := domain.EvaluationsByFeatureRequest{
		EnvironmentID:     envID,
		TargetIdentifier:  target,
		FeatureIdentifier: feature,
	}
	return req, nil
}

// decodeGetStreamRequest decodes GET /stream requests into a domain.StreamRequest that
// can be passed to the ProxyService
func decodeGetStreamRequest(c echo.Context) (interface{}, error) {
	apiKey := c.Request().Header.Get("API-Key")

	req := domain.StreamRequest{
		APIKey: apiKey,
	}

	if req.APIKey == "" {
		return nil, fmt.Errorf("%w: API-Key can't be empty", errBadRequest)
	}
	return req, nil
}

// decodeMetricsRequest decodes POST /metrics/{environment} requests into domain.Metrics
func decodeMetricsRequest(c echo.Context) (interface{}, error) {
	defer c.Request().Body.Close()

	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}

	req := domain.MetricsRequest{}
	if err := json.Unmarshal(b, &req); err != nil {
		return nil, err
	}

	req.EnvironmentID = c.Param("environment_uuid")
	if req.EnvironmentID == "" {
		return nil, errBadRouting
	}

	return req, nil
}
