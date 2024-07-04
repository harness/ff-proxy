package transport

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	proxyservice "github.com/harness/ff-proxy/v2/proxy-service"
	jsoniter "github.com/json-iterator/go"
	"github.com/labstack/echo/v4"
)

const (
	targetHeader = "Harness-Target"
)

var (
	errBadRouting   = errors.New("bad routing")
	errBadRequest   = errors.New("bad request")
	rulesQueryParam = "rules"
)

// encodeResponse is the common method to encode all the non error response types
// to the client. If we need to we can write specific encodeResponse functions
// for endpoints that require one.
func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return jsoniter.NewEncoder(w).Encode(response)
}

func encodeStreamResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	r, ok := response.(domain.StreamResponse)
	if !ok {
		return fmt.Errorf("internal error encoding stream response")
	}

	w.Header().Add("Content-Type", "text/event-stream")
	w.Header().Add("Grip-Hold", "stream")
	w.Header().Add("Grip-Channel", r.GripChannel)
	w.Header().Add("Grip-Keep-Alive", ":\\n\\n; format=cstring; timeout=15")
	return nil
}

func encodeEchoError(c echo.Context, err error) error {
	code := codeFrom(err)

	c.Response().Header().Add("Content-Type", "application/json; charset=UTF-8")
	return c.JSON(code, map[string]interface{}{
		"error": err.Error(),
	})
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

	if errors.Is(err, proxyservice.ErrStreamDisconnected) {
		return http.StatusServiceUnavailable
	}

	return http.StatusInternalServerError
}

// decodeAuthRequest decodes POST /client/auth requests into a domain.AuthRequest
// that can be passed to the service. It returns a wrapped bad request error if
// the request body is empty or if the apiKey is empty
func decodeAuthRequest(c echo.Context, l log.Logger) (interface{}, error) {
	//#nosec G307
	defer c.Request().Body.Close()

	req := domain.AuthRequest{}
	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		l.Error("failed to read auth request body", "err", err)
		return nil, err
	}

	if len(b) == 0 {
		l.Info("invalid AuthRequest, request body is empty")
		return nil, fmt.Errorf("%w: request body cannot be empty", errBadRequest)
	}

	if err := jsoniter.Unmarshal(b, &req); err != nil {
		l.Error("failed to decode auth request", "err", err)
		return nil, err
	}

	if req.APIKey == "" {
		l.Info("invalid AuthRequest, apiKey cannot be empty", "apiKey", req.APIKey)
		return nil, fmt.Errorf("%w: apiKey cannot be empty", errBadRequest)
	}

	// Mimic what the client service does and set identifier value as the
	// name if it hasn't been provided.
	if req.Target.Name == "" {
		req.Target.Name = req.Target.Identifier
	}

	if req.Target.Identifier != "" && !isIdentifierValid(req.Target.Identifier) {
		l.Warn("invalid AuthRequest, target identifier is invalid", "targetIdentifier", req.Target.Identifier)
		return nil, fmt.Errorf("%w: target identifier is invalid", errBadRequest)
	}

	if req.Target.Name != "" && !isNameValid(req.Target.Name) {
		l.Warn("invalid AuthRequest, target name is invalid", "targetName", req.Target.Name)
		return nil, fmt.Errorf("%w: target name is invalid", errBadRequest)
	}

	return req, nil
}

// decodeHealthRequest returns an empty interface
func decodeHealthRequest(_ echo.Context, _ log.Logger) (interface{}, error) {
	return nil, nil
}

// decodeGetFeatureConfigisRequest decodes GET /client/env/{environmentUUID}/feature-configs requests
// into a domain.FeatureConfigRequest that can be passed to the ProxyService
func decodeGetFeatureConfigsRequest(c echo.Context, l log.Logger) (interface{}, error) {
	envID := c.Param("environment_uuid")
	if envID == "" {
		l.Info("invalid FeatureConfigs request, envID cannot be empty", "envID", envID)
		return nil, errBadRouting
	}

	req := domain.FeatureConfigRequest{
		EnvironmentID: envID,
	}
	return req, nil
}

// decodeGetFeatureConfigsByIdentifierRequest decodes GET /client/env/{environmentUUID}/feature-configs/{identifier} requests
// into a domain.FeatureConfigsByIdentifierRequest that can be passed to the ProxyService
func decodeGetFeatureConfigsByIdentifierRequest(c echo.Context, l log.Logger) (interface{}, error) {
	envID := c.Param("environment_uuid")
	identifier := c.Param("identifier")

	if envID == "" || identifier == "" {
		l.Info("invalid FeatureConfigsByIdentifier request, envID and identifier cannot be empty", "envID", envID, "identifier", identifier)
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
func decodeGetTargetSegmentsRequest(c echo.Context, l log.Logger) (interface{}, error) {
	envID := c.Param("environment_uuid")
	if envID == "" {
		l.Info("invalid TargetSegments request, envID cannot be empty", "envID", envID)
		return nil, errBadRouting
	}
	rules := c.QueryParam(rulesQueryParam)

	req := domain.TargetSegmentsRequest{
		EnvironmentID: envID,
		Rules:         rules,
	}
	return req, nil
}

// decodeGetTargetSegmentsByIdentifierRequest decodes GET /client/env/{environmentUUID}/target-segments/{identifier}
// requests into a domain.TargetSegmentsByIdentifierRequest that can be passed to the ProxyService
func decodeGetTargetSegmentsByIdentifierRequest(c echo.Context, l log.Logger) (interface{}, error) {
	envID := c.Param("environment_uuid")
	identifier := c.Param("identifier")

	if envID == "" || identifier == "" {
		l.Info("invalid TargetSegmentsByIdentifier request, envID and identifier cannot be empty", "envID", envID, "identifier", identifier)
		return nil, errBadRouting
	}

	rules := c.QueryParam(rulesQueryParam)

	req := domain.TargetSegmentsByIdentifierRequest{
		EnvironmentID: envID,
		Identifier:    identifier,
		Rules:         rules,
	}
	return req, nil
}

func decodeBase64String(data string, value interface{}) error {
	dst := make([]byte, len(data))

	_, err := base64.StdEncoding.Decode(dst, []byte(data))
	if err != nil {
		return err
	}

	return jsoniter.Unmarshal(dst, value)
}

// decodeGetEvaluationsRequest decodes GET /client/env/{environmentUUID}/target/{target}/evaluations
// requests into a domain.EvaluationsRequest that can be passed to the ProxyService
func decodeGetEvaluationsRequest(c echo.Context, l log.Logger) (interface{}, error) {
	envID := c.Param("environment_uuid")
	targetIdentifier := c.Param("target")

	if envID == "" || targetIdentifier == "" {
		l.Info("invalid EvaluationsRequest request, envID and targetIdentifier cannot be empty", "envID", envID, "targetIdentifier", targetIdentifier)
		return nil, errBadRouting
	}

	target, err := extractTarget(c)
	if err != nil {
		l.Warn("failed to extract target from header", "err", err)
	}

	req := domain.EvaluationsRequest{
		EnvironmentID:    envID,
		TargetIdentifier: targetIdentifier,
		Target:           target,
	}
	return req, nil
}

// decodeGetEvaluationsByFeatureRequest decodes GET /client/env/{environmentUUID}/target/{target}/evaluations/{feature}
// requests into a domain.EvaluationsByFeatureRequest that can be passed to the ProxyService
func decodeGetEvaluationsByFeatureRequest(c echo.Context, l log.Logger) (interface{}, error) {
	envID := c.Param("environment_uuid")
	targetIdentifier := c.Param("target")
	feature := c.Param("feature")

	target, err := extractTarget(c)
	if err != nil {
		l.Warn("failed to extract target from header", "err", err)
	}

	req := domain.EvaluationsByFeatureRequest{
		EnvironmentID:     envID,
		TargetIdentifier:  targetIdentifier,
		FeatureIdentifier: feature,
		Target:            target,
	}
	return req, nil
}

// decodeGetStreamRequest decodes GET /stream requests into a domain.StreamRequest that
// can be passed to the ProxyService
func decodeGetStreamRequest(c echo.Context, l log.Logger) (interface{}, error) {
	apiKey := c.Request().Header.Get("API-Key")

	req := domain.StreamRequest{
		APIKey: apiKey,
	}

	if req.APIKey == "" {
		l.Info("invalid Stream request, API Key is empty", "apiKey", req.APIKey)
		return nil, fmt.Errorf("%w: API-Key can't be empty", errBadRequest)
	}
	return req, nil
}

// decodeMetricsRequest decodes POST /metrics/{environment} requests into domain.Metrics
func decodeMetricsRequest(c echo.Context, l log.Logger) (interface{}, error) {
	//#nosec G307
	defer c.Request().Body.Close()

	b, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}

	req := domain.MetricsRequest{}
	if err := jsoniter.Unmarshal(b, &req); err != nil {
		return nil, err
	}

	req.EnvironmentID = c.Param("environment_uuid")
	if req.EnvironmentID == "" {
		l.Info("invalid Metrics request, environmentID cannot be empty", "envID", req.EnvironmentID)
		return nil, errBadRouting
	}

	return req, nil
}

var (
	identifierRegex = regexp.MustCompile("^[A-Za-z0-9.@_-]*$")
	nameRegex       = regexp.MustCompile(`^[\p{L}\d .@_-]*$`)
)

// IsIdentifierValid determine is an identifier confirms to the required format
// returns true if the identifier is valid, otherwise this will return false
func isIdentifierValid(identifier string) bool {
	if identifier == "" {
		return false
	}
	return identifierRegex.MatchString(identifier)
}

// IsNameValid determine if the name confirms to the required format
// returns true if the name is valid, otherwise will return false
func isNameValid(name string) bool {
	if name == "" {
		return false
	}
	return nameRegex.MatchString(name)
}

func extractTarget(c echo.Context) (*domain.Target, error) {
	var target *domain.Target
	encodedTarget := c.Request().Header.Get(targetHeader)

	if encodedTarget == "" {
		return target, nil
	}

	if err := decodeBase64String(encodedTarget, &target); err != nil {
		return nil, fmt.Errorf("failed to decode target from header: %s", err)
	}

	return target, nil
}
